package execution

import (
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	nilssz "github.com/NilFoundation/nil/nil/common/ssz"
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

data, err := accessor.Access(tx, shardId).GetBlock().WithInMessages().ByIndex(index)
block, msgs := data.Block, data.InMessages
...
*/
type StateAccessor struct {
	cache    *accessorCache
	rawCache *rawAccessorCache
}

func NewStateAccessor() (*StateAccessor, error) {
	const (
		blocksLRUSize      = 128 // ~32Mb
		inMessagesLRUSize  = 32
		outMessagesLRUSize = 32
		receiptsLRUSize    = 32
	)

	return &StateAccessor{
		cache:    newAccessorCache(blocksLRUSize, inMessagesLRUSize, outMessagesLRUSize, receiptsLRUSize),
		rawCache: newRawAccessorCache(blocksLRUSize, inMessagesLRUSize, outMessagesLRUSize, receiptsLRUSize),
	}, nil
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
	blocksLRU      *lru.Cache[common.Hash, *types.Block]
	inMessagesLRU  *lru.Cache[common.Hash, []*types.Message]
	outMessagesLRU *lru.Cache[common.Hash, []*types.Message]
	receiptsLRU    *lru.Cache[common.Hash, []*types.Receipt]
}

func newAccessorCache(blocksLRUSize, outMessagesLRUSize, inMessagesLRUSize, receiptsLRUSize int) *accessorCache {
	blocksLRU, err := lru.New[common.Hash, *types.Block](blocksLRUSize)
	check.PanicIfErr(err)

	outMessagesLRU, err := lru.New[common.Hash, []*types.Message](outMessagesLRUSize)
	check.PanicIfErr(err)

	inMessagesLRU, err := lru.New[common.Hash, []*types.Message](inMessagesLRUSize)
	check.PanicIfErr(err)

	receiptsLRU, err := lru.New[common.Hash, []*types.Receipt](receiptsLRUSize)
	check.PanicIfErr(err)

	return &accessorCache{
		blocksLRU:      blocksLRU,
		inMessagesLRU:  inMessagesLRU,
		outMessagesLRU: outMessagesLRU,
		receiptsLRU:    receiptsLRU,
	}
}

type rawAccessorCache struct {
	blocksLRU      *lru.Cache[common.Hash, []byte]
	inMessagesLRU  *lru.Cache[common.Hash, [][]byte]
	outMessagesLRU *lru.Cache[common.Hash, [][]byte]
	receiptsLRU    *lru.Cache[common.Hash, [][]byte]
}

func newRawAccessorCache(blocksLRUSize, outMessagesLRUSize, inMessagesLRUSize, receiptsLRUSize int) *rawAccessorCache {
	blocksLRU, err := lru.New[common.Hash, []byte](blocksLRUSize)
	check.PanicIfErr(err)

	outMessagesLRU, err := lru.New[common.Hash, [][]byte](outMessagesLRUSize)
	check.PanicIfErr(err)

	inMessagesLRU, err := lru.New[common.Hash, [][]byte](inMessagesLRUSize)
	check.PanicIfErr(err)

	receiptsLRU, err := lru.New[common.Hash, [][]byte](receiptsLRUSize)
	check.PanicIfErr(err)

	return &rawAccessorCache{
		blocksLRU:      blocksLRU,
		inMessagesLRU:  inMessagesLRU,
		outMessagesLRU: outMessagesLRU,
		receiptsLRU:    receiptsLRU,
	}
}

type shardAccessor struct {
	*rawShardAccessor
}

func collectSszBlockEntities(
	block common.Hash, sa *rawShardAccessor, cache *lru.Cache[common.Hash, [][]byte], tableName db.ShardedTableName, rootHash common.Hash, res *fieldAccessor[[][]byte],
) error {
	if items, ok := cache.Get(block); ok {
		*res = initWith(items)
		return nil
	}

	root := mpt.NewDbReader(sa.tx, sa.shardId, tableName)
	root.SetRootHash(rootHash)

	items := make([][]byte, 0, 1024)
	var index types.MessageIndex
	for {
		entity, err := root.Get(index.Bytes())
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		} else if err != nil {
			return fmt.Errorf("failed to get from %v with index %v from trie: %w", tableName, index, err)
		}
		items = append(items, entity)
		index += 1
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

func (s *shardAccessor) GetInMessage() inMessageAccessor {
	return inMessageAccessor{shardAccessor: s}
}

func (s *shardAccessor) GetOutMessage() outMessageAccessor {
	return outMessageAccessor{shardAccessor: s}
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
	block       fieldAccessor[[]byte]
	inMessages  fieldAccessor[[][]byte]
	outMessages fieldAccessor[[][]byte]
	receipts    fieldAccessor[[][]byte]
	childBlocks fieldAccessor[[]common.Hash]
	dbTimestamp fieldAccessor[uint64]
}

func (r rawBlockAccessorResult) Block() []byte {
	return r.block()
}

func (r rawBlockAccessorResult) InMessages() [][]byte {
	return r.inMessages()
}

func (r rawBlockAccessorResult) OutMessages() [][]byte {
	return r.outMessages()
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

type rawBlockAccessor struct {
	rawShardAccessor *rawShardAccessor
	withInMessages   bool
	withOutMessages  bool
	withReceipts     bool
	withChildBlocks  bool
	withDbTimestamp  bool
}

func (b rawBlockAccessor) WithChildBlocks() rawBlockAccessor {
	b.withChildBlocks = true
	return b
}

func (b rawBlockAccessor) WithInMessages() rawBlockAccessor {
	b.withInMessages = true
	return b
}

func (b rawBlockAccessor) WithOutMessages() rawBlockAccessor {
	b.withOutMessages = true
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
		block:       initWith(rawBlock),
		inMessages:  notInitialized[[][]byte]("InMessages"),
		outMessages: notInitialized[[][]byte]("OutMessages"),
		receipts:    notInitialized[[][]byte]("Receipts"),
		childBlocks: notInitialized[[]common.Hash]("ChildBlocks"),
		dbTimestamp: notInitialized[uint64]("DbTimestamp"),
	}

	if b.withInMessages {
		if err := collectSszBlockEntities(hash, sa, sa.rawCache.inMessagesLRU, db.MessageTrieTable, block.InMessagesRoot, &res.inMessages); err != nil {
			return rawBlockAccessorResult{}, err
		}
	}

	if b.withOutMessages {
		if err := collectSszBlockEntities(hash, sa, sa.rawCache.outMessagesLRU, db.MessageTrieTable, block.OutMessagesRoot, &res.outMessages); err != nil {
			return rawBlockAccessorResult{}, err
		}
	}

	if b.withReceipts {
		if err := collectSszBlockEntities(hash, sa, sa.rawCache.receiptsLRU, db.ReceiptTrieTable, block.ReceiptsRoot, &res.receipts); err != nil {
			return rawBlockAccessorResult{}, err
		}
	}

	if b.withChildBlocks {
		treeShards := NewDbShardBlocksTrieReader(sa.tx, sa.shardId, block.Id)
		treeShards.SetRootHash(block.ChildBlocksRootHash)
		valuePtrs, err := treeShards.Values()
		if err != nil {
			return rawBlockAccessorResult{}, err
		}

		values := make([]common.Hash, 0, len(valuePtrs))
		for _, ptr := range valuePtrs {
			values = append(values, *ptr)
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
	block       fieldAccessor[*types.Block]
	inMessages  fieldAccessor[[]*types.Message]
	outMessages fieldAccessor[[]*types.Message]
	receipts    fieldAccessor[[]*types.Receipt]
	childBlocks fieldAccessor[[]common.Hash]
	dbTimestamp fieldAccessor[uint64]
}

func (r blockAccessorResult) Block() *types.Block {
	return r.block()
}

func (r blockAccessorResult) InMessages() []*types.Message {
	return r.inMessages()
}

func (r blockAccessorResult) OutMessages() []*types.Message {
	return r.outMessages()
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

type blockAccessor struct {
	rawBlockAccessor
}

func (b blockAccessor) WithChildBlocks() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithChildBlocks()}
}

func (b blockAccessor) WithInMessages() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithInMessages()}
}

func (b blockAccessor) WithOutMessages() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithOutMessages()}
}

