package rawapi

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"github.com/rs/zerolog"
)

var errRequestHandlerCreation = errors.New("failed to create request handler")

// NetworkTransportProtocol is a helper interface for associating the argument and result types of Api methods
// with their Protobuf representations.
type NetworkTransportProtocol interface {
	GetBlockHeader(request pb.BlockRequest) pb.RawBlockResponse
	GetFullBlockData(request pb.BlockRequest) pb.RawFullBlockResponse
	GetBlockTransactionCount(request pb.BlockRequest) pb.Uint64Response
	GetBalance(request pb.AccountRequest) pb.BalanceResponse
	GetCode(request pb.AccountRequest) pb.CodeResponse
	GetCurrencies(request pb.AccountRequest) pb.CurrenciesResponse
	Call(pb.CallRequest) pb.CallResponse
	GetInMessage(pb.MessageRequest) pb.MessageResponse
}

func SetRawApiRequestHandlers(ctx context.Context, shardId types.ShardId, api ShardApi, manager *network.Manager, logger zerolog.Logger) error {
	protocolInterfaceType := reflect.TypeOf((*NetworkTransportProtocol)(nil)).Elem()
	return setRawApiRequestHandlers(ctx, protocolInterfaceType, api, shardId, "rawapi", manager, logger)
}

func setRawApiRequestHandlers(ctx context.Context, protocolInterfaceType reflect.Type, api interface{}, shardId types.ShardId, apiName string, manager *network.Manager, logger zerolog.Logger) error {
	requestHandlers := make(map[network.ProtocolID]network.RequestHandler)
	codec, err := newApiCodec(reflect.ValueOf(api).Type(), protocolInterfaceType)
	if err != nil {
		logger.Err(err).Send()
		return errRequestHandlerCreation
	}

	apiValue := reflect.ValueOf(api)
	for method := range filtered(iterMethods(apiValue.Type()), isExportedMethod) {
		methodName := method.Name
		methodCodec, ok := codec[methodName]
		check.PanicIfNotf(ok, "Appropriate codec is not found for method %s", methodName)

		protocol := network.ProtocolID(fmt.Sprintf("/shard/%d/%s/%s", shardId, apiName, methodName))
		requestHandlers[protocol] = makeRequestHandler(apiValue.MethodByName(methodName), methodCodec)
	}
	for name, handler := range requestHandlers {
		manager.SetRequestHandler(ctx, name, handler)
	}
	return nil
}

func makeRequestHandler(apiMethod reflect.Value, codec *methodCodec) network.RequestHandler {
	return func(ctx context.Context, request []byte) ([]byte, error) {
		unpackedArguments, err := codec.unpackRequest(request)
		if err != nil {
			return codec.packError(err), nil
		}

		apiArguments := []reflect.Value{reflect.ValueOf(ctx)}
		apiArguments = append(apiArguments, unpackedArguments...)
		apiCallResults := apiMethod.Call(apiArguments)

		return codec.packResponse(apiCallResults...)
	}
}
