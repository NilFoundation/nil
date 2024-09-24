package proofprovider

import (
	"context"
	"log"
	"maps"
	"slices"

	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type taskHandler struct {
	taskStorage storage.TaskStorage
}

func newTaskHandler(taskStorage storage.TaskStorage) executor.TaskHandler {
	return &taskHandler{
		taskStorage: taskStorage,
	}
}

func (h *taskHandler) Handle(ctx context.Context, _ types.TaskExecutorId, task *types.Task) error {
	if task.TaskType != types.ProofBlock {
		log.Panicf("received task (id=%s) with unexpected type %d", task.Id, task.TaskType)
	}

	blockTasks := prepareTasksForBlock(task.BlockNum)
	return h.taskStorage.AddTaskEntries(ctx, blockTasks)
}

var circuitTypes = [...]types.CircuitType{types.Bytecode, types.MPT, types.ReadWrite, types.ZKEVM}

func prepareTasksForBlock(blockNumber coreTypes.BlockNumber) []*types.TaskEntry {
	taskEntries := make(map[types.TaskId]*types.TaskEntry)

	// Create partial proof tasks (top level, no dependencies)
	partialProofTasks := make(map[types.CircuitType]types.TaskId)
	for _, ct := range circuitTypes {
		partialProveTaskEntry := types.NewPartialProveTaskEntry(0, blockNumber, ct)
		taskEntries[partialProveTaskEntry.Task.Id] = partialProveTaskEntry
		partialProofTasks[ct] = partialProveTaskEntry.Task.Id
	}

	// aggregate FRI task depends on all the previous tasks
	aggFRITaskEntry := types.NewAggregateFRITaskEntry(0, blockNumber)
	aggFRITaskID := aggFRITaskEntry.Task.Id
	taskEntries[aggFRITaskID] = aggFRITaskEntry

	// Second level of circuit-dependent tasks
	consistencyCheckTasks := make(map[types.CircuitType]types.TaskId)
	for _, ct := range circuitTypes {
		taskEntry := types.NewFRIConsistencyCheckTaskEntry(0, blockNumber, ct)
		consistencyCheckTasks[ct] = taskEntry.Task.Id
		taskEntries[taskEntry.Task.Id] = taskEntry
	}

	// Final task, depends on all the previous ones
	mergeProofTaskEntry := types.NewMergeProofTaskEntry(0, blockNumber)
	mergeProofTaskId := mergeProofTaskEntry.Task.Id
	taskEntries[mergeProofTaskId] = mergeProofTaskEntry

	// Set pending dependencies

	// Partial proof results go to all other levels of tasks
	for ct, id := range partialProofTasks {
		ppEntry := taskEntries[id]
		ppEntry.PendingDeps = append(ppEntry.PendingDeps, aggFRITaskID, consistencyCheckTasks[ct], mergeProofTaskId)
	}

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
