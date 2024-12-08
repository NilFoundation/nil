package rpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/rs/zerolog"
)

type retryRawClient struct {
	rawClient   client.RawClient
	retryRunner common.RetryRunner
}

func newRetryRawClient(apiEndpoint string, logger zerolog.Logger) *retryRawClient {
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.ExponentialDelay(100*time.Millisecond, time.Second),
		},
		logger,
	)

	return &retryRawClient{
		rawClient:   rpc.NewRawClient(apiEndpoint, logger),
		retryRunner: retryRunner,
	}
}

func callWithRetry[Req, Res any](
	ctx context.Context,
	client *retryRawClient,
	path string,
	req Req,
) (Res, error) {
	var rawResponse json.RawMessage
	var response Res

	err := client.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		rawResponse, err = client.rawClient.RawCall(path, req)
		return err
	})
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(rawResponse, &response)
	return response, err
}
