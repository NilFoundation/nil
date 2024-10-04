package rawapi

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

type NodeRawApi struct {
	Apis map[types.ShardId]ShardApi
}

var _ Api = (*NodeRawApi)(nil)

func NewNodeRawApi(apis map[types.ShardId]ShardApi) *NodeRawApi {
	return &NodeRawApi{
		Apis: apis,
	}
}

var errShardNotFound = errors.New("shard API not found")

func (api *NodeRawApi) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, errShardNotFound
	}
	return shardApi.GetBlockHeader(ctx, blockReference)
}

func (api *NodeRawApi) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, errShardNotFound
	}
	return shardApi.GetFullBlockData(ctx, blockReference)
}

func (api *NodeRawApi) GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, errShardNotFound
	}
	return shardApi.GetBlockTransactionCount(ctx, blockReference)
}

func (api *NodeRawApi) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	shardApi, ok := api.Apis[address.ShardId()]
	if !ok {
		return types.Value{}, errShardNotFound
	}
	return shardApi.GetBalance(ctx, address, blockReference)
}
