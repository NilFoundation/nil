package rawapi

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

var ErrRequestHandlerCreation = errors.New("failed to create request handler")

// NetworkTransportProtocol is a helper interface for associating the argument and result types of Api methods
// with their Protobuf representations.
type NetworkTransportProtocol interface {
	GetBlockHeader(request pb.BlockRequest) pb.RawBlockResponse
	GetFullBlockData(request pb.BlockRequest) pb.RawFullBlockResponse
}

// Iterating through the NetworkTransportProtocol methods, we look for API methods with appropriate names.
// Next we check that the PackProtoMessage/UnpackProtoMessage functions are defined for the Protobuf request and response types.
// In this case, the following conditions are met:
//   - The set of output parameters of the UnpackProtoMessage function, up to the context and error,
//     coincides with the set of arguments of the corresponding API method
//   - The PackProtoMessage method of the response type accepts two arguments returned by the corresponding API method
//     (the second is always an error)
//
// All type checks are performed at the stage of preparing request handlers.
func SetRawApiRequestHandlers(ctx context.Context, api Api, manager *network.Manager, logger zerolog.Logger) error {
	protocolInterfaceType := reflect.TypeOf((*NetworkTransportProtocol)(nil)).Elem()
	return setRawApiRequestHandlers(ctx, protocolInterfaceType, api, "rawapi", manager, logger)
}

func setRawApiRequestHandlers(ctx context.Context, protocolInterfaceType reflect.Type, api interface{}, apiName string, manager *network.Manager, logger zerolog.Logger) error {
	requestHandlers := make(map[network.ProtocolID]network.RequestHandler)
	for m := range protocolInterfaceType.NumMethod() {
		method := protocolInterfaceType.Method(m)
		if method.PkgPath != "" {
			continue // method isn't exported
		}
		name := network.ProtocolID(apiName + "/" + method.Name)
		handler, err := makeRequestHandler(method, api, logger)
		if err != nil {
			return err
		}
		requestHandlers[name] = handler
	}
	for name, handler := range requestHandlers {
		manager.SetRequestHandler(ctx, name, handler)
	}
	return nil
}

type ErrorResponseCreator func(err error) []byte

func makeRequestHandler(method reflect.Method, api interface{}, logger zerolog.Logger) (network.RequestHandler, error) {
	logger = logger.With().Str("method", method.Name).Logger()

	apiMethodType, ok := reflect.TypeOf(api).MethodByName(method.Name)
	if !ok {
		logger.Error().Msg("Corresponding method not found in API")
		return nil, ErrRequestHandlerCreation
	}

	pbRequestType := method.Type.In(0)
	pbResponseType := method.Type.Out(0)

	unpackProtoMessage, err := obtainAndValidateRequestUnpackMethod(apiMethodType, pbRequestType)
	if err != nil {
		logger.Error().Err(err).Send()
		return nil, ErrRequestHandlerCreation
	}

	packProtoMessage, errorResponseCreator, err := obtainAndValidateResponsePackMethod(apiMethodType, pbResponseType)
	if err != nil {
		logger.Error().Err(err).Send()
		return nil, ErrRequestHandlerCreation
	}

	return func(ctx context.Context, request []byte) ([]byte, error) {
		pbRequestValuePtr := reflect.New(pbRequestType)
		message, ok := pbRequestValuePtr.Interface().(proto.Message)
		// Should never happen, so we don't use errorResponseCreator.
		check.PanicIfNotf(ok, "failed to create proto message %s", pbRequestType)
		if err := proto.Unmarshal(request, message); err != nil {
			return errorResponseCreator(err), nil
		}

		unpackedArguments, err := callMethodWithLastOutputError(unpackProtoMessage.Func, []reflect.Value{pbRequestValuePtr})
		if err != nil {
			return errorResponseCreator(err), nil
		}

		apiArguments := []reflect.Value{reflect.ValueOf(api), reflect.ValueOf(ctx)}
		apiArguments = append(apiArguments, unpackedArguments...)
		apiCallResults := apiMethodType.Func.Call(apiArguments)

		pbResponseValuePtr := reflect.New(pbResponseType)
		if _, err = callMethodWithLastOutputError(packProtoMessage.Func, append([]reflect.Value{pbResponseValuePtr}, apiCallResults...)); err != nil {
			return errorResponseCreator(err), nil
		}
		message, ok = pbResponseValuePtr.Interface().(proto.Message)
		// Should never happen, so we don't use errorResponseCreator.
		check.PanicIfNotf(ok, "failed to create proto message %s", pbResponseType)
		return proto.Marshal(message)
	}, nil
}

