package proofprovider

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskHandlerImpl struct {
	logger zerolog.Logger
}

func (h taskHandlerImpl) HandleTask(_ context.Context, _ *types.ProverTask) (executor.TaskHandleResult, error) {
	time.Sleep(1000 * time.Millisecond)
	result := executor.TaskHandleResult{Type: types.FinalProof}
	h.logger.Debug().Msg("task handled")
	return result, nil
}

func newTaskHandler(logger zerolog.Logger) executor.TaskHandler {
	return &taskHandlerImpl{logger: logger}
}
