package rawapi

import (
	"context"
	"reflect"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type LocalShardApiAccessor struct {
	RawApi  *LocalShardApi
	codec   apiCodec
	shardId types.ShardId
}

var (
	_ ShardApiRo = (*LocalShardApiAccessor)(nil)
	_ ShardApi   = (*LocalShardApiAccessor)(nil)
)

func NewLocalRawApiAccessor(shardId types.ShardId, rawapi *LocalShardApi) (*LocalShardApiAccessor, error) {
	return newLocalRawApiAccessor(shardId, rawapi, reflect.TypeFor[ShardApi](), reflect.TypeFor[NetworkTransportProtocol]())
}

func newLocalRawApiAccessor(shardId types.ShardId, rawapi *LocalShardApi, apiType, transportType reflect.Type) (*LocalShardApiAccessor, error) {
	check.PanicIfNotf(reflect.ValueOf(rawapi).Type().Implements(apiType), "api does not implement %s", apiType)
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &LocalShardApiAccessor{
		RawApi:  rawapi,
		codec:   codec,
		shardId: shardId,
	}, nil
}

func (api *LocalShardApiAccessor) setNodeApi(nodeApi NodeApi) {
	api.RawApi.setNodeApi(nodeApi)
}

func (api *LocalShardApiAccessor) GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return callLocalWithCallerMethodName[ssz.SSZEncodedData](ctx, api, "GetBlockHeader", blockReference)
}

func (api *LocalShardApiAccessor) GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	return callLocalWithCallerMethodName[*types.RawBlockWithExtractedData](ctx, api, "GetFullBlockData", blockReference)
}

func (api *LocalShardApiAccessor) GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error) {
	return callLocalWithCallerMethodName[uint64](ctx, api, "GetBlockTransactionCount", blockReference)
}

func (api *LocalShardApiAccessor) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	return callLocalWithCallerMethodName[types.Value](ctx, api, "GetBalance", address, blockReference)
}

func (api *LocalShardApiAccessor) GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error) {
	return callLocalWithCallerMethodName[types.Code](ctx, api, "GetCode", address, blockReference)
}

func (api *LocalShardApiAccessor) GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error) {
	return callLocalWithCallerMethodName[map[types.CurrencyId]types.Value](ctx, api, "GetCurrencies", address, blockReference)
}

func (api *LocalShardApiAccessor) GetContract(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (*rawapitypes.SmartContract, error) {
	return callLocalWithCallerMethodName[*rawapitypes.SmartContract](ctx, api, "GetContract", address, blockReference)
}

func (api *LocalShardApiAccessor) Call(
	ctx context.Context, args rpctypes.CallArgs, mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
) (*rpctypes.CallResWithGasPrice, error) {
	return callLocalWithCallerMethodName[*rpctypes.CallResWithGasPrice](ctx, api, "Call", args, mainBlockReferenceOrHashWithChildren, overrides, emptyMessageIsRoot)
}

func (api *LocalShardApiAccessor) GetInMessage(ctx context.Context, request rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error) {
	return callLocalWithCallerMethodName[*rawapitypes.MessageInfo](ctx, api, "GetInMessage", request)
}

func (api *LocalShardApiAccessor) GetInMessageReceipt(ctx context.Context, hash common.Hash) (*rawapitypes.ReceiptInfo, error) {
	return callLocalWithCallerMethodName[*rawapitypes.ReceiptInfo](ctx, api, "GetInMessageReceipt", hash)
}

func (api *LocalShardApiAccessor) GasPrice(ctx context.Context) (types.Value, error) {
	return callLocalWithCallerMethodName[types.Value](ctx, api, "GasPrice")
}

func (api *LocalShardApiAccessor) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return callLocalWithCallerMethodName[[]types.ShardId](ctx, api, "GetShardIdList")
}

func (api *LocalShardApiAccessor) GetMessageCount(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error) {
	return callLocalWithCallerMethodName[uint64](ctx, api, "GetMessageCount", address, blockReference)
}

func (api *LocalShardApiAccessor) SendMessage(ctx context.Context, message []byte) (msgpool.DiscardReason, error) {
	return callLocalWithCallerMethodName[msgpool.DiscardReason](ctx, api, "SendMessage", message)
}

func callLocalWithCallerMethodName[ResponseType any](ctx context.Context, api *LocalShardApiAccessor, methodName string, args ...any) (ResponseType, error) {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	return callLocal[ResponseType](api.codec, api.RawApi, methodName, ctx, args...)
}

// callLocal performs all actions that performs network implementation call but without
// using network transport. It is useful for tests to be sure that network part works correctly.
func callLocal[ResponseType any](apiCodec apiCodec, rawapi *LocalShardApi, methodName string, ctx context.Context, args ...any) (ResponseType, error) {
	codec, ok := apiCodec[methodName]
	check.PanicIfNotf(ok, "Codec for method %s not found", methodName)

	apiValue := reflect.ValueOf(rawapi)
	apiMethod := apiValue.MethodByName(methodName)
	check.PanicIfNot(!apiMethod.IsZero())

	var response ResponseType
	requestBody, err := codec.packRequest(args...)
	if err != nil {
		return response, err
	}

	unpackedArguments, err := codec.unpackRequest(requestBody)
	if err != nil {
		return response, err
	}

	apiArguments := []reflect.Value{reflect.ValueOf(ctx)}
	apiArguments = append(apiArguments, unpackedArguments...)
	apiCallResults := apiMethod.Call(apiArguments)

	responseBody, err := codec.packResponse(apiCallResults...)
	if err != nil {
		return response, err
	}
	return unpackResponse[ResponseType](codec, responseBody)
}
