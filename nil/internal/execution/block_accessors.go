package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

type BlockAccessor struct {
	headers      *BlockHeaderByHashAccessor
	hashByNumber *BlockHashByNumberAccessor
}

func NewBlockAccessor(blocksLRUSize int) *BlockAccessor {
	return &BlockAccessor{
		headers:      NewBlockHeaderByHashAccessor(blocksLRUSize),
		hashByNumber: NewBlockHashByNumberAccessor(),
	}
}

func (s BlockAccessor) GetByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, error) {
	return s.headers.Get(tx, shardId, hash)
}

func (s BlockAccessor) GetByNumber(tx db.RoTx, shardId types.ShardId, num types.BlockNumber) (*types.Block, error) {
	hash, err := s.hashByNumber.Get(tx, shardId, num)
	if err != nil {
		return nil, err
	}

	return s.headers.Get(tx, shardId, hash)
}

type headerWithRaw struct {
	block *types.Block
	raw   []byte
}

type BlockHeaderByHashAccessor struct {
	cache *lru.Cache[common.Hash, headerWithRaw]
}

func NewBlockHeaderByHashAccessor(blocksLRUSize int) *BlockHeaderByHashAccessor {
	cache, err := lru.New[common.Hash, headerWithRaw](blocksLRUSize)
	check.PanicIfErr(err)

	return &BlockHeaderByHashAccessor{
		cache: cache,
	}
}

type BlockByHashAccessor struct {
	cache *lru.Cache[common.Hash, *types.RawBlockWithExtractedData]

	headersCache *BlockHeaderByHashAccessor
}

func NewBlockByHashAccessor(blocksLRUSize int) *BlockByHashAccessor {
	fc, err := lru.New[common.Hash, *types.RawBlockWithExtractedData](blocksLRUSize)
	check.PanicIfErr(err)

	return &BlockByHashAccessor{
		cache:        fc,
		headersCache: NewBlockHeaderByHashAccessor(blocksLRUSize),
	}
}

type BlockHashByNumberAccessor struct {
	// Currently, the block can be updated, so we cannot cache
}

func NewBlockHashByNumberAccessor() *BlockHashByNumberAccessor {
	return &BlockHashByNumberAccessor{}
}

func (b *BlockHashByNumberAccessor) Get(tx db.RoTx, shardId types.ShardId, num types.BlockNumber) (common.Hash, error) {
	return db.ReadBlockHashByNumber(tx, shardId, num)
}

func (b *BlockHeaderByHashAccessor) Get(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, error) {
	h, err := b.get(tx, shardId, hash)
	if err != nil {
		return nil, err
	}
	return h.block, nil
}

func (b *BlockHeaderByHashAccessor) GetRaw(tx db.RoTx, shardId types.ShardId, hash common.Hash) ([]byte, error) {
	h, err := b.get(tx, shardId, hash)
	if err != nil {
		return nil, err
	}
	return h.raw, nil
}

func (b *BlockByHashAccessor) Get(
	tx db.RoTx, shardId types.ShardId, hash common.Hash,
) (*types.RawBlockWithExtractedData, error) {
	if rawBlockExt, ok := b.cache.Get(hash); ok {
		return rawBlockExt, nil
	}

	h, err := b.headersCache.get(tx, shardId, hash)
	if err != nil {
		return nil, err
	}

	res := &types.RawBlockWithExtractedData{
		Block: h.raw,
	}

	res.InTransactions, err = b.collectTxnIndexedEntities(tx, shardId,
		db.TransactionTrieTable, h.block.InTransactionsRoot)
	if err != nil {
		return nil, err
	}
	res.InTxCounts, err = b.collectTxnCounts(tx, shardId, db.TransactionTrieTable, h.block.InTransactionsRoot)
	if err != nil {
		return nil, err
	}

	res.OutTransactions, err = b.collectTxnIndexedEntities(tx, shardId,
		db.TransactionTrieTable, h.block.OutTransactionsRoot)
	if err != nil {
		return nil, err
	}
	res.OutTxCounts, err = b.collectTxnCounts(tx, shardId, db.TransactionTrieTable, h.block.OutTransactionsRoot)
	if err != nil {
		return nil, err
	}

	res.Receipts, err = b.collectTxnIndexedEntities(tx, shardId, db.ReceiptTrieTable, h.block.ReceiptsRoot)
	if err != nil {
		return nil, err
	}

	res.ChildBlocks, err = b.collectChildBlocks(tx, shardId, h.block)
	if err != nil {
		return nil, err
	}

	res.DbTimestamp, err = db.ReadBlockTimestamp(tx, shardId, hash)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	if shardId.IsMainShard() {
		res.Config, err = b.collectConfig(tx, shardId, h.block)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (b *BlockHeaderByHashAccessor) get(tx db.RoTx, shardId types.ShardId, hash common.Hash) (headerWithRaw, error) {
	if h, ok := b.cache.Get(hash); ok {
		return h, nil
	}

	raw, err := db.ReadBlockBytes(tx, shardId, hash)
	if err != nil {
		return headerWithRaw{}, err
	}

	block := &types.Block{}
	if err := block.UnmarshalNil(raw); err != nil {
		return headerWithRaw{}, err
	}

	if assert.Enable {
		blockHash := block.Hash(shardId)
		check.PanicIfNotf(blockHash == hash, "block hash mismatch: %s != %s", blockHash, hash)
	}

	res := headerWithRaw{
		block: block,
		raw:   raw,
	}
	b.cache.Add(hash, res)

	return res, nil
}

func (b *BlockByHashAccessor) collectTxnCounts(
	tx db.RoTx, shardId types.ShardId,
	tableName db.ShardedTableName, root common.Hash,
) ([][]byte, error) {
	reader := mpt.NewDbReader(tx, shardId, tableName)
	if err := reader.SetRootHash(root); err != nil {
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

func (b *BlockByHashAccessor) collectTxnIndexedEntities(
	tx db.RoTx, shardId types.ShardId,
	tableName db.ShardedTableName, root common.Hash,
) ([][]byte, error) {
	reader := mpt.NewDbReader(tx, shardId, tableName)
	if err := reader.SetRootHash(root); err != nil {
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

func (b *BlockByHashAccessor) collectChildBlocks(
	tx db.RoTx, shardId types.ShardId, block *types.Block,
) ([]common.Hash, error) {
	treeShards := NewDbShardBlocksTrieReader(tx, shardId, block.Id)
	if err := treeShards.SetRootHash(block.ChildBlocksRootHash); err != nil {
		return nil, err
	}

	shards := make(map[types.ShardId]common.Hash)
	for key, value := range treeShards.Iterate() {
		shards[types.BytesToShardId(key)] = common.BytesToHash(value)
	}

	values := make([]common.Hash, len(shards))
	for key, value := range shards {
		values[key-1] = value // the main shard is omitted
	}
	return values, nil
}

func (b *BlockByHashAccessor) collectConfig(
	tx db.RoTx, shardId types.ShardId, block *types.Block,
) (map[string][]byte, error) {
	res := mpt.NewDbReader(tx, shardId, db.ConfigTrieTable)
	reader, err := res, res.SetRootHash(block.ConfigRoot)
	if err != nil {
		return nil, err
	}
	configMap := make(map[string][]byte)
	for key, value := range reader.Iterate() {
		configMap[string(key)] = value
	}
	return configMap, nil
}
