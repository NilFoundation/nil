package internal

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

type nodeApiOverShardApis struct {
	apisRo  map[types.ShardId]shardApiRo
	apisRw  map[types.ShardId]shardApiRw
	apisDev map[types.ShardId]shardApiDev

	allApis []shardApiBase
}

var _ NodeApi = (*nodeApiOverShardApis)(nil)

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
	return makeCallError(methodName, shardId, rawapitypes.ErrShardNotFound)
}

func makeCallError(methodName string, shardId types.ShardId, err error) error {
	return fmt.Errorf("failed to call method %s on shard %d: %w", methodName, shardId, err)
}

func (api *nodeApiOverShardApis) GetBlockHeader(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
) (sszx.SSZEncodedData, error) {
	methodName := methodNameChecked("GetBlockHeader")
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetBlockHeader(ctx, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetFullBlockData(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
) (*types.RawBlockWithExtractedData, error) {
	methodName := methodNameChecked("GetFullBlockData")
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetFullBlockData(ctx, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetBlockTransactionCount(
	ctx context.Context,
	shardId types.ShardId,
	blockReference rawapitypes.BlockReference,
) (uint64, error) {
	methodName := methodNameChecked("GetBlockTransactionCount")
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetBlockTransactionCount(ctx, blockReference)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetBalance(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Value, error) {
	methodName := methodNameChecked("GetBalance")
	shardId := address.ShardId()
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return types.Value{}, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetBalance(ctx, address, blockReference)
	if err != nil {
		return types.Value{}, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetCode(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (types.Code, error) {
	methodName := methodNameChecked("GetCode")
	shardId := address.ShardId()
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return types.Code{}, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetCode(ctx, address, blockReference)
	if err != nil {
		return types.Code{}, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetTokens(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	methodName := methodNameChecked("GetTokens")
	shardId := address.ShardId()
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTokens(ctx, address, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetContract(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (*rawapitypes.SmartContract, error) {
	methodName := methodNameChecked("GetContract")
	shardId := address.ShardId()
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetContract(ctx, address, blockReference)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) Call(
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
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.Call(ctx, args, mainBlockReferenceOrHashWithChildren, overrides)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetInTransaction(
	ctx context.Context,
	shardId types.ShardId,
	request rawapitypes.TransactionRequest,
) (*rawapitypes.TransactionInfo, error) {
	methodName := methodNameChecked("GetInTransaction")
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetInTransaction(ctx, request)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetInTransactionReceipt(
	ctx context.Context,
	shardId types.ShardId,
	hash common.Hash,
) (*rawapitypes.ReceiptInfo, error) {
	methodName := methodNameChecked("GetInTransactionReceipt")
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetInTransactionReceipt(ctx, hash)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error) {
	methodName := methodNameChecked("GasPrice")
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return types.Value{}, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GasPrice(ctx)
	if err != nil {
		return types.Value{}, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	methodName := methodNameChecked("GetShardIdList")
	shardId := types.MainShardId
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetShardIdList(ctx)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetNumShards(ctx context.Context) (uint64, error) {
	methodName := methodNameChecked("GetNumShards")
	shardId := types.MainShardId
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetNumShards(ctx)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetTransactionCount(
	ctx context.Context,
	address types.Address,
	blockReference rawapitypes.BlockReference,
) (uint64, error) {
	methodName := methodNameChecked("GetTransactionCount")
	shardId := address.ShardId()
	shardApi, ok := api.apisRw[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTransactionCount(ctx, address, blockReference)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) SendTransaction(
	ctx context.Context,
	shardId types.ShardId,
	transaction []byte,
) (txnpool.DiscardReason, error) {
	methodName := methodNameChecked("SendTransaction")
	shardApi, ok := api.apisRw[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.SendTransaction(ctx, transaction)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) ClientVersion(ctx context.Context) (string, error) {
	methodName := methodNameChecked("ClientVersion")
	shardId := types.MainShardId
	shardApi, ok := api.apisRo[shardId]
	if !ok {
		return "", makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.ClientVersion(ctx)
	if err != nil {
		return "", makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error) {
	methodName := methodNameChecked("DoPanicOnShard")
	shardApi, ok := api.apisDev[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	return shardApi.DoPanicOnShard(ctx)
}

func (api *nodeApiOverShardApis) GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (uint64, error) {
	methodName := methodNameChecked("GetTxpoolStatus")
	shardApi, ok := api.apisRw[shardId]
	if !ok {
		return 0, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTxpoolStatus(ctx)
	if err != nil {
		return 0, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) GetTxpoolContent(
	ctx context.Context,
	shardId types.ShardId,
) ([]*types.Transaction, error) {
	methodName := methodNameChecked("GetTxpoolContent")
	shardApi, ok := api.apisRw[shardId]
	if !ok {
		return nil, makeShardNotFoundError(methodName, shardId)
	}
	result, err := shardApi.GetTxpoolContent(ctx)
	if err != nil {
		return nil, makeCallError(methodName, shardId, err)
	}
	return result, nil
}

func (api *nodeApiOverShardApis) SetP2pRequestHandlers(
	ctx context.Context,
	networkManager network.Manager,
	logger logging.Logger,
) error {
	if networkManager == nil {
		return nil
	}
	for _, api := range api.allApis {
		if err := api.setAsP2pRequestHandlersIfAllowed(ctx, networkManager, logger); err != nil {
			logger.Error().
				Err(err).
				Stringer(logging.FieldShardId, api.shardId()).
				Msg("Failed to set raw API request handler")
			return err
		}
	}
	return nil
}
