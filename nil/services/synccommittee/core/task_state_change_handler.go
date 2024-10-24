package core

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskStateChangeHandler struct {
	blockStorage storage.BlockStorage
	logger       zerolog.Logger
}

func newTaskStateChangeHandler(
	blockStorage storage.BlockStorage,
	logger zerolog.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		blockStorage: blockStorage,
		logger:       logger,
	}
}

func (h taskStateChangeHandler) OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.TaskType != types.AggregateProofs {
		h.logger.Debug().
			Interface("taskId", task.Id).
			Interface("parentTaskId", task.ParentTaskId).
			Msgf("task has type %s, just update pending dependency", task.TaskType)
		return nil
	}

	if !result.IsSuccess {
		h.logger.Error().
			Str("errorText", result.ErrorText).
			Interface("taskId", task.Id).
			Interface(logging.FieldShardId, coreTypes.MainShardId).
			Interface(logging.FieldBlockNumber, task.BlockNum).
			Msg("block proof task has failed, data won't be sent to the L1")
		return types.ErrBlockProofTaskFailed
	}

	h.logger.Info().
		Interface("taskId", task.Id).
		Interface("batchId", task.BatchId).
		Interface("parentTaskId", task.ParentTaskId).
		Msg("Proof bacth completed")

	return h.blockStorage.SetBlockAsProved(ctx, coreTypes.MainShardId, task.BlockHash)
}
