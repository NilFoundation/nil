package execution

import (
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
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
	cache *accessorCache
}

func NewStateAccessor() (*StateAccessor, error) {
	const (
		blocksLRUSize      = 128 // ~32Mb
		inMessagesLRUSize  = 32
		outMessagesLRUSize = 32
		receiptsLRUSize    = 32
		childBlocksLRUSize = 32
	)

	blocksLRU, err := lru.New[common.Hash, *types.Block](blocksLRUSize)
	if err != nil {
		return nil, err
	}
	outMessagesLRU, err := lru.New[common.Hash, []*types.Message](outMessagesLRUSize)
	if err != nil {
		return nil, err
	}
	inMessagesLRU, err := lru.New[common.Hash, []*types.Message](inMessagesLRUSize)
	if err != nil {
		return nil, err
	}
	receiptsLRU, err := lru.New[common.Hash, []*types.Receipt](receiptsLRUSize)
	if err != nil {
		return nil, err
	}
	childBlocksLRU, err := lru.New[common.Hash, []*common.Hash](childBlocksLRUSize)
	if err != nil {
		return nil, err
	}

	return &StateAccessor{&accessorCache{
		blocksLRU:      blocksLRU,
		inMessagesLRU:  inMessagesLRU,
		outMessagesLRU: outMessagesLRU,
		receiptsLRU:    receiptsLRU,
		childBlocksLRU: childBlocksLRU,
	}}, nil
}

func (s *StateAccessor) Access(tx db.RoTx, shardId types.ShardId) *shardAccessor {
	return &shardAccessor{cache: s.cache, tx: tx, shardId: shardId}
}

type accessorCache struct {
	blocksLRU      *lru.Cache[common.Hash, *types.Block]
	inMessagesLRU  *lru.Cache[common.Hash, []*types.Message]
	outMessagesLRU *lru.Cache[common.Hash, []*types.Message]
	receiptsLRU    *lru.Cache[common.Hash, []*types.Receipt]
	childBlocksLRU *lru.Cache[common.Hash, []*common.Hash]
}

type shardAccessor struct {
	cache   *accessorCache
	tx      db.RoTx
	shardId types.ShardId
}

func collectBlockEntities[
	T interface {
		~*S
		ssz.Unmarshaler
	},
	S any,
](block common.Hash, sa *shardAccessor, cache *lru.Cache[common.Hash, []*S], tableName db.ShardedTableName, rootHash common.Hash, res *fieldAccessor[[]*S]) error {
	if items, ok := cache.Get(block); ok {
		*res = initWith(items)
		return nil
	}

	root := mpt.NewReaderWithRoot(sa.tx, sa.shardId, tableName, rootHash)

	items := make([]*S, 0, 1024)
	var index uint64
	for {
		k := ssz.MarshalUint64(nil, index)

		entity, err := mpt.GetEntity[T](root, k)
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

func (s *shardAccessor) mptReader(tableName db.ShardedTableName, rootHash common.Hash) *mpt.Reader {
	return mpt.NewReaderWithRoot(s.tx, s.shardId, tableName, rootHash)
}

func (s *shardAccessor) GetBlock() blockAccessor {
	return blockAccessor{shardAccessor: s}
}

func (s *shardAccessor) GetInMessage() inMessageAccessor {
	return inMessageAccessor{shardAccessor: s}
}

func (s *shardAccessor) GetOutMessage() outMessageAccessor {
	return outMessageAccessor{shardAccessor: s}
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
	shardAccessor   *shardAccessor
	withInMessages  bool
	withOutMessages bool
	withReceipts    bool
	withChildBlocks bool
	withDbTimestamp bool
}

func (b blockAccessor) WithChildBlocks() blockAccessor {
	b.withChildBlocks = true
	return b
}

func (b blockAccessor) WithInMessages() blockAccessor {
	b.withInMessages = true
	return b
}

func (b blockAccessor) WithOutMessages() blockAccessor {
	b.withOutMessages = true
	return b
}

func (b blockAccessor) WithReceipts() blockAccessor {
	b.withReceipts = true
	return b
}

func (b blockAccessor) WithDbTimestamp() blockAccessor {
	b.withDbTimestamp = true
	return b
}

func (b blockAccessor) ByHash(hash common.Hash) (blockAccessorResult, error) {
	sa := b.shardAccessor
	block, ok := sa.cache.blocksLRU.Get(hash)
	if !ok {
		var err error
		block, err = db.ReadBlock(sa.tx, b.shardAccessor.shardId, hash)
		if err != nil {
			return blockAccessorResult{}, err
		}
	}

	res := blockAccessorResult{
		block:       initWith(block),
		inMessages:  notInitialized[[]*types.Message]("InMessages"),
		outMessages: notInitialized[[]*types.Message]("OutMessages"),
		receipts:    notInitialized[[]*types.Receipt]("Receipts"),
		childBlocks: notInitialized[[]common.Hash]("ChildBlocks"),
		dbTimestamp: notInitialized[uint64]("DbTimestamp"),
	}

	sa.cache.blocksLRU.Add(hash, block)

	if b.withInMessages {
		if err := collectBlockEntities[*types.Message](hash, sa, sa.cache.inMessagesLRU, db.MessageTrieTable, block.InMessagesRoot, &res.inMessages); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withOutMessages {
		if err := collectBlockEntities[*types.Message](hash, sa, sa.cache.outMessagesLRU, db.MessageTrieTable, block.OutMessagesRoot, &res.outMessages); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withReceipts {
		if err := collectBlockEntities[*types.Receipt](hash, sa, sa.cache.receiptsLRU, db.ReceiptTrieTable, block.ReceiptsRoot, &res.receipts); err != nil {
			return blockAccessorResult{}, err
		}
	}

	if b.withChildBlocks {
		treeShards := NewShardBlocksTrieReader(
			mpt.NewReaderWithRoot(sa.tx, sa.shardId, db.ShardBlocksTrieTableName(block.Id), block.ChildBlocksRootHash))
		valuePtrs, err := treeShards.Values()
		if err != nil {
			return blockAccessorResult{}, err
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
			return blockAccessorResult{}, err
		}

		res.dbTimestamp = initWith(ts)
	}

	return res, nil
}

func (b blockAccessor) ByNumber(num types.BlockNumber) (blockAccessorResult, error) {
	hash, err := db.ReadBlockHashByNumber(b.shardAccessor.tx, b.shardAccessor.shardId, num)
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
	check.PanicIfNot(data.Message() == nil || data.Message().Hash() == hash)
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
