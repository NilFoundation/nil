//go:build test

package testaide

import (
	"crypto/rand"
	"math"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func GenerateTaskEntry(modifiedAt time.Time, status types.TaskStatus, owner types.TaskExecutorId) *types.TaskEntry {
	return &types.TaskEntry{
		Task:     GenerateTask(),
		Created:  modifiedAt.Add(-1 * time.Hour),
		Modified: modifiedAt,
		Status:   status,
		Owner:    owner,
	}
}

func GenerateTask() types.Task {
	return types.Task{
		Id:            types.NewTaskId(),
		BatchNum:      1,
		BlockNum:      1,
		TaskType:      types.Preprocess,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.TaskId]types.TaskResult),
		DependencyNum: 0,
	}
}

func GenerateRandomExecutorId() types.TaskExecutorId {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		panic(err)
	}
	return types.TaskExecutorId(uint32(bigInt.Uint64()))
}
