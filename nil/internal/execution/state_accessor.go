package execution

import (
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
StateAccessor supposed usage:

// Share accessor between goroutines, it is thread-safe.
accessor := execution.NewStateAccessor(1000, 2000)

rawHeader, err := accessor.Access(tx, shardId).GetRawBlockHeaderByHash(hash)

data, err := accessor.Access(tx, shardId).GetFullBlockByNumber(index)
header, txns := data.Block, data.InTransactions
...
*/

type StateAccessor struct {
	txnCache *txnCache

	blockByHash       *BlockByHashAccessor
	blockHashByNumber *BlockHashByNumberAccessor
}

func NewStateAccessor(blockLRUSize, txnLRUSize int) *StateAccessor {
	return &StateAccessor{
		txnCache:          newTxnCache(txnLRUSize),
		blockByHash:       NewBlockByHashAccessor(blockLRUSize),
		blockHashByNumber: NewBlockHashByNumberAccessor(blockLRUSize),
	}
}

func (s *StateAccessor) BlockAccessor() *BlockAccessor {
	return &BlockAccessor{
		headers:      s.blockByHash.headersCache,
		hashByNumber: s.blockHashByNumber,
	}
}

func (s *StateAccessor) Access(tx db.RoTx, shardId types.ShardId) *ShardAccessor {
	return &ShardAccessor{
		cache:             s.txnCache,
		blockByHash:       s.blockByHash,
		blockHashByNumber: s.blockHashByNumber,
		tx:                tx,
		shardId:           shardId,
	}
}

type ShardAccessor struct {
	cache *txnCache

	blockHashByNumber *BlockHashByNumberAccessor
	blockByHash       *BlockByHashAccessor

	tx      db.RoTx
	shardId types.ShardId
}

type txnCache struct {
	inTxnLRU      *lru.Cache[db.BlockHashAndTransactionIndex, *Txn]
	outTxnLRU     *lru.Cache[db.BlockHashAndTransactionIndex, *Txn]
	inTxnIndexLRU *lru.Cache[common.Hash, db.BlockHashAndTransactionIndex]
}

func newTxnCache(txnLRUSize int) *txnCache {
	inTxnLRU, err := lru.New[db.BlockHashAndTransactionIndex, *Txn](txnLRUSize)
	check.PanicIfErr(err)

	outTxnLRU, err := lru.New[db.BlockHashAndTransactionIndex, *Txn](txnLRUSize)
	check.PanicIfErr(err)

	inTxnIndexLRU, err := lru.New[common.Hash, db.BlockHashAndTransactionIndex](txnLRUSize)
	check.PanicIfErr(err)

	return &txnCache{
		inTxnLRU:      inTxnLRU,
		outTxnLRU:     outTxnLRU,
		inTxnIndexLRU: inTxnIndexLRU,
	}
}

//////// block accessors //////////

func (s *ShardAccessor) GetRawBlockHeaderByHash(hash common.Hash) (serialization.EncodedData, error) {
	h, err := s.blockByHash.headersCache.get(s.tx, s.shardId, hash)
	if err != nil {
		return nil, err
	}
	return h.raw, nil
}

func (s *ShardAccessor) GetFullBlockByHash(hash common.Hash) (*types.RawBlockWithExtractedData, error) {
	return s.blockByHash.Get(s.tx, s.shardId, hash)
}

func (s *ShardAccessor) GetBlockHeaderByHash(hash common.Hash) (*types.Block, error) {
	return s.blockByHash.headersCache.Get(s.tx, s.shardId, hash)
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

func (s *ShardAccessor) GetOutTxnByIndex(idx types.TransactionIndex, block *types.BlockWithHash) (*Txn, error) {
	return s.getTxnByIndex(false, idx, block)
}

func (s *ShardAccessor) GetInTxnByIndex(idx types.TransactionIndex, block *types.BlockWithHash) (*Txn, error) {
	return s.getTxnByIndex(true, idx, block)
}

func (s *ShardAccessor) GetInTxnByHash(hash common.Hash) (*Txn, error) {
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
		check.PanicIfNot(res.Transaction.Hash() == hash)
	}

	return res, nil
}

func (s *ShardAccessor) getInTxnIndexByHash(hash common.Hash) (db.BlockHashAndTransactionIndex, error) {
	if idx, ok := s.cache.inTxnIndexLRU.Get(hash); ok {
		return idx, nil
	}

	idx, err := db.ReadTxnNumberByHash(s.tx, s.shardId, hash)
	if err != nil {
		return idx, err
	}

	s.cache.inTxnIndexLRU.Add(hash, idx)
	return idx, nil
}

func (s *ShardAccessor) getTxnByIndex(
	incoming bool, idx types.TransactionIndex, block *types.BlockWithHash,
) (*Txn, error) {
	cache := s.cache.outTxnLRU
	if incoming {
		cache = s.cache.inTxnLRU
	}

	fullIdx := db.BlockHashAndTransactionIndex{
		BlockHash:        block.Hash,
		TransactionIndex: idx,
	}
	if txn, ok := cache.Get(fullIdx); ok {
		return txn, nil
	}

	res, err := s.makeTxn(incoming, idx, block)
	if err != nil {
		return nil, err
	}

	cache.Add(fullIdx, res)
	return res, nil
}

func (s *ShardAccessor) makeTxn(incoming bool, idx types.TransactionIndex, block *types.BlockWithHash) (*Txn, error) {
	res := &Txn{
		Block: block,
		Index: idx,
	}

	if cached, ok := s.blockByHash.cache.Get(block.Hash); ok {
		if incoming {
			res.RawTxn = cached.InTransactions[idx]
			res.RawReceipt = cached.Receipts[idx]
		} else {
			res.RawTxn = cached.OutTransactions[idx]
		}
	} else if incoming {
		if err := s.readTxn(block.InTransactionsRoot, res); err != nil {
			return nil, err
		}
		if err := s.readReceipt(res); err != nil {
			return nil, err
		}
	} else {
		if err := s.readTxn(block.OutTransactionsRoot, res); err != nil {
			return nil, err
		}
	}

	res.Transaction = &types.Transaction{}
	if err := res.Transaction.UnmarshalNil(res.RawTxn); err != nil {
		return nil, err
	}
	if len(res.RawReceipt) > 0 {
		res.Receipt = &types.Receipt{}
		if err := res.Receipt.UnmarshalNil(res.RawReceipt); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (s *ShardAccessor) readTxn(root common.Hash, res *Txn) error {
	txnTrie := mpt.NewDbReader(s.tx, s.shardId, db.TransactionTrieTable)
	err := txnTrie.SetRootHash(root)
	if err != nil {
		return err
	}
	res.RawTxn, err = txnTrie.Get(res.Index.Bytes())
	return err
}

func (s *ShardAccessor) readReceipt(res *Txn) error {
	receiptTrie := mpt.NewDbReader(s.tx, s.shardId, db.ReceiptTrieTable)
	err := receiptTrie.SetRootHash(res.Block.ReceiptsRoot)
	if err != nil {
		return err
	}
	res.RawReceipt, err = receiptTrie.Get(res.Index.Bytes())
	return err
}
