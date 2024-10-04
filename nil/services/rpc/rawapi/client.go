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

type NetworkRawApiAccessorImpl struct {
	networkManager *network.Manager
	serverPeerId   network.PeerID
	codec          apiCodec
}

var _ ShardApi = (*NetworkRawApiAccessorImpl)(nil)

type NetworkRawApiAccessor struct {
	impl    *NetworkRawApiAccessorImpl
	shardId types.ShardId
}

var _ ShardApi = (*NetworkRawApiAccessor)(nil)

func (api *NetworkRawApiAccessorImpl) GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return nil, nil
}

func (api *NetworkRawApiAccessorImpl) GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	return nil, nil
}

func (api *NetworkRawApiAccessorImpl) GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error) {
	return 0, nil
}

func NewNetworkRawApiAccessor(ctx context.Context, shardId types.ShardId, networkManager *network.Manager, serverAddress string) (*NetworkRawApiAccessor, error) {
	impl, err := newNetworkRawApiAccessor(ctx, networkManager, serverAddress, reflect.TypeOf(&NetworkRawApiAccessorImpl{}), reflect.TypeFor[NetworkTransportProtocol]())
	if err != nil {
		return nil, err
	}
	return &NetworkRawApiAccessor{
		impl:    impl,
		shardId: shardId,
	}, nil
}

func newNetworkRawApiAccessor(ctx context.Context, networkManager *network.Manager, serverAddress string, apiType, transportType reflect.Type) (*NetworkRawApiAccessorImpl, error) {
	serverPeerId, err := networkManager.Connect(ctx, serverAddress)
	if err != nil {
		return nil, err
	}
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	return &NetworkRawApiAccessorImpl{
		networkManager: networkManager,
		serverPeerId:   serverPeerId,
		codec:          codec,
	}, nil
}

func (api *NetworkRawApiAccessor) GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[ssz.SSZEncodedData](ctx, api.impl, api.shardId, "GetBlockHeader", blockReference)
}

func (api *NetworkRawApiAccessor) GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*types.RawBlockWithExtractedData](ctx, api.impl, api.shardId, "GetFullBlockData", blockReference)
}

func (api *NetworkRawApiAccessor) GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api.impl, api.shardId, "GetBlockTransactionCount", blockReference)
}

func sendRequestAndGetResponseWithCallerMethodName[ResponseType any](ctx context.Context, api *NetworkRawApiAccessorImpl, shardId types.ShardId, methodName string, args ...any) (ResponseType, error) {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	protocol := fmt.Sprintf("/shard/%d/rawapi", shardId)
	return sendRequestAndGetResponse[ResponseType](api.codec, api.networkManager, api.serverPeerId, protocol, methodName, ctx, args...)
}

func sendRequestAndGetResponse[ResponseType any](apiCodec apiCodec, networkManager *network.Manager, serverPeerId network.PeerID, apiName string, methodName string, ctx context.Context, args ...any) (ResponseType, error) {
	codec, ok := apiCodec[methodName]
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
