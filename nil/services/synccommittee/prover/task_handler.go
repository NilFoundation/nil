package prover

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type TaskHandleResult struct {
	Type        types.ProverResultType
	DataAddress string
}

type TaskHandler interface {
	HandleTask(ctx context.Context, task *types.ProverTask) (TaskHandleResult, error)
}

type TaskHandlerImpl struct {
	logger zerolog.Logger
}

func (d TaskHandlerImpl) HandleTask(_ context.Context, _ *types.ProverTask) (TaskHandleResult, error) {
	time.Sleep(100 * time.Millisecond)
	result := TaskHandleResult{Type: types.FinalProof}
	return result, nil
}

func NewTaskHandler(logger zerolog.Logger) TaskHandler {
	return &TaskHandlerImpl{logger: logger}
}
