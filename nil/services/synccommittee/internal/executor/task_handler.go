package executor

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type TaskHandleResult struct {
	Type        types.ProverResultType
	DataAddress string
}

type TaskHandler interface {
	HandleTask(ctx context.Context, task *types.ProverTask) (TaskHandleResult, error)
}
