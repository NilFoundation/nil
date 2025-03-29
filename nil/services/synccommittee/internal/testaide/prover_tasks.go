//go:build test

package testaide

import (
	"crypto/rand"
	"math"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func NewPendingTaskEntries(modifiedAt time.Time, count int) []*types.TaskEntry {
	tasks := make([]*types.TaskEntry, 0, count)
	for range count {
		task := NewTaskEntry(modifiedAt, types.WaitingForExecutor, types.UnknownExecutorId)
		tasks = append(tasks, task)
	}
	return tasks
}

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
		Id:       types.NewTaskId(),
		BatchId:  types.NewBatchId(),
		TaskType: taskType,
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
	randVal, err := rand.Int(rand.Reader, big.NewInt(int64(1024)))
	check.PanicIfErr(err)
	size := randVal.Int64() + 1
	dataBytes := make([]byte, size)

	_, err = rand.Read(dataBytes)
	check.PanicIfErr(err)
	return dataBytes
}

func NewSuccessTaskResult(taskId types.TaskId, executor types.TaskExecutorId) *types.TaskResult {
	return types.NewSuccessProverTaskResult(
		taskId,
		executor,
		types.TaskOutputArtifacts{},
		types.TaskResultData{},
	)
}

func NewRetryableErrorTaskResult(taskId types.TaskId, executor types.TaskExecutorId) *types.TaskResult {
	return types.NewFailureProverTaskResult(
		taskId,
		executor,
		types.NewTaskExecError(types.TaskErrUnknown, "something went wrong"),
	)
}

func NewNonRetryableErrorTaskResult(taskId types.TaskId, executor types.TaskExecutorId) *types.TaskResult {
	return types.NewFailureProverTaskResult(
		taskId,
		executor,
		types.NewTaskExecError(types.TaskErrProofGenerationFailed, "failed to proof block"),
	)
}
