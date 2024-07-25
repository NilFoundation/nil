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
	data, err := api.getBlockWithEntities(ctx, shardId, numOrHash)
	if err != nil || data.Block == nil {
		return nil, err
	}

	return NewRPCBlock(shardId, data, fullTx)
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
	data, err := api.getBlockWithEntities(ctx, shardId, numOrHash)
	if err != nil {
		return 0, err
	}

	return hexutil.Uint(len(data.InMessages)), nil
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

type BlockWithEntities struct {
	Block       *types.Block
	Receipts    []*types.Receipt
	InMessages  []*types.Message
	ChildBlocks []common.Hash
	DbTimestamp uint64
}

func (api *APIImpl) getBlockWithEntities(
	ctx context.Context, shardId types.ShardId, numOrHash transport.BlockNumberOrHash) (
	*BlockWithEntities, error,
) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	hash, err := api.extractBlockHash(tx, shardId, numOrHash)
	if err != nil {
		return nil, err
	}

	data, err := api.accessor.Access(tx, shardId).
		GetBlock().
		WithReceipts().
		WithInMessages().
		WithChildBlocks().
		WithDbTimestamp().
		ByHash(hash)
	if err != nil {
		return nil, err
	}

	return &BlockWithEntities{
		Block:       data.Block(),
		Receipts:    data.Receipts(),
		InMessages:  data.InMessages(),
		ChildBlocks: data.ChildBlocks(),
		DbTimestamp: data.DbTimestamp(),
	}, nil
}
