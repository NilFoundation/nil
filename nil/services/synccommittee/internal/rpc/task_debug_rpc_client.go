package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/rs/zerolog"
)

type taskDebugRpcClient struct {
	client *retryRawClient
}

func NewTaskDebugRpcClient(apiEndpoint string, logger zerolog.Logger) public.TaskDebugApi {
	return &taskDebugRpcClient{
		client: newRetryRawClient(apiEndpoint, logger),
	}
}

func (c *taskDebugRpcClient) GetTasks(ctx context.Context, request *public.TaskDebugRequest) ([]*public.TaskView, error) {
	return callWithRetry[*public.TaskDebugRequest, []*public.TaskView](
		ctx,
		c.client,
		public.DebugGetTasks,
		request,
	)
}

func (c *taskDebugRpcClient) GetTaskTree(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error) {
	return callWithRetry[types.TaskId, *public.TaskTreeView](
		ctx,
		c.client,
		public.DebugGetTaskTree,
		taskId,
	)
}
