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

func doRPCCall[Req, Res any](
	ctx context.Context,
	rawClient client.RawClient,
	path string,
	req Req,
) (Res, error) {
	var response Res
	rawResponse, err := rawClient.RawCall(ctx, path, req)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(rawResponse, &response)
	return response, err
}
