package rawapi

import (
	"context"
	"fmt"
	"iter"
	"reflect"

	"github.com/NilFoundation/nil/nil/common/check"
	"google.golang.org/protobuf/proto"
)

type ErrorResponseCreator func(err error) []byte

type methodCodec struct {
	pbRequestType        reflect.Type
	pbResponseType       reflect.Type
	requestPackMethod    reflect.Method
	requestUnpackMethod  reflect.Method
	responsePackMethod   reflect.Method
	responseUnpackMethod reflect.Method
	errorResponseCreator ErrorResponseCreator
}

func (c *methodCodec) unpackRequest(request []byte) ([]reflect.Value, error) {
	pbRequestValuePtr := reflect.New(c.pbRequestType)
	message, ok := pbRequestValuePtr.Interface().(proto.Message)
	// Should never happen, so we don't use errorResponseCreator.
	check.PanicIfNotf(ok, "failed to create proto message %s", c.pbRequestType)
	err := proto.Unmarshal(request, message)
	if err != nil {
		return nil, err
	}
	return callMethodWithLastOutputError(c.requestUnpackMethod.Func, []reflect.Value{pbRequestValuePtr})
}

func (c *methodCodec) packResponse(apiCallResults ...reflect.Value) ([]byte, error) {
	pbResponseValuePtr := reflect.New(c.pbResponseType)
	if _, err := callMethodWithLastOutputError(c.responsePackMethod.Func, append([]reflect.Value{pbResponseValuePtr}, apiCallResults...)); err != nil {
		return c.packError(err), nil
	}
	message, ok := pbResponseValuePtr.Interface().(proto.Message)
	// Should never happen, so we don't use errorResponseCreator.
	check.PanicIfNotf(ok, "failed to create proto message %s", c.pbResponseType)
	return proto.Marshal(message)
}

func (c *methodCodec) packError(err error) []byte {
	return c.errorResponseCreator(err)
}

type apiCodec map[string]*methodCodec

// Iterating through the API methods, we look for NetworkTransportProtocol methods with appropriate names.
// Next we check that the PackProtoMessage/UnpackProtoMessage functions are defined for the Protobuf request and response types.
// In this case, the following conditions are met:
//   - The set of output parameters of the UnpackProtoMessage function, up to the context and error,
//     coincides with the set of arguments of the corresponding API method
//   - The PackProtoMessage method of the response type accepts two arguments returned by the corresponding API method
//     (the second is always an error)
func newApiCodec(api, transport reflect.Type) (*apiCodec, error) {
	apiCodec := (*apiCodec)(&map[string]*methodCodec{})
	for apiMethod := range filtered(iterMethods(api), isExportedMethod) {
		transportMethod, ok := transport.MethodByName(apiMethod.Name)
		if !ok {
			return nil, fmt.Errorf("method %s not found in %s", apiMethod.Name, transport)
		}
		pbRequestType, pbResponseType, err := extractPbTypes(transport, transportMethod)
		if err != nil {
			return nil, err
		}
		requestUnpackMethod, err := obtainAndValidateRequestUnpackMethod(apiMethod, pbRequestType)
		if err != nil {
			return nil, err
		}
		responsePackMethod, errorResponseCreator, err := obtainAndValidateResponsePackMethod(apiMethod, pbResponseType)
		if err != nil {
			return nil, err
		}

		(*apiCodec)[apiMethod.Name] = &methodCodec{
			pbRequestType:  pbRequestType,
			pbResponseType: pbResponseType,
			// requestPackMethod:    ...,
			requestUnpackMethod: requestUnpackMethod,
			responsePackMethod:  responsePackMethod,
			// responseUnpackMethod: ...,
			errorResponseCreator: errorResponseCreator,
		}
	}
	return apiCodec, nil
}

func iterMethods(t reflect.Type) iter.Seq[reflect.Method] {
	type Yield = func(p reflect.Method) bool
	return func(yield Yield) {
		for i := range t.NumMethod() {
			if !yield(t.Method(i)) {
				return
			}
		}
	}
}

func isExportedMethod(m reflect.Method) bool {
	return m.PkgPath == ""
}

func filtered[T any](seq iter.Seq[T], filter func(T) bool) iter.Seq[T] {
	type Yield = func(T) bool
	return func(yield Yield) {
		for v := range seq {
			if filter(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

func extractPbTypes(transportApiType reflect.Type, method reflect.Method) (reflect.Type, reflect.Type, error) {
	if method.Type.NumIn() != 1 {
		return nil, nil, fmt.Errorf("method %s.%s must have exactly 1 argument", transportApiType.Name(), method.Name)
	}
	if method.Type.NumOut() != 1 {
		return nil, nil, fmt.Errorf("method %s.%s must have exactly 1 return value", transportApiType.Name(), method.Name)
	}
	return method.Type.In(0), method.Type.Out(0), nil
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
