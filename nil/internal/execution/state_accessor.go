package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

/*
StateAccessor supposed usage

raw, err := accessor.RawAccess(tx, shardId).GetBlockHeaderByHash(hash)

data, err := accessor.Access(tx, shardId).GetFullBlockByNumber(index)
block, txns := data.Block, data.InTransactions
...
*/
type StateAccessor struct {
	cache *cache
}

func NewStateAccessor() *StateAccessor {
	const (
		blocksLRUSize = 128 // ~32Mb
		txnLRUSize    = 32
	)

	return &StateAccessor{
		cache: newRawAccessorCache(blocksLRUSize, txnLRUSize),
	}
}

func (s *StateAccessor) Access(tx db.RoTx, shardId types.ShardId) ShardAccessor {
	return ShardAccessor{s.RawAccess(tx, shardId)}
}

func (s *StateAccessor) RawAccess(tx db.RoTx, shardId types.ShardId) RawShardAccessor {
	return RawShardAccessor{
		cache:   s.cache,
		tx:      tx,
		shardId: shardId,
	}
}

type RawShardAccessor struct {
	cache   *cache
	tx      db.RoTx
	shardId types.ShardId
}

type ShardAccessor struct {
	RawShardAccessor
}

type headerWithRaw struct {
	block *types.Block
	raw   []byte
}

type cache struct {
	headersLRU      *lru.Cache[common.Hash, headerWithRaw]
	fullBlocksLRU   *lru.Cache[common.Hash, *types.RawBlockWithExtractedData]
	blockNumbersLRU *lru.Cache[types.BlockNumber, common.Hash]

	inTxnLRU      *lru.Cache[db.BlockHashAndTransactionIndex, *Txn]
	outTxnLRU     *lru.Cache[db.BlockHashAndTransactionIndex, *Txn]
	inTxnIndexLRU *lru.Cache[common.Hash, db.BlockHashAndTransactionIndex]
}

func newRawAccessorCache(blocksLRUSize, txnLRUSize int) *cache {
	// lru.New returns an error only if the size is less than 1
	headersLRU, err := lru.New[common.Hash, headerWithRaw](blocksLRUSize)
	check.PanicIfErr(err)

	fullBlocksLRU, err := lru.New[common.Hash, *types.RawBlockWithExtractedData](blocksLRUSize)
	check.PanicIfErr(err)

	blockNumbersLRU, err := lru.New[types.BlockNumber, common.Hash](blocksLRUSize)
	check.PanicIfErr(err)

	inTxnLRU, err := lru.New[db.BlockHashAndTransactionIndex, *Txn](txnLRUSize)
	check.PanicIfErr(err)

	outTxnLRU, err := lru.New[db.BlockHashAndTransactionIndex, *Txn](txnLRUSize)
	check.PanicIfErr(err)

	inTxnIndexLRU, err := lru.New[common.Hash, db.BlockHashAndTransactionIndex](txnLRUSize)
	check.PanicIfErr(err)

	return &cache{
		headersLRU:      headersLRU,
		fullBlocksLRU:   fullBlocksLRU,
		blockNumbersLRU: blockNumbersLRU,

		inTxnLRU:      inTxnLRU,
		outTxnLRU:     outTxnLRU,
		inTxnIndexLRU: inTxnIndexLRU,
	}
}

func (s RawShardAccessor) collectTxnCounts(tableName db.ShardedTableName, root common.Hash) ([][]byte, error) {
	reader, err := s.mptReader(tableName, root)
	if err != nil {
		return nil, err
	}

	items := make([][]byte, 0, 16)
	for k, v := range reader.Iterate() {
		if len(k) != types.ShardIdSize {
			continue
		}

		var transactionIndex types.TransactionIndex
		if err := transactionIndex.UnmarshalNil(v); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction index for shard %s: %w", k, err)
		}

		txCount := &types.TxCount{
			ShardId: uint16(types.BytesToShardId(k)),
			Count:   transactionIndex,
		}

		item, err := txCount.MarshalNil()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (s RawShardAccessor) collectTxnIndexedEntities(tableName db.ShardedTableName, root common.Hash) ([][]byte, error) {
	reader, err := s.mptReader(tableName, root)
	if err != nil {
		return nil, err
	}

	items := make([][]byte, 0, 1024)
	for index := types.TransactionIndex(0); ; index++ {
		entity, err := reader.Get(index.Bytes())
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		} else if err != nil {
			return nil, err
		}
		items = append(items, entity)
	}

	return items, nil
}

