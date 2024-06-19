package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

func (api *APIImpl) getBlockHashByNumber(
	tx db.RoTx, shardId types.ShardId, number transport.BlockNumber,
) (common.Hash, error) {
	var requestedBlockNumber types.BlockNumber
	switch number {
	case transport.LatestExecutedBlockNumber:
		return common.EmptyHash, errNotImplemented
	case transport.FinalizedBlockNumber:
		return common.EmptyHash, errNotImplemented
	case transport.SafeBlockNumber:
		return common.EmptyHash, errNotImplemented
	case transport.PendingBlockNumber:
		return common.EmptyHash, errNotImplemented
	case transport.LatestBlockNumber:
		lastBlock, err := db.ReadLastBlock(tx, shardId)
		if err != nil {
			return common.EmptyHash, err
		}
		requestedBlockNumber = lastBlock.Id
	case transport.EarliestBlockNumber:
		requestedBlockNumber = types.BlockNumber(0)
	default:
		requestedBlockNumber = number.BlockNumber()
	}

	return db.ReadBlockHashByNumber(tx, shardId, requestedBlockNumber)
}

func (api *APIImpl) extractBlockHash(tx db.RoTx, shardId types.ShardId, numOrHash transport.BlockNumberOrHash) (common.Hash, error) {
	if numOrHash.BlockNumber != nil {
		return api.getBlockHashByNumber(tx, shardId, *numOrHash.BlockNumber)
	}
	return *numOrHash.BlockHash, nil
}

func (api *APIImpl) fetchBlockByNumberOrHash(tx db.RoTx, shardId types.ShardId, numOrHash transport.BlockNumberOrHash) (*types.Block, error) {
	hash, err := api.extractBlockHash(tx, shardId, numOrHash)
	if err != nil {
		return nil, err
	}
	if data, err := api.accessor.Access(tx, shardId).GetBlock().ByHash(hash); err != nil {
		return nil, err
	} else {
		return data.Block(), nil
	}
}

func (api *APIImpl) getBlockByNumberOrHash(
	ctx context.Context, shardId types.ShardId, numOrHash transport.BlockNumberOrHash, fullTx bool,
) (*RPCBlock, error) {
	block, messages, receipts, err := api.getBlockWithCollectedEntitiesByNumberOrHash(ctx, shardId, numOrHash)
	if err != nil || block == nil {
		return nil, err
	}

	return NewRPCBlock(shardId, block, messages, receipts, fullTx)
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *APIImpl) GetBlockByNumber(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool,
) (*RPCBlock, error) {
	return api.getBlockByNumberOrHash(ctx, shardId, transport.BlockNumberOrHash{BlockNumber: &number}, fullTx)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *APIImpl) GetBlockByHash(
	ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool,
) (*RPCBlock, error) {
	return api.getBlockByNumberOrHash(ctx, shardId, transport.BlockNumberOrHash{BlockHash: &hash}, fullTx)
}

func (api *APIImpl) getBlockTransactionCountByNumberOrHash(
	ctx context.Context, shardId types.ShardId, numOrHash transport.BlockNumberOrHash,
) (hexutil.Uint, error) {
	_, messages, _, err := api.getBlockWithCollectedEntitiesByNumberOrHash(ctx, shardId, numOrHash)
	if err != nil {
		return 0, err
	}

	return hexutil.Uint(len(messages)), nil
}

// GetBlockTransactionCountByNumber implements eth_getBlockTransactionCountByNumber. Returns the number of transactions in a block given the block's block number.
func (api *APIImpl) GetBlockTransactionCountByNumber(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber,
) (hexutil.Uint, error) {
	return api.getBlockTransactionCountByNumberOrHash(ctx, shardId, transport.BlockNumberOrHash{BlockNumber: &number})
}

// GetBlockTransactionCountByHash implements eth_getBlockTransactionCountByHash. Returns the number of transactions in a block given the block's block hash.
func (api *APIImpl) GetBlockTransactionCountByHash(
	ctx context.Context, shardId types.ShardId, hash common.Hash,
) (hexutil.Uint, error) {
	return api.getBlockTransactionCountByNumberOrHash(ctx, shardId, transport.BlockNumberOrHash{BlockHash: &hash})
}

func (api *APIImpl) getBlockWithCollectedEntitiesByNumberOrHash(
	ctx context.Context, shardId types.ShardId, numOrHash transport.BlockNumberOrHash) (
	*types.Block, []*types.Message, []*types.Receipt, error,
) {
	if err := api.checkShard(shardId); err != nil {
		return nil, nil, nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	hash, err := api.extractBlockHash(tx, shardId, numOrHash)
	if err != nil {
		return nil, nil, nil, err
	}

	data, err := api.accessor.Access(tx, shardId).GetBlock().WithReceipts().WithInMessages().ByHash(hash)
	if err != nil {
		return nil, nil, nil, err
	}
	return data.Block(), data.InMessages(), data.Receipts(), err
}
