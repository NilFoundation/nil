package core

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	nilTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskStateChangeHandler struct {
	proposer     Proposer
	blockStorage storage.BlockStorage
	logger       zerolog.Logger
}

func newTaskStateChangeHandler(
	proposer Proposer,
	blockStorage storage.BlockStorage,
	logger zerolog.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		proposer:     proposer,
		blockStorage: blockStorage,
		logger:       logger,
	}
}

func (h taskStateChangeHandler) OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.TaskType != types.ProofBlock {
		return types.UnexpectedTaskType(task)
	}

	if !result.IsSuccess {
		h.logger.Error().
			Str("errorText", result.ErrorText).
			Interface("taskId", task.Id).
			Interface(logging.FieldShardId, nilTypes.MainShardId).
			Interface(logging.FieldBlockNumber, task.BlockNum).
			Msg("block proof task has failed, data won't be sent to the L1")
		return types.ErrBlockProofTaskFailed
	}

	return h.blockStorage.SetBlockAsProved(ctx, task.BlockHash)
}