func (s RawShardAccessor) mptReader(tableName db.ShardedTableName, root common.Hash) (*mpt.Reader, error) {
	res := mpt.NewDbReader(s.tx, s.shardId, tableName)
	return res, res.SetRootHash(root)
}

//////// raw block accessor //////////

func (s RawShardAccessor) getHeaderByHash(hash common.Hash) (headerWithRaw, error) {
	if h, ok := s.cache.headersLRU.Get(hash); ok {
		return h, nil
	}

	raw, err := db.ReadBlockBytes(s.tx, s.shardId, hash)
	if err != nil {
		return headerWithRaw{}, err
	}

	block := &types.Block{}
	if err := block.UnmarshalNil(raw); err != nil {
		return headerWithRaw{}, err
	}

	if assert.Enable {
		blockHash := block.Hash(s.shardId)
		check.PanicIfNotf(blockHash == hash, "block hash mismatch: %s != %s", blockHash, hash)
	}

	res := headerWithRaw{
		block: block,
		raw:   raw,
	}
	s.cache.headersLRU.Add(hash, res)

	return res, nil
}

func (s RawShardAccessor) GetBlockHeaderByHash(hash common.Hash) (serialization.EncodedData, error) {
	h, err := s.getHeaderByHash(hash)
	if err != nil {
		return nil, err
	}
	return h.raw, nil
}

