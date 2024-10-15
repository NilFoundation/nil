package jsonrpc

import (
	"context"

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

	res, err := api.rawapi.GetFullBlockData(ctx, shardId, blockNrToBlockReference(number))
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

// GetBlockTransactionCountByNumber implements eth_getBlockTransactionCountByNumber. Returns the number of transactions in a block given the block's block number.
func (api *APIImpl) GetBlockTransactionCountByNumber(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber,
) (hexutil.Uint, error) {
	if number < transport.LatestBlockNumber {
		return 0, errNotImplemented
	}
	res, err := api.rawapi.GetBlockTransactionCount(ctx, shardId, blockNrToBlockReference(number))
	return hexutil.Uint(res), err
}

// GetBlockTransactionCountByHash implements eth_getBlockTransactionCountByHash. Returns the number of transactions in a block given the block's block hash.
func (api *APIImpl) GetBlockTransactionCountByHash(
	ctx context.Context, shardId types.ShardId, hash common.Hash,
) (hexutil.Uint, error) {
	res, err := api.rawapi.GetBlockTransactionCount(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash))
	return hexutil.Uint(res), err
}

type BlockWithEntities struct {
	Block       *types.Block
	Receipts    []*types.Receipt
	InMessages  []*types.Message
	ChildBlocks []common.Hash
	DbTimestamp uint64
}
