package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *APIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (map[string]any, error) {
	var requestedBlockNumber uint64
	switch number {
	case transport.LatestExecutedBlockNumber:
		fallthrough
	case transport.FinalizedBlockNumber:
		fallthrough
	case transport.SafeBlockNumber:
		fallthrough
	case transport.PendingBlockNumber:
		return nil, errNotImplemented
	case transport.LatestBlockNumber:
		lastBlock, err := api.getLastBlock(ctx, shardId)
		if err != nil {
			return nil, err
		}
		if lastBlock == nil {
			return nil, nil
		}
		requestedBlockNumber = lastBlock.Id
	case transport.EarliestBlockNumber:
		fallthrough
	default:
		requestedBlockNumber = uint64(number)
	}

	// Temporarily look through all blocks from the last one until we find the right one.
	currentBlock, err := api.getLastBlock(ctx, shardId)
	if err != nil {
		return nil, err
	}
	if currentBlock == nil {
		return nil, nil
	}
	for currentBlockHash, ok := currentBlock.Hash(), true; ok; ok = currentBlock.Id != requestedBlockNumber && currentBlockHash != common.EmptyHash {
		currentBlock, err = api.getBlockByHash(ctx, shardId, currentBlockHash)
		if err != nil {
			return nil, err
		}
		currentBlockHash = currentBlock.PrevBlock
	}
	if currentBlock.Id == requestedBlockNumber {
		return toMap(currentBlock), nil
	}
	return nil, nil
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *APIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (map[string]any, error) {
	block, err := api.getBlockByHash(ctx, shardId, hash)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	return toMap(block), nil
}

// GetBlockTransactionCountByNumber implements eth_getBlockTransactionCountByNumber. Returns the number of transactions in a block given the block's block number.
func (api *APIImpl) GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, blockNr transport.BlockNumber) (*hexutil.Uint, error) {
	return nil, errNotImplemented
}

// GetBlockTransactionCountByHash implements eth_getBlockTransactionCountByHash. Returns the number of transactions in a block given the block's block hash.
func (api *APIImpl) GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, blockHash common.Hash) (*hexutil.Uint, error) {
	return nil, errNotImplemented
}

func (api *APIImpl) getLastBlock(ctx context.Context, shardId types.ShardId) (*types.Block, error) {
	lastBlockHash, err := api.db.Get(db.LastBlockTable, shardId.Bytes())
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return api.getBlockByHash(ctx, shardId, common.CastToHash(*lastBlockHash))
}

func (api *APIImpl) getBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Block, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	return db.ReadBlock(tx, shardId, hash), nil
}

func toMap(block *types.Block) map[string]any {
	var number hexutil.Big
	number.ToInt().SetUint64(block.Id)
	return map[string]any{
		"number":     number,
		"hash":       block.Hash(),
		"parentHash": block.PrevBlock,
	}
}
