package rpc

import (
	"encoding/json"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/rs/zerolog"
)

var retryConfig common.RetryConfig = common.RetryConfig{
	ShouldRetry: common.LimitRetries(5),
	NextDelay:   common.ExponentialDelay(100*time.Millisecond, time.Second),
}

func NewRetryClient(rpcEndpoint string, logger zerolog.Logger) client.Client {
	return rpc.NewClient(
		rpcEndpoint,
		logger,
		rpc.RPCRetryConfig(&retryConfig),
	)
}

func doRPCCall[Req, Res any](
	rawClient client.RawClient,
	path string,
	req Req,
) (Res, error) {
	var response Res
	rawResponse, err := rawClient.RawCall(path, req)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(rawResponse, &response)
	return response, err
}
