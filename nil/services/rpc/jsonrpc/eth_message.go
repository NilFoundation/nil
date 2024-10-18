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

func unmarshalMsgAndReceipt(data *rawapitypes.MessageInfo) (*types.Message, *types.Receipt, error) {
	msg := &types.Message{}
	if err := msg.UnmarshalSSZ(data.MessageSSZ); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	receipt := &types.Receipt{}
	if err := receipt.UnmarshalSSZ(data.ReceiptSSZ); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal receipt: %w", err)
	}
	return msg, receipt, nil
}

func makeRequestByHash(hash common.Hash) rawapitypes.MessageRequest {
	return rawapitypes.MessageRequest{
		ByHash: &rawapitypes.MessageRequestByHash{Hash: hash},
	}
}

func makeRequestByBlockRefAndIndex(ref rawapitypes.BlockReference, index types.MessageIndex) rawapitypes.MessageRequest {
	return rawapitypes.MessageRequest{
		ByBlockRefAndIndex: &rawapitypes.MessageRequestByBlockRefAndIndex{
			BlockRef: ref,
			Index:    index,
		},
	}
}

// GetInMessageByHash implements eth_getTransactionByHash. Returns the message structure
func (api *APIImplRo) GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCInMessage, error) {
	res, err := api.rawapi.GetInMessage(ctx, shardId, makeRequestByHash(hash))
	if err != nil {
		return nil, err
	}
	msg, receipt, err := unmarshalMsgAndReceipt(res)
	if err != nil {
		return nil, err
	}
	return NewRPCInMessage(msg, receipt, res.Index, res.BlockHash, res.BlockId)
}

func (api *APIImplRo) GetInMessageByBlockHashAndIndex(
	ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64,
) (*RPCInMessage, error) {
	res, err := api.rawapi.GetInMessage(
		ctx, shardId, makeRequestByBlockRefAndIndex(rawapitypes.BlockHashAsBlockReference(hash), types.MessageIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	msg, receipt, err := unmarshalMsgAndReceipt(res)
	if err != nil {
		return nil, err
	}
	return NewRPCInMessage(msg, receipt, res.Index, res.BlockHash, res.BlockId)
}

func (api *APIImplRo) GetInMessageByBlockNumberAndIndex(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64,
) (*RPCInMessage, error) {
	res, err := api.rawapi.GetInMessage(
		ctx, shardId, makeRequestByBlockRefAndIndex(blockNrToBlockReference(number), types.MessageIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	msg, receipt, err := unmarshalMsgAndReceipt(res)
	if err != nil {
		return nil, err
	}
	return NewRPCInMessage(msg, receipt, res.Index, res.BlockHash, res.BlockId)
}

func (api *APIImplRo) GetRawInMessageByBlockNumberAndIndex(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	res, err := api.rawapi.GetInMessage(
		ctx, shardId, makeRequestByBlockRefAndIndex(blockNrToBlockReference(number), types.MessageIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	return res.MessageSSZ, nil
}

func (api *APIImplRo) GetRawInMessageByBlockHashAndIndex(
	ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	res, err := api.rawapi.GetInMessage(
		ctx, shardId, makeRequestByBlockRefAndIndex(rawapitypes.BlockHashAsBlockReference(hash), types.MessageIndex(index)),
	)
	if err != nil {
		return nil, err
	}
	return res.MessageSSZ, nil
}

func (api *APIImplRo) GetRawInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (hexutil.Bytes, error) {
	res, err := api.rawapi.GetInMessage(ctx, shardId, makeRequestByHash(hash))
	if err != nil {
		return nil, err
	}
	return res.MessageSSZ, nil
}
