package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type taskDebugRpcClient struct {
	client client.RawClient
}

func NewTaskDebugRpcClient(apiEndpoint string, logger logging.Logger) public.TaskDebugApi {
	return &taskDebugRpcClient{
		client: NewRetryClient(apiEndpoint, logger),
	}
}

func (c *taskDebugRpcClient) GetTasks(
	ctx context.Context,
	request *public.TaskDebugRequest,
) ([]*public.TaskView, error) {
	return doRPCCall[*public.TaskDebugRequest, []*public.TaskView](
		ctx,
		c.client,
		public.DebugGetTasks,
		request,
	)
}

func (c *taskDebugRpcClient) GetTaskTree(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error) {
	return doRPCCall[types.TaskId, *public.TaskTreeView](
		ctx,
		c.client,
		public.DebugGetTaskTree,
		taskId,
	)
}
