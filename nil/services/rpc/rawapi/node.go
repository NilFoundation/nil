package rawapi

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type NodeApiOverShardApis struct {
	Apis map[types.ShardId]ShardApi
}

var (
	_ NodeApiRo = (*NodeApiOverShardApis)(nil)
	_ NodeApi   = (*NodeApiOverShardApis)(nil)
)

func NewNodeApiOverShardApis(apis map[types.ShardId]ShardApi) *NodeApiOverShardApis {
	nodeApi := &NodeApiOverShardApis{
		Apis: apis,
	}

	for _, api := range apis {
		api.setNodeApi(nodeApi)
	}

	return nodeApi
}

var ErrShardNotFound = errors.New("shard API not found")

func (api *NodeApiOverShardApis) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.GetBlockHeader(ctx, blockReference)
}

func (api *NodeApiOverShardApis) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.GetFullBlockData(ctx, blockReference)
}

func (api *NodeApiOverShardApis) GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, ErrShardNotFound
	}
	return shardApi.GetBlockTransactionCount(ctx, blockReference)
}

func (api *NodeApiOverShardApis) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	shardApi, ok := api.Apis[address.ShardId()]
	if !ok {
		return types.Value{}, ErrShardNotFound
	}
	return shardApi.GetBalance(ctx, address, blockReference)
}

func (api *NodeApiOverShardApis) GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error) {
	shardApi, ok := api.Apis[address.ShardId()]
	if !ok {
		return types.Code{}, ErrShardNotFound
	}
	return shardApi.GetCode(ctx, address, blockReference)
}

func (api *NodeApiOverShardApis) GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error) {
	shardApi, ok := api.Apis[address.ShardId()]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.GetCurrencies(ctx, address, blockReference)
}

func (api *NodeApiOverShardApis) Call(
	ctx context.Context, args rpctypes.CallArgs, mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
) (*rpctypes.CallResWithGasPrice, error) {
	shardApi, ok := api.Apis[args.To.ShardId()]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.Call(ctx, args, mainBlockReferenceOrHashWithChildren, overrides, emptyMessageIsRoot)
}

func (api *NodeApiOverShardApis) GetInMessage(ctx context.Context, shardId types.ShardId, request rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.GetInMessage(ctx, request)
}

func (api *NodeApiOverShardApis) GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*rawapitypes.ReceiptInfo, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.GetInMessageReceipt(ctx, hash)
}

func (api *NodeApiOverShardApis) GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return types.Value{}, ErrShardNotFound
	}
	return shardApi.GasPrice(ctx)
}

func (api *NodeApiOverShardApis) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	shardApi, ok := api.Apis[types.MainShardId]
	if !ok {
		return nil, ErrShardNotFound
	}
	return shardApi.GetShardIdList(ctx)
}

func (api *NodeApiOverShardApis) GetMessageCount(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error) {
	shardApi, ok := api.Apis[address.ShardId()]
	if !ok {
		return 0, ErrShardNotFound
	}
	return shardApi.GetMessageCount(ctx, address, blockReference)
}

func (api *NodeApiOverShardApis) SendMessage(ctx context.Context, shardId types.ShardId, message []byte) (msgpool.DiscardReason, error) {
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, ErrShardNotFound
	}
	return shardApi.SendMessage(ctx, message)
}
