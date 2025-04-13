package rawapi

import (
	"context"
	"runtime"
	"strings"

	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
)

type doApiRequestFunction func(ctx context.Context, codec *methodCodec, args ...any) ([]byte, error)

type shardApiRequestPerformer interface {
	shardApiBase

	apiCodec() apiCodec
	doApiRequest(ctx context.Context, codec *methodCodec, args ...any) ([]byte, error)
}

type shardApiRequestPerformerSetter interface {
	setShardApiRequestPerformer(shardApiRequestPerformer)
}

func sendRequestAndGetResponseWithCallerMethodName[ResponseType any](
	ctx context.Context,
	api shardApiRequestPerformer,
	methodName string,
	args ...any,
) (ResponseType, error) {
	if assert.Enable {
		callerMethodName := extractCallerMethodName(2)
		check.PanicIfNotf(callerMethodName != "", "Method name not found")
		check.PanicIfNotf(
			callerMethodName == methodName, "Method name mismatch: %s != %s", callerMethodName, methodName)
	}
	return sendRequestAndGetResponse[ResponseType](ctx, api.doApiRequest, api.apiCodec(), methodName, args...)
}

func sendRequestAndGetResponse[ResponseType any](
	ctx context.Context,
	doApiRequest doApiRequestFunction,
	apiCodec apiCodec,
	methodName string,
	args ...any,
) (ResponseType, error) {
	codec, ok := apiCodec[methodName]
	check.PanicIfNotf(ok, "Codec for method %s not found", methodName)

	var response ResponseType
	responseBody, err := doApiRequest(ctx, codec, args...)
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
