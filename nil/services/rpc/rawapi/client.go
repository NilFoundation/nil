package rawapi

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

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

type (
	doApiRequestFunction func(
		codec *methodCodec, methodName string, ctx context.Context, args ...any) ([]byte, error)
	nodeApiSetter                            func(NodeApi)
	setAsP2pRequestHandlersIfAllowedFunction func(
		ctx context.Context, networkManager *network.Manager, readonly bool, logger logging.Logger) error
)

type ShardApiAccessor struct {
	codec                              apiCodec
	doApiRequest                       doApiRequestFunction
	onSetNodeApi                       nodeApiSetter
	onSetAsP2pRequestHandlersIfAllowed setAsP2pRequestHandlersIfAllowedFunction
}

var _ ShardApi = (*ShardApiAccessor)(nil)

func NewNetworkRawApiAccessor(shardId types.ShardId, networkManager *network.Manager) (*ShardApiAccessor, error) {
	return newNetworkRawApiAccessor(
		shardId, networkManager, reflect.TypeFor[ShardApi](), reflect.TypeFor[NetworkTransportProtocol]())
}

func NewLocalRawApiAccessor(shardId types.ShardId, rawapi *LocalShardApi) (*ShardApiAccessor, error) {
	return newDirectRawApiAccessor(
		shardId, rawapi, reflect.TypeFor[ShardApi](), reflect.TypeFor[NetworkTransportProtocol]())
}

func newNetworkRawApiAccessor(
	shardId types.ShardId,
	networkManager *network.Manager,
	apiType reflect.Type,
	transportType reflect.Type,
) (*ShardApiAccessor, error) {
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &ShardApiAccessor{
		codec:        codec,
		doApiRequest: makeDoNetworkRawApiRequestFunction(networkManager, shardId, "rawapi"),
		onSetNodeApi: func(NodeApi) {},
		onSetAsP2pRequestHandlersIfAllowed: func(
			ctx context.Context, networkManager *network.Manager, readonly bool, logger logging.Logger,
		) error {
			return nil
		},
	}, nil
}

func newDirectRawApiAccessor(
	shardId types.ShardId,
	rawapi ShardApi,
	apiType reflect.Type,
	transportType reflect.Type,
) (*ShardApiAccessor, error) {
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &ShardApiAccessor{
		codec:        codec,
		doApiRequest: makeDoLocalRawApiRequestFunction(rawapi),
		onSetNodeApi: func(nodeApi NodeApi) {
			rawapi.setNodeApi(nodeApi)
		},
		onSetAsP2pRequestHandlersIfAllowed: func(
			ctx context.Context, networkManager *network.Manager, readonly bool, logger logging.Logger,
		) error {
			return SetRawApiRequestHandlers(ctx, shardId, rawapi, networkManager, readonly, logger)
		},
	}, nil
}

func makeDoNetworkRawApiRequestFunction(
	networkManager *network.Manager,
	shardId types.ShardId,
	apiName string,
) doApiRequestFunction {
	return func(codec *methodCodec, methodName string, ctx context.Context, args ...any) ([]byte, error) {
		protocol := network.ProtocolID(fmt.Sprintf("/shard/%d/%s/%s", shardId, apiName, methodName))
		serverPeerId, err := discoverAppropriatePeer(networkManager, shardId, protocol)
		if err != nil {
			return nil, err
		}

		requestBody, err := codec.packRequest(args...)
		if err != nil {
			return nil, err
		}

		return networkManager.SendRequestAndGetResponse(ctx, serverPeerId, protocol, requestBody)
	}
}

func makeDoLocalRawApiRequestFunction(rawapi ShardApi) doApiRequestFunction {
	return func(codec *methodCodec, methodName string, ctx context.Context, args ...any) ([]byte, error) {
		apiValue := reflect.ValueOf(rawapi)
		apiMethod := apiValue.MethodByName(methodName)
		check.PanicIfNot(!apiMethod.IsZero())

		requestBody, err := codec.packRequest(args...)
		if err != nil {
			return nil, err
		}

		unpackedArguments, err := codec.unpackRequest(requestBody)
		if err != nil {
			return nil, err
		}

		apiArguments := []reflect.Value{reflect.ValueOf(ctx)}
		apiArguments = append(apiArguments, unpackedArguments...)
		apiCallResults := apiMethod.Call(apiArguments)

		return codec.packResponse(apiCallResults...)
	}
}

func (api *ShardApiAccessor) GetBlockHeader(
	ctx context.Context, blockReference rawapitypes.BlockReference,
) (sszx.SSZEncodedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[sszx.SSZEncodedData](
		ctx, api, "GetBlockHeader", blockReference)
}

