package proofprovider

import (
	"context"
	"fmt"

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
	if (task.TaskType != types.ProofBlock) && (task.TaskType != types.AggregateProofs) {
		return types.UnexpectedTaskType(task)
	}

	h.logger.Info().Msgf("Create tasks %v, blockHash %v from shard %d in batch %d, task id %v", task.TaskType.String(), task.BlockHash, task.ShardId, task.BatchId, task.Id.String())

	if task.TaskType == types.ProofBlock {
		blockTasks, err := prepareTasksForBlock(task)
		if err != nil {
			return fmt.Errorf("failed to prepare tasks for block, taskId=%s: %w", task.Id, err)
		}

		for _, taskEntry := range blockTasks {
			taskEntry.Task.ParentTaskId = &task.Id
		}

		return h.taskStorage.AddTaskEntries(ctx, blockTasks)
	} else {
		aggregateTaskEntry := task.AsNewChildEntry()
		return h.taskStorage.AddSingleTaskEntry(ctx, *aggregateTaskEntry)
	}
}

var circuitTypes = [...]types.CircuitType{types.Bytecode, types.MPT, types.ReadWrite, types.ZKEVM}

func prepareTasksForBlock(providerTask *types.Task) ([]*types.TaskEntry, error) {
	taskEntries := make([]*types.TaskEntry, 0)

	// Final task, depends on partial proofs, aggregate FRI and consistency checks
	mergeProofTaskEntry := types.NewMergeProofTaskEntry(
		providerTask.BatchId, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash,
	)
	taskEntries = append(taskEntries, mergeProofTaskEntry)

	// Third level of circuit-dependent tasks
	consistencyCheckTasks := make(map[types.CircuitType]*types.TaskEntry)
	for _, ct := range circuitTypes {
		checkTaskEntry := types.NewFRIConsistencyCheckTaskEntry(
			providerTask.BatchId, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash, ct,
		)
		taskEntries = append(taskEntries, checkTaskEntry)
		consistencyCheckTasks[ct] = checkTaskEntry

		// FRI consistency check task result goes to merge proof task
		if err := mergeProofTaskEntry.AddDependency(checkTaskEntry); err != nil {
			return nil, err
		}
	}

	// aggregate FRI task depends on all the following tasks
	aggFRITaskEntry := types.NewAggregateFRITaskEntry(
		providerTask.BatchId, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash,
	)
	taskEntries = append(taskEntries, aggFRITaskEntry)
	// Aggregate FRI task result must be forwarded to merge proof task
	if err := mergeProofTaskEntry.AddDependency(aggFRITaskEntry); err != nil {
		return nil, err
	}

	for _, checkTaskEntry := range consistencyCheckTasks {
		// Also aggregate FRI task result goes to all consistency check tasks
		if err := checkTaskEntry.AddDependency(aggFRITaskEntry); err != nil {
			return nil, err
		}
	}

	// Second level of circuit-dependent tasks
	combinedQTasks := make(map[types.CircuitType]*types.TaskEntry)
	for _, ct := range circuitTypes {
		combinedQTaskEntry := types.NewCombinedQTaskEntry(
			providerTask.BatchId, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash, ct,
		)
		taskEntries = append(taskEntries, combinedQTaskEntry)
		combinedQTasks[ct] = combinedQTaskEntry
	}

	for ct, combQEntry := range combinedQTasks {
		// Combined Q task result goes to aggregate FRI task and consistency check task
		if err := aggFRITaskEntry.AddDependency(combQEntry); err != nil {
			return nil, err
		}
		if err := consistencyCheckTasks[ct].AddDependency(combQEntry); err != nil {
			return nil, err
		}
	}

	// aggregate challenge task depends on all the following tasks
	aggChallengeTaskEntry := types.NewAggregateChallengeTaskEntry(
		providerTask.BatchId, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash,
	)
	taskEntries = append(taskEntries, aggChallengeTaskEntry)

	// aggregate challenges task result goes to all combined Q tasks
	for _, combQEntry := range combinedQTasks {
		if err := combQEntry.AddDependency(aggChallengeTaskEntry); err != nil {
			return nil, err
		}
	}

	// One more destination of aggregate challenge task result is aggregate FRI task
	if err := aggFRITaskEntry.AddDependency(aggChallengeTaskEntry); err != nil {
		return nil, err
	}

	// Create partial proof tasks (bottom level, no dependencies)
	for _, ct := range circuitTypes {
		partialProveTaskEntry := types.NewPartialProveTaskEntry(
			providerTask.BatchId, providerTask.ShardId, providerTask.BlockNum, providerTask.BlockHash, ct,
		)
		taskEntries = append(taskEntries, partialProveTaskEntry)

		// Partial proof results go to all other levels of tasks
		if err := aggChallengeTaskEntry.AddDependency(partialProveTaskEntry); err != nil {
			return nil, err
		}
		if err := combinedQTasks[ct].AddDependency(partialProveTaskEntry); err != nil {
			return nil, err
		}
		if err := aggFRITaskEntry.AddDependency(partialProveTaskEntry); err != nil {
			return nil, err
		}
		if err := consistencyCheckTasks[ct].AddDependency(partialProveTaskEntry); err != nil {
			return nil, err
		}
		if err := mergeProofTaskEntry.AddDependency(partialProveTaskEntry); err != nil {
			return nil, err
		}
	}

	return taskEntries, nil
}
