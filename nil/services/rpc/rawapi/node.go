package rawapi

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

type NodeApiOverShardApis struct {
	Apis map[types.ShardId]ShardApi
}

var _ NodeApi = (*NodeApiOverShardApis)(nil)

func NewNodeApiOverShardApis(apis map[types.ShardId]ShardApi) *NodeApiOverShardApis {
	return &NodeApiOverShardApis{
		Apis: apis,
	}
}

var errShardNotFound = errors.New("shard API not found")

func (api *NodeApiOverShardApis) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, errShardNotFound
	}
	return shardApi.GetBlockHeader(ctx, blockReference)
}

func (api *NodeApiOverShardApis) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, errShardNotFound
	}
	return shardApi.GetFullBlockData(ctx, blockReference)
}

func (api *NodeApiOverShardApis) GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, errShardNotFound
	}
	return shardApi.GetBlockTransactionCount(ctx, blockReference)
}

func (api *NodeApiOverShardApis) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	shardApi, ok := api.Apis[address.ShardId()]
	if !ok {
		return types.Value{}, errShardNotFound
	}
	return shardApi.GetBalance(ctx, address, blockReference)
}
