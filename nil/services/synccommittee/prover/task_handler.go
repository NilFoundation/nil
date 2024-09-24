package prover

import (
	"context"
	"log"

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
	if task.TaskType == types.ProofBlock {
		log.Panicf("received task (id=%s) with unexpected type %d", task.Id, task.TaskType)
	}

	// todo: implement actual task handling
	taskResult := types.SuccessTaskResult(task.Id, executorId, types.PartialProof, "1A2B")
	return h.requestHandler.SetTaskResult(ctx, &taskResult)
}
