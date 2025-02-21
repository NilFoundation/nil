package core

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type ProvedBlockSetter interface {
	SetBlockAsProved(ctx context.Context, blockId types.BlockId) error
}

type StateResetLauncher interface {
	ResetState(ctx context.Context, failedMainBlockHash common.Hash) error
}

type taskStateChangeHandler struct {
	blockSetter        ProvedBlockSetter
	stateResetLauncher StateResetLauncher
	logger             zerolog.Logger
}

func newTaskStateChangeHandler(
	blockSetter ProvedBlockSetter,
	stateResetLauncher StateResetLauncher,
	logger zerolog.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		blockSetter:        blockSetter,
		stateResetLauncher: stateResetLauncher,
		logger:             logger,
	}
}

func (h *taskStateChangeHandler) OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	switch {
	case result.IsSuccess():
		log.NewTaskResultEvent(h.logger, zerolog.InfoLevel, result).
			Msg("received successful task result")
		return h.onTaskSuccess(ctx, task, result)

	case result.HasRetryableError():
		log.NewTaskResultEvent(h.logger, zerolog.WarnLevel, result).
			Msg("task execution failed with retryable error")
		return nil

	default:
		log.NewTaskResultEvent(h.logger, zerolog.WarnLevel, result).
			Msg("task execution failed with critical error, state will be reset")
		return h.stateResetLauncher.ResetState(ctx, task.BlockHash)
	}
}

func (h *taskStateChangeHandler) onTaskSuccess(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.TaskType != types.AggregateProofs {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).
			Msgf("task has type %s, just update pending dependency", task.TaskType)
		return nil
	}

	log.NewTaskResultEvent(h.logger, zerolog.InfoLevel, result).
		Stringer(logging.FieldBatchId, task.BatchId).
		Msg("Proof batch completed")

	blockId := types.NewBlockId(task.ShardId, task.BlockHash)

	if err := h.blockSetter.SetBlockAsProved(ctx, blockId); err != nil {
		return fmt.Errorf("failed to set block with id=%s as proved: %w", blockId, err)
	}

	return nil
}
