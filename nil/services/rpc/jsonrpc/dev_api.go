package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
)

type DevAPI interface {
	DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error)
}

type DevAPIImpl struct {
	rawApi rawapi.NodeApi
}

func NewDevAPI(rawApi rawapi.NodeApi) DevAPI {
	return &DevAPIImpl{
		rawApi: rawApi,
	}
}

func (d *DevAPIImpl) DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error) {
	return d.rawApi.DoPanicOnShard(ctx, shardId)
}
