package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	ssz "github.com/ferranbt/fastssz"
)

func (api *APIImpl) getBlockHashByNumber(tx db.RoTx, shardId types.ShardId, number transport.BlockNumber) (common.Hash, error) {
	var requestedBlockNumber types.BlockNumber
	switch number {
	case transport.LatestExecutedBlockNumber:
		fallthrough
	case transport.FinalizedBlockNumber:
		fallthrough
	case transport.SafeBlockNumber:
		fallthrough
	case transport.PendingBlockNumber:
		return common.EmptyHash, errNotImplemented
	case transport.LatestBlockNumber:
		lastBlock, err := api.getLastBlock(tx, shardId)
		if err != nil || lastBlock == nil {
			return common.EmptyHash, err
		}
		requestedBlockNumber = lastBlock.Id
	case transport.EarliestBlockNumber:
		requestedBlockNumber = types.BlockNumber(0)
	default:
		requestedBlockNumber = number.BlockNumber()
	}

	blockHash, err := tx.GetFromShard(shardId, db.BlockHashByNumberIndex, requestedBlockNumber.Bytes())
	if errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, nil
	}
	if err != nil {
		return common.EmptyHash, err
	}
	return common.BytesToHash(*blockHash), nil
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *APIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (map[string]any, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	blockHash, err := api.getBlockHashByNumber(tx, shardId, number)
	if err != nil || blockHash == common.EmptyHash {
		return nil, err
	}

	block, messages, receipts, err := api.getBlockWithCollectedEntitiesByHash(tx, shardId, blockHash)
	if err != nil {
		return nil, err
	}

	return toMap(shardId, block, messages, receipts, fullTx)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *APIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (map[string]any, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, messages, receipts, err := api.getBlockWithCollectedEntitiesByHash(tx, shardId, hash)
	if err != nil {
		return nil, err
	}
	return toMap(shardId, block, messages, receipts, fullTx)
}

// GetBlockTransactionCountByNumber implements eth_getBlockTransactionCountByNumber. Returns the number of transactions in a block given the block's block number.
func (api *APIImpl) GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (*hexutil.Uint, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	hash, err := api.getBlockHashByNumber(tx, shardId, number)
	if err != nil {
		return nil, err
	}
	return api.getBlockTransactionCountByHash(tx, shardId, hash)
}

func (api *APIImpl) getBlockTransactionCountByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*hexutil.Uint, error) {
	_, messages, _, err := api.getBlockWithCollectedEntitiesByHash(tx, shardId, hash)
	if err != nil {
		return nil, err
	}

	msgLen := hexutil.Uint(len(messages))
	return &msgLen, nil
}

// GetBlockTransactionCountByHash implements eth_getBlockTransactionCountByHash. Returns the number of transactions in a block given the block's block hash.
func (api *APIImpl) GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*hexutil.Uint, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()
	return api.getBlockTransactionCountByHash(tx, shardId, hash)
}

func (api *APIImpl) getBlockByHash(tx db.Tx, shardId types.ShardId, hash common.Hash) *types.Block {
	block, ok := api.blocksLRU.Get(hash)
	if ok {
		return block
	}
	block = db.ReadBlock(tx, shardId, hash)
	if block != nil {
		// We should not cache (at least without TTL) missing blocks because they may appear in the future.
		api.blocksLRU.Add(hash, block)
	}
	return block
}

func (api *APIImpl) getBlockWithCollectedEntitiesByHash(tx db.Tx, shardId types.ShardId, hash common.Hash) (
	block *types.Block, messages []*types.Message, receipts []*types.Receipt, err error,
) {
	block = api.getBlockByHash(tx, shardId, hash)
	if block == nil {
		return
	}

	messages, messagesCached := api.messagesLRU.Get(hash)
	if !messagesCached {
		messages, err = collectBlockEntities[*types.Message](tx, shardId, db.MessageTrieTable, block.InMessagesRoot)
		if err != nil {
			return
		}
		api.messagesLRU.Add(hash, messages)
	}

	receipts, receiptsCached := api.receiptsLRU.Get(hash)
	if !receiptsCached {
		receipts, err = collectBlockEntities[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot)
		if err != nil {
			return
		}
		api.receiptsLRU.Add(hash, receipts)
	}

	return
}

func (api *APIImpl) getLastBlock(tx db.Tx, shardId types.ShardId) (*types.Block, error) {
	lastBlockHash, err := db.ReadLastBlockHash(tx, shardId)
	if err != nil || lastBlockHash == common.EmptyHash {
		return nil, err
	}
	return db.ReadBlock(tx, shardId, lastBlockHash), nil
}

func collectBlockEntities[
	T interface {
		~*S
		ssz.Unmarshaler
	},
	S any,
](tx db.Tx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash) ([]*S, error) {
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, tableName, rootHash)

	entities := make([]*S, 0, 1024)
	var index uint64
	for {
		k := ssz.MarshalUint64(nil, index)

		entity, err := mpt.GetEntity[T](root, k)
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to get from %v with index %v from trie: %w", tableName, index, err)
		}
		entities = append(entities, entity)
		index += 1
	}
	return entities, nil
}

func toMap(shardId types.ShardId, block *types.Block, messages []*types.Message, receipts []*types.Receipt, fullTx bool) (
	map[string]any, error,
) {
	if block == nil {
		return nil, nil
	}

	var number hexutil.Big
	number.ToInt().SetUint64(block.Id.Uint64())

	messagesRes := make([]any, len(messages))
	if fullTx {
		for i, m := range messages {
			messagesRes[i] = NewRPCInMessage(m, receipts[i], types.MessageIndex(i), block)
		}
	} else {
		for i, m := range messages {
			messagesRes[i] = m.Hash()
		}
	}

	return map[string]any{
		"number":         number,
		"hash":           block.Hash(),
		"inMessagesRoot": block.InMessagesRoot,
		"receiptsRoot":   block.ReceiptsRoot,
		"shardId":        shardId,
		"parentHash":     block.PrevBlock,
		"messages":       messagesRes,
	}, nil
}