func (b blockAccessor) WithReceipts() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithReceipts()}
}

func (b blockAccessor) WithDbTimestamp() blockAccessor {
	return blockAccessor{b.rawBlockAccessor.WithDbTimestamp()}
}

func (b blockAccessor) ByHash(hash common.Hash) (blockAccessorResult, error) {
	sa := b.rawBlockAccessor.rawShardAccessor

	raw, err := b.rawBlockAccessor.ByHash(hash)
	if err != nil {
		return blockAccessorResult{}, err
	}

	block, err := b.decodeBlock(hash, raw.Block())
	if err != nil {
		return blockAccessorResult{}, err
	}

	res := blockAccessorResult{
		block:       initWith(block),
		inMessages:  notInitialized[[]*types.Message]("InMessages"),
		outMessages: notInitialized[[]*types.Message]("OutMessages"),
		receipts:    notInitialized[[]*types.Receipt]("Receipts"),
		childBlocks: notInitialized[[]common.Hash]("ChildBlocks"),
		dbTimestamp: notInitialized[uint64]("DbTimestamp"),
	}

	if b.withInMessages {
		if err := unmashalSszEntities[*types.Message](hash, raw.InMessages(), sa.cache.inMessagesLRU, &res.inMessages); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withOutMessages {
		if err := unmashalSszEntities[*types.Message](hash, raw.OutMessages(), sa.cache.outMessagesLRU, &res.outMessages); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withReceipts {
		if err := unmashalSszEntities[*types.Receipt](hash, raw.Receipts(), sa.cache.receiptsLRU, &res.receipts); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withChildBlocks {
		res.childBlocks = initWith(raw.ChildBlocks())
	}

	if b.withDbTimestamp {
		res.dbTimestamp = initWith(raw.DbTimestamp())
	}

	return res, nil
}

func (b blockAccessor) ByNumber(num types.BlockNumber) (blockAccessorResult, error) {
	sa := b.rawBlockAccessor.rawShardAccessor
	hash, err := db.ReadBlockHashByNumber(sa.tx, sa.shardId, num)
	if err != nil {
		return blockAccessorResult{}, err
	}
	return b.ByHash(hash)
}

//////// message accessors //////////

type messageAccessorResult struct {
	block   fieldAccessor[*types.Block]
	index   fieldAccessor[types.MessageIndex]
	message fieldAccessor[*types.Message]
}

func (r messageAccessorResult) Block() *types.Block {
	return r.block()
}

func (r messageAccessorResult) Index() types.MessageIndex {
	return r.index()
}

func (r messageAccessorResult) Message() *types.Message {
	return r.message()
}

func getBlockAndInMsgIndexByHash(sa *shardAccessor, incoming bool, hash common.Hash) (*types.Block, db.BlockHashAndMessageIndex, error) {
	var idx db.BlockHashAndMessageIndex

	table := db.BlockHashAndInMessageIndexByMessageHash
	if !incoming {
		table = db.BlockHashAndOutMessageIndexByMessageHash
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

func baseGetMsgByHash(sa *shardAccessor, incoming bool, hash common.Hash) (messageAccessorResult, error) {
	block, idx, err := getBlockAndInMsgIndexByHash(sa, incoming, hash)
	if err != nil {
		return messageAccessorResult{}, err
	}

	data, err := baseGetMsgByIndex(sa, incoming, idx.MessageIndex, block)
	if err != nil {
		return messageAccessorResult{}, err
	}
	if assert.Enable {
		check.PanicIfNot(data.Message() == nil || data.Message().Hash() == hash)
	}
	return data, nil
}

func baseGetMsgByIndex(sa *shardAccessor, incoming bool, idx types.MessageIndex, block *types.Block) (messageAccessorResult, error) {
	root := block.InMessagesRoot
	if !incoming {
		root = block.OutMessagesRoot
	}
	msgTrie := sa.mptReader(db.MessageTrieTable, root)
	msg, err := mpt.GetEntity[*types.Message](msgTrie, idx.Bytes())
	if err != nil {
		return messageAccessorResult{}, err
	}

	return messageAccessorResult{block: initWith(block), index: initWith(idx), message: initWith(msg)}, nil
}

type outMessageAccessorResult struct {
	messageAccessorResult
}

type outMessageAccessor struct {
	shardAccessor *shardAccessor
}

func (a outMessageAccessor) ByHash(hash common.Hash) (outMessageAccessorResult, error) {
	data, err := baseGetMsgByHash(a.shardAccessor, false, hash)
	return outMessageAccessorResult{data}, err
}

func (a outMessageAccessor) ByIndex(idx types.MessageIndex, block *types.Block) (outMessageAccessorResult, error) {
	data, err := baseGetMsgByIndex(a.shardAccessor, false, idx, block)
	return outMessageAccessorResult{data}, err
}

type inMessageAccessorResult struct {
	messageAccessorResult
	receipt fieldAccessor[*types.Receipt]
}

func (r inMessageAccessorResult) Receipt() *types.Receipt {
	return r.receipt()
}

type inMessageAccessor struct {
	shardAccessor *shardAccessor
	withReceipt   bool
}

func (a inMessageAccessor) WithReceipt() inMessageAccessor {
	a.withReceipt = true
	return a
}

func (a inMessageAccessor) ByHash(hash common.Hash) (inMessageAccessorResult, error) {
	data, err := baseGetMsgByHash(a.shardAccessor, true, hash)
	if err != nil {
		return inMessageAccessorResult{}, err
	}

	res := inMessageAccessorResult{
		messageAccessorResult: data,
		receipt:               notInitialized[*types.Receipt]("Receipt"),
	}

	if a.withReceipt {
		return a.addReceipt(res)
	}

	return res, nil
}

func (a inMessageAccessor) ByIndex(idx types.MessageIndex, block *types.Block) (inMessageAccessorResult, error) {
	data, err := baseGetMsgByIndex(a.shardAccessor, true, idx, block)
	if err != nil {
		return inMessageAccessorResult{}, err
	}

	res := inMessageAccessorResult{
		messageAccessorResult: data,
		receipt:               notInitialized[*types.Receipt]("Receipt"),
	}

	if a.withReceipt {
		return a.addReceipt(res)
	}
	return res, nil
}

func (a inMessageAccessor) addReceipt(accessResult inMessageAccessorResult) (inMessageAccessorResult, error) {
	if accessResult.Block() == nil {
		accessResult.receipt = initWith[*types.Receipt](nil)
		return accessResult, nil
	}
	receiptTrie := a.shardAccessor.mptReader(db.ReceiptTrieTable, accessResult.Block().ReceiptsRoot)
	receipt, err := mpt.GetEntity[*types.Receipt](receiptTrie, accessResult.Index().Bytes())
	if err != nil {
		return inMessageAccessorResult{}, err
	}
	accessResult.receipt = initWith(receipt)
	return accessResult, nil
}
