package core

import (
	"context"
	"fmt"

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
		h.logger.Warn().
			Str("errorText", result.ErrorText).
			Interface("taskId", task.Id).
			Interface(logging.FieldShardId, coreTypes.MainShardId).
			Interface(logging.FieldBlockNumber, task.BlockNum).
			Msg("block proof task has failed, data won't be sent to the L1")
		return nil
	}

	h.logger.Info().
		Interface("taskId", task.Id).
		Interface("batchId", task.BatchId).
		Interface("parentTaskId", task.ParentTaskId).
		Msg("Proof batch completed")

	blockId := types.NewBlockId(task.ShardId, task.BlockHash)

	if err := h.blockStorage.SetBlockAsProved(ctx, blockId); err != nil {
		return fmt.Errorf("failed to set block with id=%s as proved: %w", blockId, err)
	}

	return nil
}
