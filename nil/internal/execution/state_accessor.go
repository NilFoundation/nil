package execution

import (
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	nilssz "github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

type fieldAccessor[T any] func() T

func notInitialized[T any](name string) fieldAccessor[T] {
	return func() T { panic(fmt.Sprintf("field not initialized : `%s`", name)) }
}

func initWith[T any](val T) fieldAccessor[T] {
	return func() T { return val }
}

/*
supposed usage is

data, err := accessor.Access(tx, shardId).GetBlock().ByHash(hash)
block := data.Block

data, err := accessor.Access(tx, shardId).GetBlock().ByIndex(index)
block := data.Block

data, err := accessor.Access(tx, shardId).GetBlock().WithInTransactions().ByIndex(index)
block, txns := data.Block, data.InTransactions
...
*/
type StateAccessor struct {
	cache    *accessorCache
	rawCache *rawAccessorCache
}

func NewStateAccessor() *StateAccessor {
	const (
		blocksLRUSize          = 128 // ~32Mb
		inTransactionsLRUSize  = 32
		outTransactionsLRUSize = 32
		receiptsLRUSize        = 32
	)

	return &StateAccessor{
		cache:    newAccessorCache(blocksLRUSize, inTransactionsLRUSize, outTransactionsLRUSize, receiptsLRUSize),
		rawCache: newRawAccessorCache(blocksLRUSize, inTransactionsLRUSize, outTransactionsLRUSize, receiptsLRUSize),
	}
}

func (s *StateAccessor) Access(tx db.RoTx, shardId types.ShardId) *shardAccessor {
	return &shardAccessor{s.RawAccess(tx, shardId)}
}

func (s *StateAccessor) RawAccess(tx db.RoTx, shardId types.ShardId) *rawShardAccessor {
	return &rawShardAccessor{
		cache:    s.cache,
		rawCache: s.rawCache,
		tx:       tx,
		shardId:  shardId,
	}
}

type accessorCache struct {
	blocksLRU          *lru.Cache[common.Hash, *types.Block]
	inTransactionsLRU  *lru.Cache[common.Hash, []*types.Transaction]
	outTransactionsLRU *lru.Cache[common.Hash, []*types.Transaction]
	receiptsLRU        *lru.Cache[common.Hash, []*types.Receipt]
}

func newAccessorCache(
	blocksLRUSize int,
	outTransactionsLRUSize int,
	inTransactionsLRUSize int,
	receiptsLRUSize int,
) *accessorCache {
	blocksLRU, err := lru.New[common.Hash, *types.Block](blocksLRUSize)
	check.PanicIfErr(err)

	outTransactionsLRU, err := lru.New[common.Hash, []*types.Transaction](outTransactionsLRUSize)
	check.PanicIfErr(err)

	inTransactionsLRU, err := lru.New[common.Hash, []*types.Transaction](inTransactionsLRUSize)
	check.PanicIfErr(err)

	receiptsLRU, err := lru.New[common.Hash, []*types.Receipt](receiptsLRUSize)
	check.PanicIfErr(err)

	return &accessorCache{
		blocksLRU:          blocksLRU,
		inTransactionsLRU:  inTransactionsLRU,
		outTransactionsLRU: outTransactionsLRU,
		receiptsLRU:        receiptsLRU,
	}
}

type rawAccessorCache struct {
	blocksLRU          *lru.Cache[common.Hash, []byte]
	inTransactionsLRU  *lru.Cache[common.Hash, [][]byte]
	inTxCountsLRU      *lru.Cache[common.Hash, [][]byte]
	outTransactionsLRU *lru.Cache[common.Hash, [][]byte]
	outTxCountsLRU     *lru.Cache[common.Hash, [][]byte]
	receiptsLRU        *lru.Cache[common.Hash, [][]byte]
}

func newRawAccessorCache(
	blocksLRUSize int,
	outTransactionsLRUSize int,
	inTransactionsLRUSize int,
	receiptsLRUSize int,
) *rawAccessorCache {
	blocksLRU, err := lru.New[common.Hash, []byte](blocksLRUSize)
	check.PanicIfErr(err)

	outTransactionsLRU, err := lru.New[common.Hash, [][]byte](outTransactionsLRUSize)
	check.PanicIfErr(err)

	outTxCountsLRU, err := lru.New[common.Hash, [][]byte](outTransactionsLRUSize)
	check.PanicIfErr(err)

	inTransactionsLRU, err := lru.New[common.Hash, [][]byte](inTransactionsLRUSize)
	check.PanicIfErr(err)

	inTxCountsLRU, err := lru.New[common.Hash, [][]byte](inTransactionsLRUSize)
	check.PanicIfErr(err)

	receiptsLRU, err := lru.New[common.Hash, [][]byte](receiptsLRUSize)
	check.PanicIfErr(err)

	return &rawAccessorCache{
		blocksLRU:          blocksLRU,
		inTransactionsLRU:  inTransactionsLRU,
		inTxCountsLRU:      inTxCountsLRU,
		outTransactionsLRU: outTransactionsLRU,
		outTxCountsLRU:     outTxCountsLRU,
		receiptsLRU:        receiptsLRU,
	}
}

type shardAccessor struct {
	*rawShardAccessor
}

func collectSszShardCounts(
	block common.Hash, sa *rawShardAccessor, cache *lru.Cache[common.Hash, [][]byte],
	tableName db.ShardedTableName, rootHash common.Hash, res *fieldAccessor[[][]byte],
) {
	if items, ok := cache.Get(block); ok {
		*res = initWith(items)
		return
	}
	root := mpt.NewDbReader(sa.tx, sa.shardId, tableName)
	root.SetRootHash(rootHash)

	items := make([][]byte, 0, 16)
	for k, v := range root.Iterate() {
		if len(k) != types.ShardIdSize {
			continue
		}
		// FIXME: this is just byte swapping (shardId.Bytes is big endian)
		shardId := types.BytesToShardId(k)
		shardBytes := ssz.MarshalUint16([]byte{}, uint16(shardId))

		item := make([]byte, 0, len(shardBytes)+len(v))
		item = append(item, shardBytes...)
		item = append(item, v...)
		items = append(items, item)
	}

	*res = initWith(items)
	cache.Add(block, items)
}

func collectSszBlockEntities(
	block common.Hash,
	sa *rawShardAccessor,
	cache *lru.Cache[common.Hash, [][]byte],
	tableName db.ShardedTableName,
	rootHash common.Hash,
	res *fieldAccessor[[][]byte],
) error {
	if items, ok := cache.Get(block); ok {
		*res = initWith(items)
		return nil
	}

	root := mpt.NewDbReader(sa.tx, sa.shardId, tableName)
	root.SetRootHash(rootHash)

	items := make([][]byte, 0, 1024)
	var index types.TransactionIndex
	for {
		entity, err := root.Get(index.Bytes())
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		} else if err != nil {
			return fmt.Errorf("failed to get from %v with index %v from trie: %w", tableName, index, err)
		}
		items = append(items, entity)
		index++
	}

	*res = initWith(items)
	cache.Add(block, items)
	return nil
}

