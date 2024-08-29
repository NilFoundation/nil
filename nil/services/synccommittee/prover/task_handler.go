package prover

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/rs/zerolog"
)

// todo: replace ProverTaskView here with type from PR #788

type TaskHandler interface {
	HandleTask(ctx context.Context, task *api.ProverTaskView) error
}

type TaskHandlerImpl struct {
	logger zerolog.Logger
}

func (d TaskHandlerImpl) HandleTask(_ context.Context, _ *api.ProverTaskView) error {
	time.Sleep(100 * time.Millisecond)
	return nil
}

func NewTaskHandler(logger zerolog.Logger) TaskHandler {
	return &TaskHandlerImpl{logger: logger}
}
