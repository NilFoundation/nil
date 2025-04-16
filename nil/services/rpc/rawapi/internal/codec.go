package internal

import (
	"context"
	"fmt"
	"iter"
	"reflect"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"google.golang.org/protobuf/proto"
)

type methodCodec struct {
	methodName           string
	apiMethodResultType  reflect.Type
	pbRequestType        reflect.Type
	pbResponseType       reflect.Type
	requestPackMethod    reflect.Method
	requestUnpackMethod  reflect.Method
	responsePackMethod   reflect.Method
	responseUnpackMethod reflect.Method
}

func (c *methodCodec) packRequest(apiArgs ...any) ([]byte, error) {
	if c.pbRequestType == nil {
		return nil, nil
	}

	pbRequestValuePtr := reflect.New(c.pbRequestType)
	args := []reflect.Value{pbRequestValuePtr}
	for _, arg := range apiArgs {
		args = append(args, reflect.ValueOf(arg))
	}
	_, err := callMethodWithLastOutputError(c.requestPackMethod.Func, args)
	if err != nil {
		return nil, err
	}
	transaction, ok := pbRequestValuePtr.Interface().(proto.Message)
	// Should never happen, so we don't pack error to response.
	check.PanicIfNotf(ok, "failed to create proto transaction %s", c.pbRequestType)
	request, err := proto.Marshal(transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to pack Protobuf request: %w", err)
	}
	return request, nil
}

func (c *methodCodec) unpackRequest(request []byte) ([]reflect.Value, error) {
	if c.pbRequestType == nil {
		return nil, nil
	}

	pbRequestValuePtr := reflect.New(c.pbRequestType)
	transaction, ok := pbRequestValuePtr.Interface().(proto.Message)
	// Should never happen, so we don't pack error to response.
	check.PanicIfNotf(ok, "failed to create proto transaction %s", c.pbRequestType)
	err := proto.Unmarshal(request, transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack Protobuf request: %w", err)
	}
	return callMethodWithLastOutputError(c.requestUnpackMethod.Func, []reflect.Value{pbRequestValuePtr})
}

func (c *methodCodec) packResponse(apiCallResults ...reflect.Value) ([]byte, error) {
	pbResponseValuePtr := reflect.New(c.pbResponseType)
	if _, err := callMethodWithLastOutputError(
		c.responsePackMethod.Func,
		append([]reflect.Value{pbResponseValuePtr}, apiCallResults...)); err != nil {
		return c.packError(err), nil
	}
	transaction, ok := pbResponseValuePtr.Interface().(proto.Message)
	// Should never happen, so we don't pack error to response.
	check.PanicIfNotf(ok, "failed to create proto transaction %s", c.pbResponseType)
	response, err := proto.Marshal(transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to pack Protobuf response: %w", err)
	}
	return response, nil
}

func (c *methodCodec) packError(err error) []byte {
	pbResponseValuePtr := reflect.New(c.pbResponseType)
	_, err = callMethodWithLastOutputError(
		c.responsePackMethod.Func,
		[]reflect.Value{pbResponseValuePtr, reflect.New(c.apiMethodResultType).Elem(), reflect.ValueOf(err)})
	check.PanicIfErr(err)

	transaction, ok := pbResponseValuePtr.Interface().(proto.Message)
	check.PanicIfNotf(ok, "failed to create proto transaction %s", c.pbResponseType)
	response, err := proto.Marshal(transaction)
	check.PanicIfErr(err)
	return response
}

func (c *methodCodec) unpackResponse(response []byte) (any, error) {
	pbResponseValuePtr := reflect.New(c.pbResponseType)
	transaction, ok := pbResponseValuePtr.Interface().(proto.Message)
	check.PanicIfNotf(ok, "failed to create proto transaction %s", c.pbResponseType)
	err := proto.Unmarshal(response, transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack Protobuf response: %w", err)
	}
	resp, err := callMethodWithLastOutputError(c.responseUnpackMethod.Func, []reflect.Value{pbResponseValuePtr})
	if err != nil {
		return nil, err
	}
	check.PanicIfNot(len(resp) == 1)
	return resp[0].Interface(), err
}

func unpackResponse[ResultType any](codec *methodCodec, response []byte) (ResultType, error) {
	var result ResultType
	resp, err := codec.unpackResponse(response)
	if err != nil {
		return result, err
	}
	var ok bool
	result, ok = resp.(ResultType)
	check.PanicIfNotf(ok, "unexpected response type: %T", resp)
	return result, nil
}

type apiCodec map[string]*methodCodec