func unmashalSszEntities[
	T interface {
		~*S
		ssz.Unmarshaler
	},
	S any,
](block common.Hash, raw [][]byte, cache *lru.Cache[common.Hash, []*S], res *fieldAccessor[[]*S]) error {
	items, ok := cache.Get(block)
	if !ok {
		var err error
		items, err = nilssz.DecodeContainer[T](raw)
		if err != nil {
			return err
		}
		cache.Add(block, items)
	}

	*res = initWith(items)
	return nil
}

func (s *shardAccessor) mptReader(tableName db.ShardedTableName, rootHash common.Hash) *mpt.Reader {
	res := mpt.NewDbReader(s.tx, s.shardId, tableName)
	res.SetRootHash(rootHash)
	return res
}

func (s *shardAccessor) GetBlock() blockAccessor {
	return blockAccessor{rawBlockAccessor{rawShardAccessor: s.rawShardAccessor}}
}

func (s *shardAccessor) GetInTransaction() inTransactionAccessor {
	return inTransactionAccessor{shardAccessor: s}
}

func (s *shardAccessor) GetOutTransaction() outTransactionAccessor {
	return outTransactionAccessor{shardAccessor: s}
}

//////// raw block accessor //////////

type rawShardAccessor struct {
	cache    *accessorCache
	rawCache *rawAccessorCache
	tx       db.RoTx
	shardId  types.ShardId
}