func (s RawShardAccessor) GetFullBlockByHash(hash common.Hash) (*types.RawBlockWithExtractedData, error) {
	if rawBlockExt, ok := s.cache.fullBlocksLRU.Get(hash); ok {
		return rawBlockExt, nil
	}

	h, err := s.getHeaderByHash(hash)
	if err != nil {
		return nil, err
	}

	res := &types.RawBlockWithExtractedData{
		Block: h.raw,
	}

	res.InTransactions, err = s.collectTxnIndexedEntities(db.TransactionTrieTable, h.block.InTransactionsRoot)
	if err != nil {
		return nil, err
	}
	res.InTxCounts, err = s.collectTxnCounts(db.TransactionTrieTable, h.block.InTransactionsRoot)
	if err != nil {
		return nil, err
	}

	res.OutTransactions, err = s.collectTxnIndexedEntities(db.TransactionTrieTable, h.block.OutTransactionsRoot)
	if err != nil {
		return nil, err
	}
	res.OutTxCounts, err = s.collectTxnCounts(db.TransactionTrieTable, h.block.OutTransactionsRoot)
	if err != nil {
		return nil, err
	}

	res.Receipts, err = s.collectTxnIndexedEntities(db.ReceiptTrieTable, h.block.ReceiptsRoot)
	if err != nil {
		return nil, err
	}

	res.ChildBlocks, err = s.collectChildBlocks(h.block)
	if err != nil {
		return nil, err
	}

	res.DbTimestamp, err = db.ReadBlockTimestamp(s.tx, s.shardId, hash)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	if s.shardId.IsMainShard() {
		res.Config, err = s.collectConfig(h.block)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (s RawShardAccessor) collectChildBlocks(block *types.Block) ([]common.Hash, error) {
	treeShards := NewDbShardBlocksTrieReader(s.tx, s.shardId, block.Id)
	if err := treeShards.SetRootHash(block.ChildBlocksRootHash); err != nil {
		return nil, err
	}

	shards := make(map[types.ShardId]common.Hash)
	for key, value := range treeShards.Iterate() {
		var hash common.Hash

		shardId := types.BytesToShardId(key)
		hash.SetBytes(value)
		shards[shardId] = hash
	}

	values := make([]common.Hash, len(shards))
	for key, value := range shards {
		values[key-1] = value // the main shard is omitted
	}
	return values, nil
}

func (s RawShardAccessor) collectConfig(block *types.Block) (map[string][]byte, error) {
	reader, err := s.mptReader(db.ConfigTrieTable, block.ConfigRoot)
	if err != nil {
		return nil, err
	}
	configMap := make(map[string][]byte)
	for key, value := range reader.Iterate() {
		configMap[string(key)] = value
	}
	return configMap, nil
}

func (s RawShardAccessor) GetFullBlockByNumber(num types.BlockNumber) (*types.RawBlockWithExtractedData, error) {
	hash, ok := s.cache.blockNumbersLRU.Get(num)
	if !ok {
		var err error
		hash, err = db.ReadBlockHashByNumber(s.tx, s.shardId, num)
		if err != nil {
			return nil, err
		}

		s.cache.blockNumbersLRU.Add(num, hash)
	}

	return s.GetFullBlockByHash(hash)
}

//////// block accessor //////////

func (s ShardAccessor) GetBlockHeaderByHash(hash common.Hash) (*types.Block, error) {
	h, err := s.getHeaderByHash(hash)
	if err != nil {
		return nil, err
	}
	return h.block, nil
}

//////// transaction accessors //////////

type Txn struct {
	Block       *types.BlockWithHash
	Index       types.TransactionIndex
	Transaction *types.Transaction
	RawTxn      []byte

	// Receipt is only set for incoming transactions (the shard does not contain receipts for outgoing transactions).
	Receipt    *types.Receipt
	RawReceipt []byte
}

func (s ShardAccessor) GetOutTxnByIndex(idx types.TransactionIndex, block *types.BlockWithHash) (*Txn, error) {
	return s.getTxnByIndex(false, idx, block)
}

func (s ShardAccessor) GetInTxnByIndex(idx types.TransactionIndex, block *types.BlockWithHash) (*Txn, error) {
	return s.getTxnByIndex(true, idx, block)
}

func (s ShardAccessor) GetInTxnByHash(hash common.Hash) (*Txn, error) {
	idx, err := s.getInTxnIndexByHash(hash)
	if err != nil {
		return nil, err
	}

	block, err := s.GetBlockHeaderByHash(idx.BlockHash)
	if err != nil {
		return nil, err
	}

	res, err := s.getTxnByIndex(true, idx.TransactionIndex, types.NewBlockWithRawHash(block, idx.BlockHash))
	if err != nil {
		return nil, err
	}

	if assert.Enable {
		check.PanicIfNot(res.Transaction == nil || res.Transaction.Hash() == hash)
	}

	return res, nil
}

func (s ShardAccessor) getInTxnIndexByHash(hash common.Hash) (db.BlockHashAndTransactionIndex, error) {
	if idx, ok := s.cache.inTxnIndexLRU.Get(hash); ok {
		return idx, nil
	}

	var idx db.BlockHashAndTransactionIndex

	value, err := s.tx.GetFromShard(s.shardId, db.BlockHashAndInTransactionIndexByTransactionHash, hash.Bytes())
	if err != nil {
		return idx, err
	}

	if err := idx.UnmarshalNil(value); err != nil {
		return idx, err
	}

	s.cache.inTxnIndexLRU.Add(hash, idx)
	return idx, nil
}

func (s ShardAccessor) getTxnByIndex(incoming bool,
	idx types.TransactionIndex, block *types.BlockWithHash,
) (*Txn, error) {
	fullIdx := db.BlockHashAndTransactionIndex{
		BlockHash:        block.Hash,
		TransactionIndex: idx,
	}
	if incoming {
		if txn, ok := s.cache.inTxnLRU.Get(fullIdx); ok {
			return txn, nil
		}
	} else {
		if txn, ok := s.cache.outTxnLRU.Get(fullIdx); ok {
			return txn, nil
		}
	}

	res := &Txn{
		Block:       block,
		Index:       idx,
		Transaction: &types.Transaction{},
	}

	if cached, ok := s.cache.fullBlocksLRU.Get(block.Hash); ok {
		if incoming {
			res.RawTxn = cached.InTransactions[idx]
			res.RawReceipt = cached.Receipts[idx]
		} else {
			res.RawTxn = cached.OutTransactions[idx]
		}
		if err := res.Transaction.UnmarshalNil(res.RawTxn); err != nil {
			return nil, err
		}
		if res.RawReceipt != nil {
			res.Receipt = &types.Receipt{}
			if err := res.Receipt.UnmarshalNil(res.RawReceipt); err != nil {
				return nil, err
			}
		}
		return res, nil
	}

	root := block.InTransactionsRoot
	if !incoming {
		root = block.OutTransactionsRoot
	}

	txnTrie, err := s.mptReader(db.TransactionTrieTable, root)
	if err != nil {
		return nil, err
	}
	res.RawTxn, err = txnTrie.Get(idx.Bytes())
	if err != nil {
		return nil, err
	}
	if err := res.Transaction.UnmarshalNil(res.RawTxn); err != nil {
		return nil, err
	}

	if incoming {
		receiptTrie, err := s.mptReader(db.ReceiptTrieTable, block.ReceiptsRoot)
		if err != nil {
			return nil, err
		}
		res.RawReceipt, err = receiptTrie.Get(idx.Bytes())
		if err != nil {
			return nil, err
		}
		res.Receipt = &types.Receipt{}
		if err := res.Receipt.UnmarshalNil(res.RawReceipt); err != nil {
			return nil, err
		}
	}

	if incoming {
		s.cache.inTxnLRU.Add(fullIdx, res)
	} else {
		s.cache.outTxnLRU.Add(fullIdx, res)
	}

	return res, nil
}
