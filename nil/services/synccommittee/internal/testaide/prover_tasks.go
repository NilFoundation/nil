//go:build test

package testaide

import (
	"crypto/rand"
	"math"
	"math/big"
	"time"

	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func GenerateTaskEntry(modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId) *types.TaskEntry {
	entry := &types.TaskEntry{
		Task:    GenerateTask(),
		Created: modifiedAt.Add(-1 * time.Hour),
		Status:  status,
		Owner:   owner,
	}

	if status == types.Running {
		entry.Started = &modifiedAt
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
