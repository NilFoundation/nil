//go:build test

package testaide

import (
	"crypto/rand"
	"math"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func GenerateTaskEntry(modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId) *types.TaskEntry {
	return &types.TaskEntry{
		Task:    GenerateTask(),
		Created: modifiedAt.Add(-1 * time.Hour),
		Status:  status,
		Owner:   owner,
	}
}

func GenerateTask() types.Task {
	return GenerateTaskOfType(types.PartialProve)
}

func GenerateTaskOfType(taskType types.TaskType) types.Task {
	return types.Task{
		Id:            types.NewTaskId(),
		BatchId:       types.NewBatchId(),
		ShardId:       coreTypes.MainShardId,
		BlockNum:      1,
		BlockHash:     common.EmptyHash,
		TaskType:      taskType,
		CircuitType:   types.Bytecode,
		Dependencies:  types.EmptyDependencies(),
		DependencyNum: 0,
	}
}

func RandomExecutorId() types.TaskExecutorId {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		panic(err)
	}
	return types.TaskExecutorId(uint32(bigInt.Uint64()))
}