// Iterating through the API methods, we look for NetworkTransportProtocol methods with appropriate names.
// Next we check that the PackProtoMessage/UnpackProtoMessage functions are defined for the Protobuf request
// and response types.
// In this case, the following conditions are met:
//   - The PackProtoMessage method of the request type accepts the same arguments as the corresponding API method
//     (excluding the context)
//   - The set of output parameters of the UnpackProtoMessage method of the request, up to the context and error,
//     coincides with the set of arguments of the corresponding API method
//   - The PackProtoMessage method of the response type accepts two arguments returned by the corresponding API method
//     (the second is always an error)
//   - The UnpackProtoMessage method of the response type returns the same type as the corresponding API method
//     and error
//
// If any of the conditions are not met, an error is returned.
func newApiCodec(api, transport reflect.Type) (apiCodec, error) {
	apiCodec := make(apiCodec)
	for apiMethod := range common.Filter(iterMethods(api), isExportedMethod) {
		if err := checkApiMethodSignature(apiMethod); err != nil {
			return nil, err
		}

		transportMethod, ok := transport.MethodByName(apiMethod.Name)
		if !ok {
			return nil, fmt.Errorf("method %s not found in %s", apiMethod.Name, transport)
		}
		pbRequestType, pbResponseType, err := checkTransportMethodSignatureAndExtractPbTypes(transport, transportMethod)
		if err != nil {
			return nil, err
		}
		requestPackMethod, requestUnpackMethod, err := //
			obtainAndValidateRequestConversionMethods(apiMethod, pbRequestType)
		if err != nil {
			return nil, err
		}
		responsePackMethod, responseUnpackMethod, err := //
			obtainAndValidateResponseConversionMethods(apiMethod, pbResponseType)
		if err != nil {
			return nil, err
		}

		apiCodec[apiMethod.Name] = &methodCodec{
			methodName:           apiMethod.Name,
			apiMethodResultType:  apiMethod.Type.Out(0),
			pbRequestType:        pbRequestType,
			pbResponseType:       pbResponseType,
			requestPackMethod:    requestPackMethod,
			requestUnpackMethod:  requestUnpackMethod,
			responsePackMethod:   responsePackMethod,
			responseUnpackMethod: responseUnpackMethod,
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
	return m.IsExported()
}

func checkTransportMethodSignatureAndExtractPbTypes(
	transportApiType reflect.Type,
	method reflect.Method,
) (reflect.Type, reflect.Type, error) {
	if method.Type.NumIn() > 1 {
		return nil, nil, fmt.Errorf(
			"method %s.%s must have 1 or 0 arguments, got: %d",
			transportApiType.Name(), method.Name, method.Type.NumIn())
	}
	if method.Type.NumOut() != 1 {
		return nil, nil, fmt.Errorf(
			"method %s.%s must have exactly 1 return value", transportApiType.Name(), method.Name)
	}
	var reqType reflect.Type
	if method.Type.NumIn() == 1 {
		reqType = method.Type.In(0)
	}
	return reqType, method.Type.Out(0), nil
}

func checkApiMethodSignature(apiMethod reflect.Method) error {
	apiMethodType := apiMethod.Type
	if apiMethodType.NumIn() < 1 {
		return fmt.Errorf("API method %s must have at least one argument", apiMethod.Name)
	}
	if !apiMethodType.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		return fmt.Errorf("first argument of API method %s must be context.Context", apiMethod.Name)
	}

	if apiMethodType.NumOut() != 2 {
		return fmt.Errorf(
			"API method %s must return exactly 2 values, but returned %d", apiMethod.Name, apiMethodType.NumOut())
	}
	if !isErrorType(apiMethodType.Out(1)) {
		return fmt.Errorf("second output argument of API method %s must be error", apiMethod.Name)
	}
	return nil
}

func obtainAndValidateRequestConversionMethods(
	apiMethod reflect.Method,
	pbRequestType reflect.Type,
) (reflect.Method, reflect.Method, error) {
	if pbRequestType == nil {
		return reflect.Method{}, reflect.Method{}, nil
	}

	const packMethodName = "PackProtoMessage"
	const unpackMethodName = "UnpackProtoMessage"

	packProtoMessage, ok := reflect.PointerTo(pbRequestType).MethodByName(packMethodName)
	if !ok {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"method %s not found in %s", packMethodName, pbRequestType)
	}

	unpackProtoMessage, ok := reflect.PointerTo(pbRequestType).MethodByName(unpackMethodName)
	if !ok {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"method %s not found in %s", unpackMethodName, pbRequestType)
	}

	apiMethodType := apiMethod.Type
	packProtoMessageType := packProtoMessage.Type
	unpackProtoMessageType := unpackProtoMessage.Type

	if packProtoMessageType.NumOut() != 1 {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"%s must return exactly 1 value, but returned %d", packMethodName, packProtoMessageType.NumOut())
	}
	if !isLastOutputError(packProtoMessage) {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"%s of type %s must return error", packMethodName, pbRequestType.Name())
	}

	if !isLastOutputError(unpackProtoMessage) {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"last output argument of %s.%s must be error", pbRequestType, unpackMethodName)
	}

	apiMethodSkipArgumentCount := 1 // context
	apiMethodArgumentsCount := apiMethodType.NumIn() - apiMethodSkipArgumentCount
	packProtoMessageSkipArgumentCount := 1 // receiver
	packProtoMessageArgumentCount := packProtoMessageType.NumIn() - packProtoMessageSkipArgumentCount
	unpackProtoMessageSkipResultCount := 1 // cut off error
	unpackProtoMessageResultCount := unpackProtoMessageType.NumOut() - unpackProtoMessageSkipResultCount

	if apiMethodArgumentsCount != packProtoMessageArgumentCount {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"API method %s requires %d arguments, but %s.%s accepts %d arguments",
			apiMethod.Name, apiMethodArgumentsCount, pbRequestType, packMethodName, packProtoMessageArgumentCount)
	}
	if apiMethodArgumentsCount != unpackProtoMessageResultCount {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"API method %s requires %d arguments, but %s.%s returns %d arguments, including the error",
			apiMethod.Name, apiMethodArgumentsCount, pbRequestType, unpackMethodName, unpackProtoMessageType.NumOut())
	}

	for i := range apiMethodArgumentsCount {
		if apiMethodType.In(i+apiMethodSkipArgumentCount) !=
			packProtoMessageType.In(i+packProtoMessageSkipArgumentCount) {
			return reflect.Method{}, reflect.Method{}, fmt.Errorf(
				"type of #%d (excluding the context) argument of API method %s and #%d of %s.%s does not match: %s != %s", //nolint: lll
				i, apiMethod.Name,
				i, pbRequestType, packMethodName,
				apiMethodType.In(i+apiMethodSkipArgumentCount),
				packProtoMessageType.In(i+packProtoMessageSkipArgumentCount))
		}
		if apiMethodType.In(i+apiMethodSkipArgumentCount) != unpackProtoMessageType.Out(i) {
			return reflect.Method{}, reflect.Method{}, fmt.Errorf(
				"type of #%d (excluding the context) argument of API method %s and #%d return type of %s.%s does not match: %s != %s", //nolint: lll
				i, apiMethod.Name,
				i, pbRequestType, unpackMethodName,
				apiMethodType.In(i+apiMethodSkipArgumentCount), unpackProtoMessageType.Out(i))
		}
	}

	return packProtoMessage, unpackProtoMessage, nil
}

