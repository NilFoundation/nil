package proofprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

var ErrChildTaskFailed = errors.New("child prover task failed")

type taskStateChangeHandler struct {
	requestHandler    api.TaskRequestHandler
	currentExecutorId types.TaskExecutorId
	logger            zerolog.Logger
}

func newTaskStateChangeHandler(
	requestHandler api.TaskRequestHandler,
	currentExecutorId types.TaskExecutorId,
	logger zerolog.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		requestHandler:    requestHandler,
		currentExecutorId: currentExecutorId,
		logger:            logger,
	}
}

func (h taskStateChangeHandler) OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.ParentTaskId == nil {
		h.logger.Error().
			Interface("taskId", task.Id).
			Msgf("task with id=%s has nil parentTaskId", task.Id)
		return nil
	}

	if task.TaskType != types.MergeProof && task.TaskType != types.AggregateProofs {
		h.logger.Debug().
			Interface("taskId", task.Id).
			Interface("parentTaskId", task.ParentTaskId).
			Msgf("task has type %d, skipping", task.TaskType)
		return nil
	}

	if !result.IsSuccess {
		h.logger.Warn().
			Str("errorText", result.ErrorText).
			Interface("taskId", task.Id).
			Interface("parentTaskId", task.ParentTaskId).
			Msgf("prover task has failed")
	}

	var parentTaskResult *types.TaskResult
	if result.IsSuccess {
		parentTaskResult = types.NewSuccessProviderTaskResult(*task.ParentTaskId, h.currentExecutorId, result.DataAddresses, result.Data)
	} else {
		parentTaskResult = types.NewFailureProviderTaskResult(
			*task.ParentTaskId,
			h.currentExecutorId,
			fmt.Errorf("%w: childTaskId=%s, errorText=%s", ErrChildTaskFailed, task.Id, result.ErrorText),
		)
	}

	err := h.requestHandler.SetTaskResult(ctx, parentTaskResult)
	if err != nil {
		h.logger.Error().
			Err(err).
			Interface("taskId", task.Id).
			Interface("parentTaskId", task.ParentTaskId).
			Msgf("failed to send parent task result")
	}
	return err
}
