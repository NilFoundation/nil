package rawapi

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

type NetworkShardApiAccessor struct {
	networkManager *network.Manager
	serverPeerId   network.PeerID
	codec          apiCodec
	shardId        types.ShardId
}

var _ ShardApi = (*NetworkShardApiAccessor)(nil)

func NewNetworkRawApiAccessor(ctx context.Context, shardId types.ShardId, networkManager *network.Manager, serverAddress string) (*NetworkShardApiAccessor, error) {
	return newNetworkRawApiAccessor(ctx, shardId, networkManager, serverAddress, reflect.TypeFor[*NetworkShardApiAccessor](), reflect.TypeFor[NetworkTransportProtocol]())
}

func newNetworkRawApiAccessor(ctx context.Context, shardId types.ShardId, networkManager *network.Manager, serverAddress string, apiType, transportType reflect.Type) (*NetworkShardApiAccessor, error) {
	serverPeerId, err := networkManager.Connect(ctx, serverAddress)
	if err != nil {
		return nil, err
	}
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &NetworkShardApiAccessor{
		networkManager: networkManager,
		serverPeerId:   serverPeerId,
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

func sendRequestAndGetResponseWithCallerMethodName[ResponseType any](ctx context.Context, api *NetworkShardApiAccessor, methodName string, args ...any) (ResponseType, error) {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	return sendRequestAndGetResponse[ResponseType](api.codec, api.networkManager, api.serverPeerId, api.shardId, "rawapi", methodName, ctx, args...)
}

func sendRequestAndGetResponse[ResponseType any](apiCodec apiCodec, networkManager *network.Manager, serverPeerId network.PeerID, shardId types.ShardId, apiName string, methodName string, ctx context.Context, args ...any) (ResponseType, error) {
	codec, ok := apiCodec[methodName]
	check.PanicIfNotf(ok, "Codec for method %s not found", methodName)

	var response ResponseType
	requestBody, err := codec.packRequest(args...)
	if err != nil {
		return response, err
	}

	protocol := network.ProtocolID(fmt.Sprintf("/shard/%d/%s/%s", shardId, apiName, methodName))
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
