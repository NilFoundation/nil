package api

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type TaskHandler interface {
	Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error
	IsReadyToHandle(ctx context.Context) (bool, error)
}

//go:generate bash ../scripts/generate_mock.sh TaskHandler
