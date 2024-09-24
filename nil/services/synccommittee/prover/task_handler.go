package prover

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type taskHandler struct {
	requestHandler api.TaskRequestHandler
}

func newTaskHandler(requestHandler api.TaskRequestHandler) executor.TaskHandler {
	return &taskHandler{
		requestHandler: requestHandler,
	}
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	// todo: implement actual task handling
	taskResult := types.SuccessTaskResult(task.Id, executorId, types.PartialProof, "1A2B")
	return h.requestHandler.SetTaskResult(ctx, &taskResult)
}