func (s *rawShardAccessor) GetBlock() rawBlockAccessor {
	return rawBlockAccessor{rawShardAccessor: s}
}

type rawBlockAccessorResult struct {
	block           fieldAccessor[[]byte]
	inTransactions  fieldAccessor[[][]byte]
	inTxCounts      fieldAccessor[[][]byte]
	outTransactions fieldAccessor[[][]byte]
	outTxCounts     fieldAccessor[[][]byte]
	receipts        fieldAccessor[[][]byte]
	childBlocks     fieldAccessor[[]common.Hash]
	dbTimestamp     fieldAccessor[uint64]
	config          fieldAccessor[map[string][]byte]
}

func (r rawBlockAccessorResult) Block() []byte {
	return r.block()
}

func (r rawBlockAccessorResult) InTransactions() [][]byte {
	return r.inTransactions()
}

func (r rawBlockAccessorResult) InTxCounts() [][]byte {
	return r.inTxCounts()
}

func (r rawBlockAccessorResult) OutTransactions() [][]byte {
	return r.outTransactions()
}

func (r rawBlockAccessorResult) OutTxCounts() [][]byte {
	return r.outTxCounts()
}

func (r rawBlockAccessorResult) Receipts() [][]byte {
	return r.receipts()
}

func (r rawBlockAccessorResult) ChildBlocks() []common.Hash {
	return r.childBlocks()
}

func (r rawBlockAccessorResult) DbTimestamp() uint64 {
	return r.dbTimestamp()
}

func (r rawBlockAccessorResult) Config() map[string][]byte {
	return r.config()
}

type rawBlockAccessor struct {
	rawShardAccessor    *rawShardAccessor
	withInTransactions  bool
	withOutTransactions bool
	withReceipts        bool
	withChildBlocks     bool
	withDbTimestamp     bool
	withConfig          bool
}

func (b rawBlockAccessor) WithChildBlocks() rawBlockAccessor {
	b.withChildBlocks = true
	return b
}

func (b rawBlockAccessor) WithInTransactions() rawBlockAccessor {
	b.withInTransactions = true
	return b
}

func (b rawBlockAccessor) WithOutTransactions() rawBlockAccessor {
	b.withOutTransactions = true
	return b
}

func (b rawBlockAccessor) WithReceipts() rawBlockAccessor {
	b.withReceipts = true
	return b
}

func (b rawBlockAccessor) WithDbTimestamp() rawBlockAccessor {
	b.withDbTimestamp = true
	return b
}

func (b rawBlockAccessor) WithConfig() rawBlockAccessor {
	b.withConfig = true
	return b
}

func (b rawBlockAccessor) decodeBlock(hash common.Hash, data []byte) (*types.Block, error) {
	sa := b.rawShardAccessor
	block, ok := sa.cache.blocksLRU.Get(hash)
	if !ok {
		block = &types.Block{}
		if err := block.UnmarshalSSZ(data); err != nil {
			return nil, err
		}
		sa.cache.blocksLRU.Add(hash, block)
	}
	return block, nil
}

