//go:build test

package testaide

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"time"

	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func GenerateTaskEntry(modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId) *types.TaskEntry {
	return GenerateTaskEntryOfType(types.PartialProve, modifiedAt, status, owner)
}

func GenerateTaskEntryOfType(
	taskType types.TaskType, modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId,
) *types.TaskEntry {
	entry := &types.TaskEntry{
		Task:    GenerateTaskOfType(taskType),
		Created: modifiedAt.Add(-1 * time.Hour),
		Status:  status,
		Owner:   owner,
	}

	if status == types.Running {
		entry.Started = &modifiedAt
	}
	if status == types.Failed {
		started := modifiedAt.Add(-10 * time.Minute)
		entry.Started = &started
		entry.Finished = &modifiedAt
	}

	return entry
}

func GenerateTask() types.Task {
	return GenerateTaskOfType(types.PartialProve)
}

func GenerateTaskOfType(taskType types.TaskType) types.Task {
	return types.Task{
		Id:        types.NewTaskId(),
		BatchId:   types.NewBatchId(),
		ShardId:   coreTypes.MainShardId,
		BlockNum:  1,
		BlockHash: RandomHash(),
		TaskType:  taskType,
	}
}

func RandomExecutorId() types.TaskExecutorId {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		panic(err)
	}
	return types.TaskExecutorId(uint32(bigInt.Uint64()))
}

func SuccessTaskResult(taskId types.TaskId, executor types.TaskExecutorId) types.TaskResult {
	return types.SuccessProverTaskResult(
		taskId,
		executor,
		types.TaskResultAddresses{},
		types.TaskResultData{},
	)
}

func FailureTaskResult(taskId types.TaskId, executor types.TaskExecutorId) types.TaskResult {
	return types.FailureProverTaskResult(
		taskId,
		executor,
		errors.New("something went wrong"),
	)
}
