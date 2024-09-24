package prover

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

func (h taskHandlerImpl) HandleTask(_ context.Context, _ *types.Task) (executor.TaskHandleResult, error) {
	time.Sleep(100 * time.Millisecond)
	result := executor.TaskHandleResult{Type: types.FinalProof}
	h.logger.Debug().Msg("task handled")
	return result, nil
}

func _(logger zerolog.Logger) executor.TaskHandler {
	return &taskHandlerImpl{logger: logger}
}
