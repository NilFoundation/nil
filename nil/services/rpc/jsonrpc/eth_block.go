package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

func sszToRPCBlock(shardId types.ShardId, raw *types.RawBlockWithExtractedData, fullTx bool) (*RPCBlock, error) {
	data, err := raw.DecodeSSZ()
	if err != nil {
		return nil, err
	}

	block := &BlockWithEntities{
		Block:          data.Block,
		Receipts:       data.Receipts,
		InTransactions: data.InTransactions,
		ChildBlocks:    data.ChildBlocks,
		DbTimestamp:    data.DbTimestamp,
	}
	return NewRPCBlock(shardId, block, fullTx)
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *APIImplRo) GetBlockByNumber(
	ctx context.Context,
	shardId types.ShardId,
	number transport.BlockNumber,
	fullTx bool,
) (*RPCBlock, error) {
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
func (api *APIImplRo) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (*RPCBlock, error) {
	shardId := types.ShardIdFromHash(hash)
	res, err := api.rawapi.GetFullBlockData(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash))
	if err != nil {
		return nil, err
	}
	return sszToRPCBlock(shardId, res, fullTx)
}

// GetBlockTransactionCountByNumber implements eth_getBlockTransactionCountByNumber.
// Returns the number of transactions in a block given the block's block number.
func (api *APIImplRo) GetBlockTransactionCountByNumber(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber,
) (hexutil.Uint, error) {
	if number < transport.LatestBlockNumber {
		return 0, errNotImplemented
	}
	res, err := api.rawapi.GetBlockTransactionCount(ctx, shardId, blockNrToBlockReference(number))
	return hexutil.Uint(res), err
}

// GetBlockTransactionCountByHash implements eth_getBlockTransactionCountByHash.
// Returns the number of transactions in a block given the block's block hash.
func (api *APIImplRo) GetBlockTransactionCountByHash(
	ctx context.Context, hash common.Hash,
) (hexutil.Uint, error) {
	shardId := types.ShardIdFromHash(hash)
	res, err := api.rawapi.GetBlockTransactionCount(ctx, shardId, rawapitypes.BlockHashAsBlockReference(hash))
	return hexutil.Uint(res), err
}

type BlockWithEntities struct {
	Block          *types.Block
	Receipts       []*types.Receipt
	InTransactions []*types.Transaction
	ChildBlocks    []common.Hash
	DbTimestamp    uint64
}
