package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

func unmarshalTxnAndReceipt(data *rawapitypes.TransactionInfo) (*types.Transaction, *types.Receipt, error) {
	txn := &types.Transaction{}
	if err := txn.UnmarshalSSZ(data.TransactionSSZ); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	receipt := &types.Receipt{}
	if err := receipt.UnmarshalSSZ(data.ReceiptSSZ); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}
	return txn, receipt, nil
}

func makeRequestByHash(hash common.Hash) rawapitypes.TransactionRequest {
	return rawapitypes.TransactionRequest{
		ByHash: &rawapitypes.TransactionRequestByHash{Hash: hash},
	}
}

func makeRequestByBlockRefAndIndex(
	ref rawapitypes.BlockReference,
	index types.TransactionIndex,
) rawapitypes.TransactionRequest {
	return rawapitypes.TransactionRequest{
		ByBlockRefAndIndex: &rawapitypes.TransactionRequestByBlockRefAndIndex{
			BlockRef: ref,
			Index:    index,
		},
	}
}

// GetInTransactionByHash implements eth_getTransactionByHash. Returns the transaction structure
func (api *APIImplRo) GetInTransactionByHash(ctx context.Context, hash common.Hash) (*RPCInTransaction, error) {
	shardId := types.ShardIdFromHash(hash)
	res, err := api.rawapi.GetInTransaction(ctx, shardId, makeRequestByHash(hash))
	if err != nil {
		return nil, err
	}
	txn, receipt, err := unmarshalTxnAndReceipt(res)
	if err != nil {
		return nil, err
	}
	return NewRPCInTransaction(txn, receipt, res.Index, res.BlockHash, res.BlockId)
}

func (api *APIImplRo) GetInTransactionByBlockHashAndIndex(
	ctx context.Context, hash common.Hash, index hexutil.Uint64,
) (*RPCInTransaction, error) {
	shardId := types.ShardIdFromHash(hash)
	res, err := api.rawapi.GetInTransaction(
		ctx,
		shardId,
		makeRequestByBlockRefAndIndex(rawapitypes.BlockHashAsBlockReference(hash), types.TransactionIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	txn, receipt, err := unmarshalTxnAndReceipt(res)
	if err != nil {
		return nil, err
	}
	return NewRPCInTransaction(txn, receipt, res.Index, res.BlockHash, res.BlockId)
}

func (api *APIImplRo) GetInTransactionByBlockNumberAndIndex(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64,
) (*RPCInTransaction, error) {
	res, err := api.rawapi.GetInTransaction(
		ctx, shardId, makeRequestByBlockRefAndIndex(blockNrToBlockReference(number), types.TransactionIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	txn, receipt, err := unmarshalTxnAndReceipt(res)
	if err != nil {
		return nil, err
	}
	return NewRPCInTransaction(txn, receipt, res.Index, res.BlockHash, res.BlockId)
}

func (api *APIImplRo) GetRawInTransactionByBlockNumberAndIndex(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	res, err := api.rawapi.GetInTransaction(
		ctx, shardId, makeRequestByBlockRefAndIndex(blockNrToBlockReference(number), types.TransactionIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	return res.TransactionSSZ, nil
}

func (api *APIImplRo) GetRawInTransactionByBlockHashAndIndex(
	ctx context.Context, hash common.Hash, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	shardId := types.ShardIdFromHash(hash)
	res, err := api.rawapi.GetInTransaction(
		ctx,
		shardId,
		makeRequestByBlockRefAndIndex(rawapitypes.BlockHashAsBlockReference(hash), types.TransactionIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	return res.TransactionSSZ, nil
}

func (api *APIImplRo) GetRawInTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	shardId := types.ShardIdFromHash(hash)
	res, err := api.rawapi.GetInTransaction(ctx, shardId, makeRequestByHash(hash))
	if err != nil {
		return nil, err
	}
	return res.TransactionSSZ, nil
}