func (b rawBlockAccessor) ByHash(hash common.Hash) (rawBlockAccessorResult, error) {
	sa := b.rawShardAccessor

	// Extract raw block
	rawBlock, ok := sa.rawCache.blocksLRU.Get(hash)
	if !ok {
		var err error
		rawBlock, err = db.ReadBlockSSZ(sa.tx, sa.shardId, hash)
		if err != nil {
			return rawBlockAccessorResult{}, err
		}
		sa.rawCache.blocksLRU.Add(hash, rawBlock)
	}

	// We need to decode some block data anyway
	block, err := b.decodeBlock(hash, rawBlock)
	if err != nil {
		return rawBlockAccessorResult{}, err
	}

	res := rawBlockAccessorResult{
		block:           initWith(rawBlock),
		inTransactions:  notInitialized[[][]byte]("InTransactions"),
		inTxCounts:      notInitialized[[][]byte]("InTxCounts"),
		outTransactions: notInitialized[[][]byte]("OutTransactions"),
		outTxCounts:     notInitialized[[][]byte]("OutTxCounts"),
		receipts:        notInitialized[[][]byte]("Receipts"),
		childBlocks:     notInitialized[[]common.Hash]("ChildBlocks"),
		dbTimestamp:     notInitialized[uint64]("DbTimestamp"),
		config:          notInitialized[map[string][]byte]("Config"),
	}

	if b.withInTransactions {
		if err := collectSszBlockEntities(
			hash,
			sa,
			sa.rawCache.inTransactionsLRU,
			db.TransactionTrieTable,
			block.InTransactionsRoot,
			&res.inTransactions,
		); err != nil {
			return rawBlockAccessorResult{}, err
		}
		collectSszShardCounts(
			hash, sa, sa.rawCache.inTxCountsLRU, db.TransactionTrieTable,
			block.InTransactionsRoot, &res.inTxCounts,
		)
	}

	if b.withOutTransactions {
		if err := collectSszBlockEntities(
			hash,
			sa,
			sa.rawCache.outTransactionsLRU,
			db.TransactionTrieTable,
			block.OutTransactionsRoot,
			&res.outTransactions,
		); err != nil {
			return rawBlockAccessorResult{}, err
		}
		collectSszShardCounts(
			hash, sa, sa.rawCache.outTxCountsLRU, db.TransactionTrieTable,
			block.OutTransactionsRoot, &res.outTxCounts,
		)
	}

	if b.withReceipts {
		if err := collectSszBlockEntities(
			hash,
			sa,
			sa.rawCache.receiptsLRU,
			db.ReceiptTrieTable,
			block.ReceiptsRoot,
			&res.receipts,
		); err != nil {
			return rawBlockAccessorResult{}, err
		}
	}

	if b.withChildBlocks {
		treeShards := NewDbShardBlocksTrieReader(sa.tx, sa.shardId, block.Id)
		treeShards.SetRootHash(block.ChildBlocksRootHash)

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
		res.childBlocks = initWith(values)
	}

	if b.withDbTimestamp {
		ts, err := db.ReadBlockTimestamp(sa.tx, sa.shardId, hash)
		// This is needed for old blocks that don't have their timestamp stored
		if errors.Is(err, db.ErrKeyNotFound) {
			ts = types.InvalidDbTimestamp
		} else if err != nil {
			return rawBlockAccessorResult{}, err
		}

		res.dbTimestamp = initWith(ts)
	}

	// config is included only for main shard, empty for others
	if b.withConfig {
		root := mpt.NewDbReader(sa.tx, sa.shardId, db.ConfigTrieTable)
		root.SetRootHash(block.ConfigRoot)
		configMap := make(map[string][]byte)
		for key, value := range root.Iterate() {
			configMap[string(key)] = value
		}
		res.config = initWith(configMap)
	}

	return res, nil
}

func (b rawBlockAccessor) ByNumber(num types.BlockNumber) (rawBlockAccessorResult, error) {
	sa := b.rawShardAccessor
	hash, err := db.ReadBlockHashByNumber(sa.tx, sa.shardId, num)
	if err != nil {
		return rawBlockAccessorResult{}, err
	}
	return b.ByHash(hash)
}

//////// block accessor //////////

type blockAccessorResult struct {
	block           fieldAccessor[*types.Block]
	inTransactions  fieldAccessor[[]*types.Transaction]
	outTransactions fieldAccessor[[]*types.Transaction]
	receipts        fieldAccessor[[]*types.Receipt]
	childBlocks     fieldAccessor[[]common.Hash]
	dbTimestamp     fieldAccessor[uint64]
	config          fieldAccessor[map[string][]byte]
}

func (r blockAccessorResult) Block() *types.Block {
	return r.block()
}

func (r blockAccessorResult) InTransactions() []*types.Transaction {
	return r.inTransactions()
}

func (r blockAccessorResult) OutTransactions() []*types.Transaction {
	return r.outTransactions()
}

func (r blockAccessorResult) Receipts() []*types.Receipt {
	return r.receipts()
}

func (r blockAccessorResult) ChildBlocks() []common.Hash {
	return r.childBlocks()
}

