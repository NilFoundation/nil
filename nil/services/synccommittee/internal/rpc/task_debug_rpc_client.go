package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskDebugRpcClient struct {
	client *retryRawClient
}

func NewTaskDebugRpcClient(apiEndpoint string, logger zerolog.Logger) api.TaskDebugApi {
	return &taskDebugRpcClient{
		client: newRetryRawClient(apiEndpoint, logger),
	}
}

func (c *taskDebugRpcClient) GetTasks(ctx context.Context, request *api.TaskDebugRequest) ([]*types.TaskEntry, error) {
	return callWithRetry[*api.TaskDebugRequest, []*types.TaskEntry](
		ctx,
		c.client,
		api.DebugGetTasks,
		request,
	)
}

func (c *taskDebugRpcClient) GetTaskTree(ctx context.Context, taskId types.TaskId) (*types.TaskTree, error) {
	return callWithRetry[types.TaskId, *types.TaskTree](
		ctx,
		c.client,
		api.DebugGetTaskTree,
		taskId,
	)
}
