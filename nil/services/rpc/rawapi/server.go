package rawapi

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"unicode"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi/pb"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"
)

var ErrRequestHandlerCreation = errors.New("failed to create request handler")

type NetworkTransportProtocol interface {
	GetBlockHeader(ctx context.Context, request pb.BlockRequest) pb.RawBlockResponse
	GetFullBlockData(ctx context.Context, request pb.BlockRequest) pb.RawFullBlockResponse
}

func SetRawApiRequestHandlers(ctx context.Context, localApi Api, manager *network.Manager, logger zerolog.Logger) error {
	ifaceT := reflect.TypeOf((*NetworkTransportProtocol)(nil)).Elem()
	requestHandlers := make(map[network.ProtocolID]network.RequestHandler)
	for m := range ifaceT.NumMethod() {
		method := ifaceT.Method(m)
		if method.PkgPath != "" {
			continue // method isn't exported
		}
		name := formatName(method.Name)
		handler, err := makeRequestHandler(method, localApi, logger)
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

func formatName(name string) network.ProtocolID {
	ret := []rune(name)
	if len(ret) > 0 {
		ret[0] = unicode.ToLower(ret[0])
	}
	return network.ProtocolID(ret)
}

type ErrorResponseCreator func(err error) []byte

func makeRequestHandler(method reflect.Method, localApi Api, logger zerolog.Logger) (network.RequestHandler, error) {
	logger = logger.With().Str("method", method.Name).Logger()

	apiMethodType, ok := reflect.TypeOf(localApi).MethodByName(method.Name)
	if !ok {
		logger.Error().Msg("Corresponding method not found in API")
		return nil, ErrRequestHandlerCreation
	}

	pbRequestType := method.Type.In(1)
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

	return func(ctx context.Context, request []byte) (_ []byte, errRes error) {
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

		apiArguments := []reflect.Value{reflect.ValueOf(localApi), reflect.ValueOf(ctx)}
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
