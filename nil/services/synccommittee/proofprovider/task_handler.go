package proofprovider

import (
	"context"
	"maps"
	"slices"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskHandler struct {
	taskStorage storage.TaskStorage
	logger      zerolog.Logger
}

func newTaskHandler(taskStorage storage.TaskStorage, logger zerolog.Logger) api.TaskHandler {
	return &taskHandler{
		taskStorage: taskStorage,
		logger:      logger,
	}
}

func (h *taskHandler) Handle(ctx context.Context, _ types.TaskExecutorId, task *types.Task) error {
	if task.TaskType != types.ProofBlock {
		return types.UnexpectedTaskType(task)
	}

	h.logger.Info().Msgf("Create tasks for prove block %v from shard %d in batch %d, prove block task id %v", task.BlockHash, task.ShardId, task.BatchNum, task.Id.String())

	blockTasks := prepareTasksForBlock(task)

	for _, taskEntry := range blockTasks {
		taskEntry.Task.ParentTaskId = &task.Id
	}

	return h.taskStorage.AddTaskEntries(ctx, blockTasks)
}

var circuitTypes = [...]types.CircuitType{types.Bytecode, types.MPT, types.ReadWrite, types.ZKEVM}

func prepareTasksForBlock(providerTask *types.Task) []*types.TaskEntry {
	taskEntries := make(map[types.TaskId]*types.TaskEntry)

	// Create partial proof tasks (top level, no dependencies)
	partialProofTasks := make(map[types.CircuitType]types.TaskId)
	for _, ct := range circuitTypes {
		partialProveTaskEntry := types.NewPartialProveTaskEntry(0, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash, ct)
		taskEntries[partialProveTaskEntry.Task.Id] = partialProveTaskEntry
		partialProofTasks[ct] = partialProveTaskEntry.Task.Id
	}

	// aggregate challenge task depends on all the previous tasks
	aggChallengeTaskEntry := types.NewAggregateChallengeTaskEntry(0, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash)
	aggChallengeTaskID := aggChallengeTaskEntry.Task.Id
	taskEntries[aggChallengeTaskID] = aggChallengeTaskEntry

	// Second level of circuit-dependent tasks
	combinedQTasks := make(map[types.CircuitType]types.TaskId)
	for _, ct := range circuitTypes {
		combinedQTaskEntry := types.NewCombinedQTaskEntry(0, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash, ct)
		taskEntries[combinedQTaskEntry.Task.Id] = combinedQTaskEntry
		combinedQTasks[ct] = combinedQTaskEntry.Task.Id
	}

	// aggregate FRI task depends on all the previous tasks
	aggFRITaskEntry := types.NewAggregateFRITaskEntry(0, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash)
	aggFRITaskID := aggFRITaskEntry.Task.Id
	taskEntries[aggFRITaskID] = aggFRITaskEntry

	// Third level of circuit-dependent tasks
	consistencyCheckTasks := make(map[types.CircuitType]types.TaskId)
	for _, ct := range circuitTypes {
		taskEntry := types.NewFRIConsistencyCheckTaskEntry(0, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash, ct)
		consistencyCheckTasks[ct] = taskEntry.Task.Id
		taskEntries[taskEntry.Task.Id] = taskEntry
	}

	// Final task, depends on partial proofs, aggregate FRI and consistency checks
	mergeProofTaskEntry := types.NewMergeProofTaskEntry(0, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash)
	mergeProofTaskId := mergeProofTaskEntry.Task.Id
	taskEntries[mergeProofTaskId] = mergeProofTaskEntry

	// Set pending dependencies

	// Partial proof results go to all other levels of tasks
	for ct, id := range partialProofTasks {
		ppEntry := taskEntries[id]
		ppEntry.PendingDeps = append(ppEntry.PendingDeps,
			aggChallengeTaskID,
			combinedQTasks[ct],
			aggFRITaskID,
			consistencyCheckTasks[ct],
			mergeProofTaskId)
	}

	for ct, id := range combinedQTasks {
		combQEntry := taskEntries[id]
		// combined Q task result goes to aggregate FRI task and consistency check task
		combQEntry.PendingDeps = append(combQEntry.PendingDeps, aggFRITaskID, consistencyCheckTasks[ct])
		// aggregate challenges task result goes to all combined Q tasks
		aggChallengeTaskEntry.PendingDeps = append(aggChallengeTaskEntry.PendingDeps, id)
	}

	// One more destination of aggregate challenge task result is aggregate FRI task
	aggChallengeTaskEntry.PendingDeps = append(aggChallengeTaskEntry.PendingDeps, aggFRITaskID)

	for _, id := range consistencyCheckTasks {
		ccEntry := taskEntries[id]
		// consistency check task result goes to merge proof task
		ccEntry.PendingDeps = append(ccEntry.PendingDeps, mergeProofTaskId)
		// aggregate FRI task result goes to all consistency check tasks
		aggFRITaskEntry.PendingDeps = append(aggFRITaskEntry.PendingDeps, id)
	}

	// Also aggregate FRI task result must be forwarded to merge proof task
	aggFRITaskEntry.PendingDeps = append(aggFRITaskEntry.PendingDeps, mergeProofTaskId)
	return slices.Collect(maps.Values(taskEntries))
}
