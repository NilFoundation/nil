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
	TransportType any,
	T interface {
		~*S
		shardApiRequestPerformerSetter
	},
	S any,
](
	apiName string,
	shardApi ShardApiType,
) (T, error) {
	apiType := reflect.TypeFor[ShardApiType]()
	transportType := reflect.TypeFor[TransportType]()

	codec, err := newApiCodec(apiType, transportType)
	if err != nil {
		return nil, err
	}

	var rv T = new(S)
	rv.setShardApiRequestPerformer(&shardApiRequestPerformerDirectEmulator{
		apiName:       apiName,
		apiType:       apiType,
		transportType: transportType,
		shardApi:      shardApi,
		codec:         codec,
		derived:       rv,
	})
	return rv, nil
}

type shardApiRequestPerformerDirectEmulator struct {
	apiName       string
	apiType       reflect.Type
	transportType reflect.Type

	shardApi shardApiBase
	codec    apiCodec

	// Reference to a specific type that uses the request performer as part of its implementation,
	// which should be used for registering libp2p handlers.
	derived any
}

var _ shardApiRequestPerformer = (*shardApiRequestPerformerDirectEmulator)(nil)

func (api *shardApiRequestPerformerDirectEmulator) shardId() types.ShardId {
	return api.shardApi.shardId()
}

func (api *shardApiRequestPerformerDirectEmulator) doApiRequest(
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

func (api *shardApiRequestPerformerDirectEmulator) setNodeApi(nodeApi NodeApi) {
	api.shardApi.setNodeApi(nodeApi)
}

func (api *shardApiRequestPerformerDirectEmulator) setAsP2pRequestHandlersIfAllowed(
	ctx context.Context,
	networkManager network.Manager,
	logger logging.Logger,
) error {
	return setRawApiRequestHandlers(
		ctx, api.transportType, api.apiType, api.derived, api.shardId(), api.apiName, networkManager, logger)
}

func (api *shardApiRequestPerformerDirectEmulator) apiCodec() apiCodec {
	return api.codec
}
