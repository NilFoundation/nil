package rpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
)

var retryConfig = common.RetryConfig{
	ShouldRetry: common.LimitRetries(5),
	NextDelay:   common.DelayExponential(100*time.Millisecond, time.Second),
}

func NewRetryClient(rpcEndpoint string, logger logging.Logger) client.Client {
	return rpc.NewClient(
		rpcEndpoint,
		logger,
		rpc.RPCRetryConfig(&retryConfig),
	)
}

func handleRPCResponse[Res any](rawResponse []byte) (Res, error) {
	var response Res
	err := json.Unmarshal(rawResponse, &response)
	return response, err
}

func doRPCCall[Res any](
	ctx context.Context,
	rawClient client.RawClient,
	path string,
) (Res, error) {
	rawResponse, err := rawClient.RawCall(ctx, path)
	if err != nil {
		var emptyRes Res
		return emptyRes, err
	}
	return handleRPCResponse[Res](rawResponse)
}

func doRPCCall2[Req, Res any](
	ctx context.Context,
	rawClient client.RawClient,
	path string,
	req Req,
) (Res, error) {
	rawResponse, err := rawClient.RawCall(ctx, path, req)
	if err != nil {
		var emptyRes Res
		return emptyRes, err
	}
	return handleRPCResponse[Res](rawResponse)
}
