package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type ProvedBatchSetter interface {
	BatchExists(ctx context.Context, batchId types.BatchId) (bool, error)
	SetBatchAsProved(ctx context.Context, batchId types.BatchId) error
}

//go:generate bash ../internal/scripts/generate_mock.sh StateResetLauncher

type StateResetLauncher interface {
	LaunchPartialResetWithSuspension(ctx context.Context, failedBatchId types.BatchId) error
}

type taskStateChangeHandler struct {
	batchSetter        ProvedBatchSetter
	stateResetLauncher StateResetLauncher
	logger             logging.Logger
}

func newTaskStateChangeHandler(
	batchSetter ProvedBatchSetter,
	stateResetLauncher StateResetLauncher,
	logger logging.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		batchSetter:        batchSetter,
		stateResetLauncher: stateResetLauncher,
		logger:             logger,
	}
}

func (h *taskStateChangeHandler) OnTaskTerminated(
	ctx context.Context,
	task *types.Task,
	result *types.TaskResult,
) error {
	batchExists, err := h.batchSetter.BatchExists(ctx, task.BatchId)
	if err != nil {
		return fmt.Errorf("failed to check if batch with id=%s exists: %w", task.BatchId, err)
	}
	if !batchExists {
		h.onBatchDoesNotExist(task, err, "OnTaskTerminated")
		return nil
	}

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
		return h.resetState(ctx, task, result)
	}
}

func (h *taskStateChangeHandler) resetState(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	log.NewTaskResultEvent(h.logger, zerolog.WarnLevel, result).
		Msg("task execution failed with critical error, state will be reset")

	err := h.stateResetLauncher.LaunchPartialResetWithSuspension(ctx, task.BatchId)

	switch {
	case err == nil:
		return nil

	case errors.Is(err, types.ErrBatchNotFound):
		h.onBatchDoesNotExist(task, err, "LaunchPartialResetWithSuspension")
		return nil

	default:
		return fmt.Errorf("failed to reset state, batchId=%s: %w", task.BatchId, err)
	}
}

func (h *taskStateChangeHandler) onTaskSuccess(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.TaskType != types.ProofBatch {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).
			Msgf("task has type %s, just update pending dependency", task.TaskType)
		return nil
	}

	log.NewTaskResultEvent(h.logger, zerolog.InfoLevel, result).
		Stringer(logging.FieldBatchId, task.BatchId).
		Msg("Proof batch completed")

	err := h.batchSetter.SetBatchAsProved(ctx, task.BatchId)

	switch {
	case err == nil:
		return nil

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err

	case errors.Is(err, types.ErrBatchNotFound):
		h.onBatchDoesNotExist(task, err, "SetBatchAsProved")
		return nil

	default:
		return fmt.Errorf("failed to set batch with id=%s as proved: %w", task.BatchId, err)
	}
}

func (h *taskStateChangeHandler) onBatchDoesNotExist(task *types.Task, err error, operationName string) {
	log.NewTaskEvent(h.logger, zerolog.WarnLevel, task).
		Err(err).
		Stringer(logging.FieldBatchId, task.BatchId).
		Msgf("Batch does not exist, skipping (%s)", operationName)
}
