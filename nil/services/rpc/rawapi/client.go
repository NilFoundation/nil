package rawapi

import (
	"context"
	"reflect"
	"runtime"
	"strings"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

type NetworkRawApiAccessor struct {
	networkManager *network.Manager
	serverPeerId   network.PeerID
	codec          *apiCodec
}

var _ Api = (*NetworkRawApiAccessor)(nil)

func NewNetworkRawApiAccessor(ctx context.Context, networkManager *network.Manager, serverAddress string) (*NetworkRawApiAccessor, error) {
	return newNetworkRawApiAccessor(ctx, networkManager, serverAddress, reflect.TypeOf(&NetworkRawApiAccessor{}), reflect.TypeFor[NetworkTransportProtocol]())
}

func newNetworkRawApiAccessor(ctx context.Context, networkManager *network.Manager, serverAddress string, apiType, transportType reflect.Type) (*NetworkRawApiAccessor, error) {
	serverPeerId, err := networkManager.Connect(ctx, serverAddress)
	if err != nil {
		return nil, err
	}
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &NetworkRawApiAccessor{
		networkManager: networkManager,
		serverPeerId:   serverPeerId,
		codec:          codec,
	}, nil
}

func (api *NetworkRawApiAccessor) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[ssz.SSZEncodedData](api, ctx, shardId, blockReference)
}

func (api *NetworkRawApiAccessor) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*types.RawBlockWithExtractedData](api, ctx, shardId, blockReference)
}

func sendRequestAndGetResponseWithCallerMethodName[ResponseType any](api *NetworkRawApiAccessor, ctx context.Context, args ...any) (ResponseType, error) {
	callerMethodName := extractCallerMethodName(2)
	check.PanicIfNotf(callerMethodName != "", "Method name not found")
	return sendRequestAndGetResponse[ResponseType](api.codec, api.networkManager, api.serverPeerId, "rawapi", callerMethodName, ctx, args...)
}

func sendRequestAndGetResponse[ResponseType any](apiCodec *apiCodec, networkManager *network.Manager, serverPeerId network.PeerID, apiName string, methodName string, ctx context.Context, args ...any) (ResponseType, error) {
	codec, ok := (*apiCodec)[methodName]
	check.PanicIfNotf(ok, "Codec for method %s not found", methodName)

	var response ResponseType
	requestBody, err := codec.packRequest(args...)
	if err != nil {
		return response, err
	}

	responseBody, err := networkManager.SendRequestAndGetResponse(ctx, serverPeerId, network.ProtocolID(apiName+"/"+codec.methodName), requestBody)
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
