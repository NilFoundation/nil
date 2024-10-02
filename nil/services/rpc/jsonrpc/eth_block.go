package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

func getBlockHashByNumber(
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
		lastBlockHash, err := db.ReadLastBlockHash(tx, shardId)
		if err != nil {
			return common.EmptyHash, err
		}
		return lastBlockHash, nil
	case transport.EarliestBlockNumber:
		requestedBlockNumber = types.BlockNumber(0)
	default:
		requestedBlockNumber = number.BlockNumber()
	}

	return db.ReadBlockHashByNumber(tx, shardId, requestedBlockNumber)
}

func extractBlockHash(tx db.RoTx, shardId types.ShardId, numOrHash transport.BlockNumberOrHash) (common.Hash, error) {
	if numOrHash.BlockNumber != nil {
		return getBlockHashByNumber(tx, shardId, *numOrHash.BlockNumber)
	}
	return *numOrHash.BlockHash, nil
}

func (api *APIImpl) fetchBlockByNumberOrHash(tx db.RoTx, shardId types.ShardId, numOrHash transport.BlockNumberOrHash) (*types.Block, error) {
	hash, err := extractBlockHash(tx, shardId, numOrHash)
	if err != nil {
		return nil, err
	}
	if data, err := api.accessor.Access(tx, shardId).GetBlock().ByHash(hash); err != nil {
		return nil, err
	} else {
		return data.Block(), nil
	}
}

func sszToRPCBlock(shardId types.ShardId, raw *types.RawBlockWithExtractedData, fullTx bool) (*RPCBlock, error) {
	data, err := raw.DecodeSSZ()
	if err != nil {
		return nil, err
	}

	block := &BlockWithEntities{
		Block:       data.Block,
		Receipts:    data.Receipts,
		InMessages:  data.InMessages,
		ChildBlocks: data.ChildBlocks,
		DbTimestamp: data.DbTimestamp,
	}
	return NewRPCBlock(shardId, block, fullTx)
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *APIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (*RPCBlock, error) {
	if number < transport.LatestBlockNumber {
		return nil, errNotImplemented
	}

	var ref rawapitypes.BlockReference
	if number <= 0 {
		ref = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.NamedBlockIdentifier(number))
	} else {
		ref = rawapitypes.BlockNumberAsBlockReference(types.BlockNumber(number))
	}

	res, err := api.rawapi.GetFullBlockData(ctx, shardId, ref)
	if err != nil {
		return nil, err
	}
	return sszToRPCBlock(shardId, res, fullTx)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *APIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (*RPCBlock, error) {
	res, err := api.rawapi.GetFullBlockData(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash))
	if err != nil {
		return nil, err
	}
	return sszToRPCBlock(shardId, res, fullTx)
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

	hash, err := extractBlockHash(tx, shardId, numOrHash)
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
