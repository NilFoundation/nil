package proofprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

var ErrChildTaskFailed = errors.New("child prover task failed")

type taskStateChangeHandler struct {
	resultStorage     storage.TaskResultStorage
	currentExecutorId types.TaskExecutorId
	logger            zerolog.Logger
}

func newTaskStateChangeHandler(
	resultStorage storage.TaskResultStorage,
	currentExecutorId types.TaskExecutorId,
	logger zerolog.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		resultStorage:     resultStorage,
		currentExecutorId: currentExecutorId,
		logger:            logger,
	}
}

func (h taskStateChangeHandler) OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.ParentTaskId == nil {
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).Msg("Task has nil parentTaskId")
		return nil
	}

	if task.TaskType != types.MergeProof && task.TaskType != types.AggregateProofs {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).Msgf("Task has type %d, skipping", task.TaskType)
		return nil
	}

	if !result.IsSuccess {
		log.NewTaskEvent(h.logger, zerolog.WarnLevel, task).Msgf("Prover task has failed")
	}

	var parentTaskResult *types.TaskResult
	if result.IsSuccess {
		parentTaskResult = types.NewSuccessProviderTaskResult(*task.ParentTaskId, h.currentExecutorId, result.OutputArtifacts, result.Data)
	} else {
		parentTaskResult = types.NewFailureProviderTaskResult(
			*task.ParentTaskId,
			h.currentExecutorId,
			fmt.Errorf("%w: childTaskId=%s, errorText=%s", ErrChildTaskFailed, task.Id, result.ErrorText),
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
