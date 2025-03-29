package proofprovider

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type TaskResultSaver interface {
	Put(ctx context.Context, result *types.TaskResult) error
}

type taskStateChangeHandler struct {
	resultStorage     TaskResultSaver
	currentExecutorId types.TaskExecutorId
	logger            logging.Logger
}

func newTaskStateChangeHandler(
	resultStorage TaskResultSaver,
	currentExecutorId types.TaskExecutorId,
	logger logging.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		resultStorage:     resultStorage,
		currentExecutorId: currentExecutorId,
		logger:            logger,
	}
}

func (h taskStateChangeHandler) OnTaskTerminated(
	ctx context.Context,
	task *types.Task,
	result *types.TaskResult,
) error {
	if task.ParentTaskId == nil {
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).Msg("Task has nil parentTaskId")
		return nil
	}

	if task.TaskType != types.MergeProof {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).Msgf("Task has type %d, skipping", task.TaskType)
		return nil
	}

	if result.HasRetryableError() {
		log.NewTaskResultEvent(h.logger, zerolog.WarnLevel, result).
			Stringer(logging.FieldTaskParentId, task.ParentTaskId).
			Msgf("Child task will be rescheduled for retry, parent is not updated")
		return nil
	}

	var parentTaskResult *types.TaskResult
	if result.IsSuccess() {
		parentTaskResult = types.NewSuccessProviderTaskResult(
			*task.ParentTaskId,
			h.currentExecutorId,
			result.OutputArtifacts,
			result.Data,
		)
	} else {
		log.NewTaskResultEvent(h.logger, zerolog.WarnLevel, result).
			Stringer(logging.FieldTaskParentId, task.ParentTaskId).
			Msgf("Prover task cannot be retried, parent will marked as failed")
		parentTaskResult = types.NewFailureProviderTaskResult(
			*task.ParentTaskId,
			h.currentExecutorId,
			types.NewTaskErrChildFailed(result),
		)
	}

	err := h.resultStorage.Put(ctx, parentTaskResult)
	if err != nil {
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).
			Err(err).
			Msgf("Failed to send parent task result")
	}
	return err
}