func obtainAndValidateResponseConversionMethods(
	apiMethod reflect.Method,
	pbResponseType reflect.Type,
) (reflect.Method, reflect.Method, error) {
	const packMethodName = "PackProtoMessage"
	const unpackMethodName = "UnpackProtoMessage"

	packProtoMessage, ok := reflect.PointerTo(pbResponseType).MethodByName(packMethodName)
	if !ok {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"method %s not found in %s", packMethodName, pbResponseType)
	}

	unpackProtoMessage, ok := reflect.PointerTo(pbResponseType).MethodByName(unpackMethodName)
	if !ok {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"method %s not found in %s", unpackMethodName, pbResponseType)
	}

	apiMethodType := apiMethod.Type
	packProtoMessageType := packProtoMessage.Type
	unpackProtoMessageType := unpackProtoMessage.Type

	if packProtoMessageType.NumIn()-1 != 2 {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"%s must accept exactly 2 arguments, but accepted %d",
			packMethodName, packProtoMessageType.NumIn()-1)
	}
	if !isErrorType(packProtoMessageType.In(2)) {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf("last argument of %s must be error", packMethodName)
	}

	if unpackProtoMessageType.NumIn() != 1 {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"%s must accept exactly 1 argument, but accepted %d",
			unpackMethodName, unpackProtoMessageType.NumIn())
	}
	if unpackProtoMessageType.NumOut() != 2 {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"%s must return exactly 2 values, but returned %d",
			unpackMethodName, unpackProtoMessageType.NumOut())
	}
	if !isErrorType(unpackProtoMessageType.Out(1)) {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"last output argument of %s must be error", unpackMethodName)
	}

	if apiMethodType.Out(0) != packProtoMessageType.In(1) {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"API method outputs %s type, but %s expects %s",
			apiMethodType.Out(0), packMethodName, packProtoMessageType.In(1))
	}

	if apiMethodType.Out(0) != unpackProtoMessageType.Out(0) {
		return reflect.Method{}, reflect.Method{}, fmt.Errorf(
			"API method outputs %s type, but %s expects %s",
			apiMethodType.Out(0), unpackMethodName, unpackProtoMessageType.Out(0))
	}

	return packProtoMessage, unpackProtoMessage, nil
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