func obtainAndValidateRequestUnpackMethod(apiMethod reflect.Method, pbRequestType reflect.Type) (reflect.Method, error) {
	const methodName = "UnpackProtoMessage"
	unpackProtoMessage, ok := reflect.PointerTo(pbRequestType).MethodByName(methodName)
	if !ok {
		return reflect.Method{}, fmt.Errorf("method %s not found in %s", methodName, pbRequestType)
	}

	apiMethodType := apiMethod.Type
	unpackProtoMessageType := unpackProtoMessage.Type

	if !isLastOutputError(unpackProtoMessage) {
		return reflect.Method{}, fmt.Errorf("last output argument of %s.%s must be error", pbRequestType, methodName)
	}

	apiMethodSkipArgumentCount := 2 // receiver & context
	apiMethodArgumentsCount := apiMethodType.NumIn() - apiMethodSkipArgumentCount
	if apiMethodArgumentsCount != unpackProtoMessageType.NumOut()-1 { // cut off error
		return reflect.Method{}, fmt.Errorf("API method %s requires %d arguments, but %s.%s returns %d arguments, including the error", apiMethod.Name, apiMethodArgumentsCount, pbRequestType, methodName, unpackProtoMessageType.NumOut())
	}

	if !apiMethodType.In(1).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		return reflect.Method{}, fmt.Errorf("first argument of API method %s must be context.Context", apiMethod.Name)
	}

	for i := range apiMethodArgumentsCount {
		if apiMethodType.In(i+apiMethodSkipArgumentCount) != unpackProtoMessageType.Out(i) {
			return reflect.Method{}, fmt.Errorf("type of #%d (excluding the context) argument of API method %s and #%d return type of %s.%s does not match: %s != %s",
				i, apiMethod.Name, i, pbRequestType, methodName, apiMethodType.In(i+2), unpackProtoMessageType.Out(i))
		}
	}

	return unpackProtoMessage, nil
}

func obtainAndValidateResponsePackMethod(apiMethod reflect.Method, pbResponseType reflect.Type) (reflect.Method, ErrorResponseCreator, error) {
	const methodName = "PackProtoMessage"
	packProtoMessage, ok := reflect.PointerTo(pbResponseType).MethodByName("PackProtoMessage")
	if !ok {
		return reflect.Method{}, nil, fmt.Errorf("method %s not found in %s", methodName, pbResponseType)
	}

	apiMethodType := apiMethod.Type
	packProtoMessageType := packProtoMessage.Type

	if apiMethodType.NumOut() != 2 {
		return reflect.Method{}, nil, fmt.Errorf("API method %s must return exactly 2 values, but returned %d", apiMethod.Name, apiMethodType.NumOut())
	}
	if !isErrorType(apiMethodType.Out(1)) {
		return reflect.Method{}, nil, fmt.Errorf("second output argument of API method %s must be error", apiMethod.Name)
	}

	if packProtoMessageType.NumIn()-1 != 2 { // -1 for receiver
		return reflect.Method{}, nil, fmt.Errorf("%s must accept exactly 2 arguments, but accepted %d", methodName, packProtoMessageType.NumIn()-1)
	}
	if !isErrorType(packProtoMessageType.In(2)) {
		return reflect.Method{}, nil, fmt.Errorf("last argument of %s must be error", methodName)
	}

	if packProtoMessageType.NumOut() != 1 {
		return reflect.Method{}, nil, fmt.Errorf("%s must return exactly 1 value, but returned %d", methodName, packProtoMessageType.NumOut())
	}
	if !isLastOutputError(packProtoMessage) {
		return reflect.Method{}, nil, fmt.Errorf("%s of type %s must return error", methodName, pbResponseType.Name())
	}

	if apiMethodType.Out(0) != packProtoMessageType.In(1) {
		return reflect.Method{}, nil, fmt.Errorf("API method outputs %s type, but %s expects %s", apiMethodType.Out(0), methodName, packProtoMessageType.In(1))
	}
	outType := apiMethodType.Out(0)

	errorResponseCreator := func(err error) []byte {
		pbResponse := reflect.New(pbResponseType)
		_, err = callMethodWithLastOutputError(packProtoMessage.Func, []reflect.Value{pbResponse, reflect.New(outType).Elem(), reflect.ValueOf(err)})
		check.PanicIfErr(err)

		message, ok := pbResponse.Interface().(proto.Message)
		check.PanicIfNotf(ok, "failed to create proto message %s", pbResponseType)
		response, err := proto.Marshal(message)
		check.PanicIfErr(err)
		return response
	}

	return packProtoMessage, errorResponseCreator, nil
}

func isErrorType(t reflect.Type) bool {
	return t.Implements(reflect.TypeOf((*error)(nil)).Elem())
}

func isLastOutputError(method reflect.Method) bool {
	if method.Type.NumOut() == 0 {
		return false
	}
	return isErrorType(method.Type.Out(method.Type.NumOut() - 1))
}

func getError(values []reflect.Value) error {
	check.PanicIfNotf(len(values) > 0, "values must not be empty")
	lastValue := values[len(values)-1]
	if lastValue.IsNil() {
		return nil
	}
	err, ok := lastValue.Interface().(error)
	check.PanicIfNotf(ok, "last value must implement error")
	return err
}

func splitError(values []reflect.Value) ([]reflect.Value, error) {
	err := getError(values)
	return values[:len(values)-1], err
}

func callMethodWithLastOutputError(apiMethodValue reflect.Value, apiArgs []reflect.Value) ([]reflect.Value, error) {
	apiCallResults := apiMethodValue.Call(apiArgs)
	return splitError(apiCallResults)
}
