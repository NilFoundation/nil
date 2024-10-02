package prover

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type taskHandler struct {
	requestHandler api.TaskRequestHandler
}

func newTaskHandler(requestHandler api.TaskRequestHandler) api.TaskHandler {
	return &taskHandler{
		requestHandler: requestHandler,
	}
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	if task.TaskType == types.ProofBlock {
		return types.UnexpectedTaskType(task)
	}

	// todo: implement actual task handling
	taskResult := types.SuccessProverTaskResult(task.Id, executorId, types.PartialProof, "1A2B")
	return h.requestHandler.SetTaskResult(ctx, &taskResult)
}
