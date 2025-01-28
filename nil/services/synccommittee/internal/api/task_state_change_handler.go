package api

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type TaskStateChangeHandler interface {
	OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error
}

//go:generate bash ../scripts/generate_mock.sh TaskStateChangeHandler
