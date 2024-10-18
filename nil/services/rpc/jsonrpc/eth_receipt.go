package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func (api *APIImplRo) GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCReceipt, error) {
	info, err := api.rawapi.GetInMessageReceipt(ctx, shardId, hash)
	if err != nil {
		return nil, err
	}
	return NewRPCReceipt(info)
}
