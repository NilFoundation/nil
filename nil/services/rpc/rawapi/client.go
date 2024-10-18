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
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type NetworkShardApiAccessor struct {
	networkManager *network.Manager
	codec          apiCodec
	shardId        types.ShardId
}

var _ ShardApi = (*NetworkShardApiAccessor)(nil)

func NewNetworkRawApiAccessor(shardId types.ShardId, networkManager *network.Manager) (*NetworkShardApiAccessor, error) {
	return newNetworkRawApiAccessor(shardId, networkManager, reflect.TypeFor[ShardApi](), reflect.TypeFor[NetworkTransportProtocol]())
}

func newNetworkRawApiAccessor(shardId types.ShardId, networkManager *network.Manager, apiType, transportType reflect.Type) (*NetworkShardApiAccessor, error) {
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &NetworkShardApiAccessor{
		networkManager: networkManager,
		codec:          codec,
		shardId:        shardId,
	}, nil
}

func (api *NetworkShardApiAccessor) GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[ssz.SSZEncodedData](ctx, api, "GetBlockHeader", blockReference)
}

func (api *NetworkShardApiAccessor) GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*types.RawBlockWithExtractedData](ctx, api, "GetFullBlockData", blockReference)
}

func (api *NetworkShardApiAccessor) GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "GetBlockTransactionCount", blockReference)
}

func (api *NetworkShardApiAccessor) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Value](ctx, api, "GetBalance", address, blockReference)
}

func (api *NetworkShardApiAccessor) GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Code](ctx, api, "GetCode", address, blockReference)
}

func (api *NetworkShardApiAccessor) GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[map[types.CurrencyId]types.Value](ctx, api, "GetCurrencies", address, blockReference)
}

func (api *NetworkShardApiAccessor) Call(
	ctx context.Context, args rpctypes.CallArgs, mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
) (*rpctypes.CallResWithGasPrice, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rpctypes.CallResWithGasPrice](ctx, api, "Call", args, mainBlockReferenceOrHashWithChildren, overrides, emptyMessageIsRoot)
}

func (api *NetworkShardApiAccessor) GetInMessage(ctx context.Context, request rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.MessageInfo](ctx, api, "GetInMessage", request)
}

func (api *NetworkShardApiAccessor) GetInMessageReceipt(ctx context.Context, hash common.Hash) (*rawapitypes.ReceiptInfo, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.ReceiptInfo](ctx, api, "GetInMessageReceipt", hash)
}

func (api *NetworkShardApiAccessor) GasPrice(ctx context.Context) (types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Value](ctx, api, "GasPrice")
}

func (api *NetworkShardApiAccessor) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return sendRequestAndGetResponseWithCallerMethodName[[]types.ShardId](ctx, api, "GetShardIdList")
}

func (api *NetworkShardApiAccessor) GetMessageCount(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "GetMessageCount", address, blockReference)
}

func (api *NetworkShardApiAccessor) SendMessage(ctx context.Context, message []byte) (msgpool.DiscardReason, error) {
	return sendRequestAndGetResponseWithCallerMethodName[msgpool.DiscardReason](ctx, api, "SendMessage", message)
}

func (api *NetworkShardApiAccessor) setNodeApi(NodeApi) {
}

func sendRequestAndGetResponseWithCallerMethodName[ResponseType any](ctx context.Context, api *NetworkShardApiAccessor, methodName string, args ...any) (ResponseType, error) {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	return sendRequestAndGetResponse[ResponseType](api.codec, api.networkManager, api.shardId, "rawapi", methodName, ctx, args...)
}

func discoverAppropriatePeer(networkManager *network.Manager, shardId types.ShardId, protocol network.ProtocolID) (network.PeerID, error) {
	peersWithSpecifiedShard := networkManager.GetPeersForProtocol(protocol)
	if len(peersWithSpecifiedShard) == 0 {
		return "", fmt.Errorf("No peers with shard %d found", shardId)
	}
	return peersWithSpecifiedShard[0], nil
}

func sendRequestAndGetResponse[ResponseType any](apiCodec apiCodec, networkManager *network.Manager, shardId types.ShardId, apiName string, methodName string, ctx context.Context, args ...any) (ResponseType, error) {
	codec, ok := apiCodec[methodName]
	check.PanicIfNotf(ok, "Codec for method %s not found", methodName)

	var response ResponseType

	protocol := network.ProtocolID(fmt.Sprintf("/shard/%d/%s/%s", shardId, apiName, methodName))
	serverPeerId, err := discoverAppropriatePeer(networkManager, shardId, protocol)
	if err != nil {
		return response, err
	}

	requestBody, err := codec.packRequest(args...)
	if err != nil {
		return response, err
	}

	responseBody, err := networkManager.SendRequestAndGetResponse(ctx, serverPeerId, protocol, requestBody)
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
