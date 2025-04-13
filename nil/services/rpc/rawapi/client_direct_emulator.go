package rawapi

import (
	"context"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func newShardApiClientDirectEmulator[
	ShardApiType shardApiBase,
	T interface {
		~*S
		shardApiRequestPerformerSetter
	},
	S any,
](
	apiName string,
	shardApi ShardApiType,
	apiType reflect.Type,
	transportType reflect.Type,
) (T, error) {
	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	var rv T = new(S)
	rv.setShardApiRequestPerformer(&shardApiRequestPerformerDirectEmulator[ShardApiType]{
		apiName:       apiName,
		apiType:       apiType,
		transportType: transportType,
		shardApi:      shardApi,
		codec:         codec,
		derived:       rv,
	})
	return rv, nil
}

type shardApiRequestPerformerDirectEmulator[RawApiType shardApiBase] struct {
	apiName       string
	apiType       reflect.Type
	transportType reflect.Type

	shardApi RawApiType
	codec    apiCodec

	// Reference to a specific type that uses the request performer as part of its implementation,
	// which should be used for registering libp2p handlers.
	derived any
}

var (
	_ shardApiRequestPerformer = (*shardApiRequestPerformerDirectEmulator[ShardApiRo])(nil)
	_ shardApiRequestPerformer = (*shardApiRequestPerformerDirectEmulator[ShardApiRw])(nil)
)

func (api *shardApiRequestPerformerDirectEmulator[RawApiType]) shardId() types.ShardId {
	return api.shardApi.shardId()
}

func (api *shardApiRequestPerformerDirectEmulator[RawApiType]) doApiRequest(
	ctx context.Context, codec *methodCodec, args ...any,
) ([]byte, error) {
	apiValue := reflect.ValueOf(api.shardApi)
	apiMethod := apiValue.MethodByName(codec.methodName)
	check.PanicIfNot(!apiMethod.IsZero())

	requestBody, err := codec.packRequest(args...)
	if err != nil {
		return nil, err
	}

	unpackedArguments, err := codec.unpackRequest(requestBody)
	if err != nil {
		return nil, err
	}

	// TODO: check args == unpackedArguments

	apiArguments := []reflect.Value{reflect.ValueOf(ctx)}
	apiArguments = append(apiArguments, unpackedArguments...)
	apiCallResults := apiMethod.Call(apiArguments)

	return codec.packResponse(apiCallResults...)
}

func (api *shardApiRequestPerformerDirectEmulator[RawApiType]) setNodeApi(nodeApi NodeApi) {
	api.shardApi.setNodeApi(nodeApi)
}

func (api *shardApiRequestPerformerDirectEmulator[RawApiType]) setAsP2pRequestHandlersIfAllowed(
	ctx context.Context,
	networkManager network.Manager,
	logger logging.Logger,
) error {
	return setRawApiRequestHandlers(
		ctx, api.transportType, api.apiType, api.derived, api.shardId(), api.apiName, networkManager, logger)
}

func (api *shardApiRequestPerformerDirectEmulator[RawApiType]) apiCodec() apiCodec {
	return api.codec
}