func (api *ShardApiAccessor) GetFullBlockData(
	ctx context.Context, blockReference rawapitypes.BlockReference,
) (*types.RawBlockWithExtractedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*types.RawBlockWithExtractedData](
		ctx, api, "GetFullBlockData", blockReference)
}

func (api *ShardApiAccessor) GetBlockTransactionCount(
	ctx context.Context, blockReference rawapitypes.BlockReference,
) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "GetBlockTransactionCount", blockReference)
}

func (api *ShardApiAccessor) GetBalance(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Value](ctx, api, "GetBalance", address, blockReference)
}

func (api *ShardApiAccessor) GetCode(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (types.Code, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Code](ctx, api, "GetCode", address, blockReference)
}

func (api *ShardApiAccessor) GetTokens(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[map[types.TokenId]types.Value](
		ctx, api, "GetTokens", address, blockReference)
}

func (api *ShardApiAccessor) GetContract(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (*rawapitypes.SmartContract, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.SmartContract](
		ctx, api, "GetContract", address, blockReference)
}

func (api *ShardApiAccessor) Call(
	ctx context.Context,
	args rpctypes.CallArgs,
	mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
	overrides *rpctypes.StateOverrides,
) (*rpctypes.CallResWithGasPrice, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rpctypes.CallResWithGasPrice](
		ctx, api, "Call", args, mainBlockReferenceOrHashWithChildren, overrides)
}

func (api *ShardApiAccessor) GetInTransaction(
	ctx context.Context, request rawapitypes.TransactionRequest,
) (*rawapitypes.TransactionInfo, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.TransactionInfo](
		ctx, api, "GetInTransaction", request)
}

func (api *ShardApiAccessor) GetInTransactionReceipt(
	ctx context.Context, hash common.Hash,
) (*rawapitypes.ReceiptInfo, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.ReceiptInfo](
		ctx, api, "GetInTransactionReceipt", hash)
}

func (api *ShardApiAccessor) GasPrice(ctx context.Context) (types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Value](ctx, api, "GasPrice")
}

func (api *ShardApiAccessor) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return sendRequestAndGetResponseWithCallerMethodName[[]types.ShardId](ctx, api, "GetShardIdList")
}

func (api *ShardApiAccessor) GetNumShards(ctx context.Context) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "GetNumShards")
}

func (api *ShardApiAccessor) GetTransactionCount(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](
		ctx, api, "GetTransactionCount", address, blockReference)
}

func (api *ShardApiAccessor) SendTransaction(ctx context.Context, transaction []byte) (txnpool.DiscardReason, error) {
	return sendRequestAndGetResponseWithCallerMethodName[txnpool.DiscardReason](
		ctx, api, "SendTransaction", transaction)
}

func (api *ShardApiAccessor) ClientVersion(ctx context.Context) (string, error) {
	return sendRequestAndGetResponseWithCallerMethodName[string](ctx, api, "ClientVersion")
}

func (api *ShardApiAccessor) DoPanicOnShard(ctx context.Context) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "DoPanicOnShard")
}

func (api *ShardApiAccessor) setNodeApi(nodeApi NodeApi) {
	api.onSetNodeApi(nodeApi)
}

func (api *ShardApiAccessor) setAsP2pRequestHandlersIfAllowed(
	ctx context.Context,
	networkManager *network.Manager,
	readonly bool,
	logger logging.Logger,
) error {
	return api.onSetAsP2pRequestHandlersIfAllowed(ctx, networkManager, readonly, logger)
}

func sendRequestAndGetResponseWithCallerMethodName[ResponseType any](
	ctx context.Context,
	api *ShardApiAccessor,
	methodName string,
	args ...any,
) (ResponseType, error) {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(
			callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	return sendRequestAndGetResponse[ResponseType](api.doApiRequest, api.codec, methodName, ctx, args...)
}

func discoverAppropriatePeer(
	networkManager *network.Manager,
	shardId types.ShardId,
	protocol network.ProtocolID,
) (network.PeerID, error) {
	peersWithSpecifiedShard := networkManager.GetPeersForProtocol(protocol)
	if len(peersWithSpecifiedShard) == 0 {
		return "", fmt.Errorf("No peers with shard %d found", shardId)
	}
	return peersWithSpecifiedShard[0], nil
}

func sendRequestAndGetResponse[ResponseType any](
	doApiRequest doApiRequestFunction,
	apiCodec apiCodec,
	methodName string,
	ctx context.Context,
	args ...any,
) (ResponseType, error) {
	codec, ok := apiCodec[methodName]
	check.PanicIfNotf(ok, "Codec for method %s not found", methodName)

	var response ResponseType
	responseBody, err := doApiRequest(codec, methodName, ctx, args...)
	if err != nil {
		return response, err
	}

	return unpackResponse[ResponseType](codec, responseBody)
}

func extractCallerMethodName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	fullMethodName := fn.Name()
	parts := strings.Split(fullMethodName, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
