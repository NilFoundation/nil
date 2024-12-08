package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskRequestRpcClient struct {
	client *retryRawClient
}

func NewTaskRequestRpcClient(apiEndpoint string, logger zerolog.Logger) api.TaskRequestHandler {
	return &taskRequestRpcClient{
		client: newRetryRawClient(apiEndpoint, logger),
	}
}

func (r *taskRequestRpcClient) GetTask(ctx context.Context, request *api.TaskRequest) (*types.Task, error) {
	return callWithRetry[*api.TaskRequest, *types.Task](
		ctx,
		r.client,
		api.TaskRequestHandlerGetTask,
		request,
	)
}

func (r *taskRequestRpcClient) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	_, err := callWithRetry[*types.TaskResult, any](
		ctx,
		r.client,
		api.TaskRequestHandlerSetTaskResult,
		result,
	)
	return err
}
