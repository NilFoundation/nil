//go:build test

package testaide

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func NewTaskEntry(modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId) *types.TaskEntry {
	return NewTaskEntryOfType(types.PartialProve, modifiedAt, status, owner)
}

func NewTaskEntryOfType(
	taskType types.TaskType, modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId,
) *types.TaskEntry {
	task := NewTaskOfType(taskType)

	entry := &types.TaskEntry{
		Task:    *task,
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

func NewTask() *types.Task {
	return NewTaskOfType(types.PartialProve)
}

func NewTaskOfType(taskType types.TaskType) *types.Task {
	return &types.Task{
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

func RandomTaskResultData() types.TaskResultData {
	size, err := rand.Int(rand.Reader, big.NewInt(int64(1024)))
	check.PanicIfErr(err)
	dataBytes := make([]byte, size.Int64())

	_, err = rand.Read(dataBytes)
	check.PanicIfErr(err)
	return dataBytes
}

func NewSuccessTaskResult(taskId types.TaskId, executor types.TaskExecutorId) *types.TaskResult {
	return types.NewSuccessProverTaskResult(
		taskId,
		executor,
		types.TaskResultAddresses{},
		types.TaskResultData{},
	)
}

func NewFailureTaskResult(taskId types.TaskId, executor types.TaskExecutorId) *types.TaskResult {
	return types.NewFailureProverTaskResult(
		taskId,
		executor,
		errors.New("something went wrong"),
	)
}