func (r blockAccessorResult) DbTimestamp() uint64 {
	return r.dbTimestamp()
}

func (r blockAccessorResult) Config() map[string][]byte {
	return r.config()
}

type blockAccessor struct {
	rawBlockAccessor
}

func (b blockAccessor) WithChildBlocks() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithChildBlocks()}
}

func (b blockAccessor) WithInTransactions() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithInTransactions()}
}

func (b blockAccessor) WithOutTransactions() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithOutTransactions()}
}

func (b blockAccessor) WithReceipts() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithReceipts()}
}

func (b blockAccessor) WithDbTimestamp() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithDbTimestamp()}
}

func (b blockAccessor) WithConfig() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithConfig()}
}

func (b blockAccessor) ByHash(hash common.Hash) (blockAccessorResult, error) {
	sa := b.rawShardAccessor

	raw, err := b.rawBlockAccessor.ByHash(hash)
	if err != nil {
		return blockAccessorResult{}, err
	}

	block, err := b.decodeBlock(hash, raw.Block())
	if err != nil {
		return blockAccessorResult{}, err
	}

	res := blockAccessorResult{
		block:           initWith(block),
		inTransactions:  notInitialized[[]*types.Transaction]("InTransactions"),
		outTransactions: notInitialized[[]*types.Transaction]("OutTransactions"),
		receipts:        notInitialized[[]*types.Receipt]("Receipts"),
		childBlocks:     notInitialized[[]common.Hash]("ChildBlocks"),
		dbTimestamp:     notInitialized[uint64]("DbTimestamp"),
		config:          notInitialized[map[string][]byte]("Config"),
	}

	if b.withInTransactions {
		if err := unmashalSszEntities[*types.Transaction](
			hash,
			raw.InTransactions(),
			sa.cache.inTransactionsLRU,
			&res.inTransactions,
		); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withOutTransactions {
		if err := unmashalSszEntities[*types.Transaction](
			hash,
			raw.OutTransactions(),
			sa.cache.outTransactionsLRU,
			&res.outTransactions,
		); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withReceipts {
		if err := unmashalSszEntities[*types.Receipt](
			hash, raw.Receipts(), sa.cache.receiptsLRU, &res.receipts,
		); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withChildBlocks {
		res.childBlocks = initWith(raw.ChildBlocks())
	}

	if b.withDbTimestamp {
		res.dbTimestamp = initWith(raw.DbTimestamp())
	}

	if b.withConfig {
		res.config = initWith(raw.Config())
	}

	return res, nil
}

func (b blockAccessor) ByNumber(num types.BlockNumber) (blockAccessorResult, error) {
	sa := b.rawShardAccessor
	hash, err := db.ReadBlockHashByNumber(sa.tx, sa.shardId, num)
	if err != nil {
		return blockAccessorResult{}, err
	}
	return b.ByHash(hash)
}

//////// transaction accessors //////////

type transactionAccessorResult struct {
	block       fieldAccessor[*types.Block]
	index       fieldAccessor[types.TransactionIndex]
	transaction fieldAccessor[*types.Transaction]
}

func (r transactionAccessorResult) Block() *types.Block {
	return r.block()
}

func (r transactionAccessorResult) Index() types.TransactionIndex {
	return r.index()
}

func (r transactionAccessorResult) Transaction() *types.Transaction {
	return r.transaction()
}

func getBlockAndInTxnIndexByHash(
	sa *shardAccessor,
	incoming bool,
	hash common.Hash,
) (*types.Block, db.BlockHashAndTransactionIndex, error) {
	var idx db.BlockHashAndTransactionIndex

	table := db.BlockHashAndInTransactionIndexByTransactionHash
	if !incoming {
		table = db.BlockHashAndOutTransactionIndexByTransactionHash
	}

	value, err := sa.tx.GetFromShard(sa.shardId, table, hash.Bytes())
	if err != nil {
		return nil, idx, err
	}

	if err = idx.UnmarshalSSZ(value); err != nil {
		return nil, idx, err
	}

	data, err := sa.GetBlock().ByHash(idx.BlockHash)
	if err != nil {
		return nil, idx, err
	}

	return data.Block(), idx, nil
}

func baseGetTxnByHash(sa *shardAccessor, incoming bool, hash common.Hash) (transactionAccessorResult, error) {
	block, idx, err := getBlockAndInTxnIndexByHash(sa, incoming, hash)
	if err != nil {
		return transactionAccessorResult{}, err
	}

	data, err := baseGetTxnByIndex(sa, incoming, idx.TransactionIndex, block)
	if err != nil {
		return transactionAccessorResult{}, err
	}
	if assert.Enable {
		check.PanicIfNot(data.Transaction() == nil || data.Transaction().Hash() == hash)
	}
	return data, nil
}

func baseGetTxnByIndex(
	sa *shardAccessor,
	incoming bool,
	idx types.TransactionIndex,
	block *types.Block,
) (transactionAccessorResult, error) {
	root := block.InTransactionsRoot
	if !incoming {
		root = block.OutTransactionsRoot
	}
	txnTrie := sa.mptReader(db.TransactionTrieTable, root)
	txn, err := mpt.GetEntity[*types.Transaction](txnTrie, idx.Bytes())
	if err != nil {
		return transactionAccessorResult{}, err
	}

	return transactionAccessorResult{block: initWith(block), index: initWith(idx), transaction: initWith(txn)}, nil
}

type outTransactionAccessorResult struct {
	transactionAccessorResult
}

type outTransactionAccessor struct {
	shardAccessor *shardAccessor
}

func (a outTransactionAccessor) ByHash(hash common.Hash) (outTransactionAccessorResult, error) {
	data, err := baseGetTxnByHash(a.shardAccessor, false, hash)
	return outTransactionAccessorResult{data}, err
}

func (a outTransactionAccessor) ByIndex(
	idx types.TransactionIndex,
	block *types.Block,
) (outTransactionAccessorResult, error) {
	data, err := baseGetTxnByIndex(a.shardAccessor, false, idx, block)
	return outTransactionAccessorResult{data}, err
}

type inTransactionAccessorResult struct {
	transactionAccessorResult
	receipt fieldAccessor[*types.Receipt]
}

func (r inTransactionAccessorResult) Receipt() *types.Receipt {
	return r.receipt()
}

type inTransactionAccessor struct {
	shardAccessor *shardAccessor
	withReceipt   bool
}

func (a inTransactionAccessor) WithReceipt() inTransactionAccessor {
	a.withReceipt = true
	return a
}

func (a inTransactionAccessor) ByHash(hash common.Hash) (inTransactionAccessorResult, error) {
	data, err := baseGetTxnByHash(a.shardAccessor, true, hash)
	if err != nil {
		return inTransactionAccessorResult{}, err
	}

	res := inTransactionAccessorResult{
		transactionAccessorResult: data,
		receipt:                   notInitialized[*types.Receipt]("Receipt"),
	}

	if a.withReceipt {
		return a.addReceipt(res)
	}

	return res, nil
}

func (a inTransactionAccessor) ByIndex(
	idx types.TransactionIndex,
	block *types.Block,
) (inTransactionAccessorResult, error) {
	data, err := baseGetTxnByIndex(a.shardAccessor, true, idx, block)
	if err != nil {
		return inTransactionAccessorResult{}, err
	}

	res := inTransactionAccessorResult{
		transactionAccessorResult: data,
		receipt:                   notInitialized[*types.Receipt]("Receipt"),
	}

	if a.withReceipt {
		return a.addReceipt(res)
	}
	return res, nil
}

func (a inTransactionAccessor) addReceipt(
	accessResult inTransactionAccessorResult,
) (inTransactionAccessorResult, error) {
	if accessResult.Block() == nil {
		accessResult.receipt = initWith[*types.Receipt](nil)
		return accessResult, nil
	}
	receiptTrie := a.shardAccessor.mptReader(db.ReceiptTrieTable, accessResult.Block().ReceiptsRoot)
	receipt, err := mpt.GetEntity[*types.Receipt](receiptTrie, accessResult.Index().Bytes())
	if err != nil {
		return inTransactionAccessorResult{}, err
	}
	accessResult.receipt = initWith(receipt)
	return accessResult, nil
}
