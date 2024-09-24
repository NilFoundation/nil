package proofprovider

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type taskHandler struct{}

func newTaskHandler() executor.TaskHandler {
	return &taskHandler{}
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	// todo
	panic("not implemented")
}
