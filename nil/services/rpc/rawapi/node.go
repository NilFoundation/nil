package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
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

func methodNameChecked(methodName string) string {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(
			callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	return methodName
}

func makeShardNotFoundError(methodName string, shardId types.ShardId) error {
	return makeCallError(methodName, shardId, ErrShardNotFound)
}

func makeCallError(methodName string, shardId types.ShardId, err error) error {
	return fmt.Errorf("failed to call method %s on shard %d: %w", methodName, shardId, err)
}

func (api *NodeApiOverShardApis) GetBlockHeader(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
) (sszx.SSZEncodedData, error) {
	methodName := methodNameChecked("GetBlockHeader")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetBlockHeader(ctx, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetFullBlockData(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
) (*types.RawBlockWithExtractedData, error) {
	methodName := methodNameChecked("GetFullBlockData")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetFullBlockData(ctx, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetBlockTransactionCount(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
) (uint64, error) {
	methodName := methodNameChecked("GetBlockTransactionCount")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetBlockTransactionCount(ctx, blockReference)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetBalance(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Value, error) {
	methodName := methodNameChecked("GetBalance")
	shardId := address.ShardId()
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return types.Value{}, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetBalance(ctx, address, blockReference)
	if err != nil {
		return types.Value{}, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetCode(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Code, error) {
	methodName := methodNameChecked("GetCode")
	shardId := address.ShardId()
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return types.Code{}, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetCode(ctx, address, blockReference)
	if err != nil {
		return types.Code{}, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetTokens(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	methodName := methodNameChecked("GetTokens")
	shardId := address.ShardId()
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTokens(ctx, address, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetContract(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (*rawapitypes.SmartContract, error) {
	methodName := methodNameChecked("GetContract")
	shardId := address.ShardId()
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetContract(ctx, address, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) Call(
	ctx context.Context,
	args rpctypes.CallArgs,
	mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
	overrides *rpctypes.StateOverrides,
) (*rpctypes.CallResWithGasPrice, error) {
	methodName := methodNameChecked("Call")

	txn, err := args.ToTransaction()
	if err != nil {
		return nil, err
	}

	shardId := txn.To.ShardId()
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.Call(ctx, args, mainBlockReferenceOrHashWithChildren, overrides)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetInTransaction(
	ctx context.Context,
	shardId types.ShardId,
	request rawapitypes.TransactionRequest,
) (*rawapitypes.TransactionInfo, error) {
	methodName := methodNameChecked("GetInTransaction")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetInTransaction(ctx, request)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetInTransactionReceipt(
	ctx context.Context,
	shardId types.ShardId,
	hash common.Hash,
) (*rawapitypes.ReceiptInfo, error) {
	methodName := methodNameChecked("GetInTransactionReceipt")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetInTransactionReceipt(ctx, hash)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error) {
	methodName := methodNameChecked("GasPrice")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return types.Value{}, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GasPrice(ctx)
	if err != nil {
		return types.Value{}, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	methodName := methodNameChecked("GetShardIdList")
	shardId := types.MainShardId
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetShardIdList(ctx)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetNumShards(ctx context.Context) (uint64, error) {
	methodName := methodNameChecked("GetNumShards")
	shardId := types.MainShardId
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetNumShards(ctx)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetTransactionCount(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (uint64, error) {
	methodName := methodNameChecked("GetTransactionCount")
	shardId := address.ShardId()
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTransactionCount(ctx, address, blockReference)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) SendTransaction(
	ctx context.Context,
	shardId types.ShardId,
	transaction []byte,
) (txnpool.DiscardReason, error) {
	methodName := methodNameChecked("SendTransaction")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.SendTransaction(ctx, transaction)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) ClientVersion(ctx context.Context) (string, error) {
	methodName := methodNameChecked("ClientVersion")
	shardId := types.MainShardId
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return "", makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.ClientVersion(ctx)
	if err != nil {
		return "", makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error) {
	methodName := methodNameChecked("DoPanicOnShard")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	return shardApi.DoPanicOnShard(ctx)
}

func (api *NodeApiOverShardApis) GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (uint64, error) {
	methodName := methodNameChecked("GetTxpoolStatus")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTxpoolStatus(ctx)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *NodeApiOverShardApis) GetTxpoolContent(
	ctx context.Context,
	shardId types.ShardId,
) ([]*types.Transaction, error) {
	methodName := methodNameChecked("GetTxpoolContent")
	shardApi, ok := api.Apis[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTxpoolContent(ctx)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}
