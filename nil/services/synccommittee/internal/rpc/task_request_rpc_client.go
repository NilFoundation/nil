package rpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

const (
	maxRetryCount = 5
	initialDelay  = 100 * time.Millisecond
)

type TaskRequestRpcClient struct {
	client      client.RawClient
	retryRunner common.RetryRunner
}

func NewTaskRequestRpcClient(apiEndpoint string, logger zerolog.Logger) *TaskRequestRpcClient {
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: func(attemptNumber uint32, _ error) bool {
				return attemptNumber < maxRetryCount
			},
			NextDelay: func(attemptNumber uint32) time.Duration {
				result := time.Duration(1)
				for range attemptNumber {
					result *= initialDelay
				}
				return result
			},
		},
		logger,
	)

	return &TaskRequestRpcClient{
		client:      rpc.NewRawClient(apiEndpoint, logger),
		retryRunner: retryRunner,
	}
}

func (r *TaskRequestRpcClient) GetTask(ctx context.Context, request *api.TaskRequest) (*types.Task, error) {
	var response json.RawMessage

	err := r.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		response, err = r.client.RawCall(api.TaskRequestHandlerGetTask, request)
		return err
	})
	if err != nil {
		return nil, err
	}

	var task *types.Task
	if err = json.Unmarshal(response, &task); err != nil {
		return nil, err
	}

	return task, nil
}

func (r *TaskRequestRpcClient) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	return r.retryRunner.Do(ctx, func(ctx context.Context) error {
		_, err := r.client.RawCall(api.TaskRequestHandlerSetTaskResult, result)
		return err
	})
}
