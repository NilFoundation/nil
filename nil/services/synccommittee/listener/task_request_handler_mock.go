//go:build test

package listener

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"

	"github.com/NilFoundation/nil/nil/common/math"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type TaskEntry struct {
	Task     types.ProverTask
	Status   types.ProverTaskStatus
	ProverId *types.ProverId
	Result   *types.ProverTaskResult
}

type TaskRequestHandlerMock struct {
	mutex sync.Mutex
	tasks map[types.ProverTaskId]TaskEntry
}

func NewTaskRequestHandlerMock() *TaskRequestHandlerMock {
	return &TaskRequestHandlerMock{
		tasks: make(map[types.ProverTaskId]TaskEntry),
	}
}

func (m *TaskRequestHandlerMock) GetTaskEntry(id types.ProverTaskId) *TaskEntry {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	entry, exists := m.tasks[id]
	if exists {
		return &entry
	}
	return nil
}

func (m *TaskRequestHandlerMock) GetAllEntries() *[]TaskEntry {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entries := make([]TaskEntry, 0, len(m.tasks))
	for _, entry := range m.tasks {
		entries = append(entries, entry)
	}

	return &entries
}

func (m *TaskRequestHandlerMock) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tasks = make(map[types.ProverTaskId]TaskEntry)
}

func GenerateTask() (*types.ProverTask, error) {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		return nil, err
	}

	return &types.ProverTask{
		Id:            types.ProverTaskId(uint32(bigInt.Uint64())),
		BatchNum:      1,
		BlockNum:      1,
		TaskType:      types.Preprocess,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.ProverTaskId]types.ProverTaskResult),
		DependencyNum: 0,
	}, nil
}

func (m *TaskRequestHandlerMock) AddTask(task *types.ProverTask) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.tasks[task.Id] = TaskEntry{Task: *task, Status: types.WaitingForProver, ProverId: nil}
}

func (m *TaskRequestHandlerMock) GetTask(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for key, entry := range m.tasks {
		if entry.Status == types.WaitingForProver {
			task := entry.Task
			delete(m.tasks, key)
			m.tasks[task.Id] = TaskEntry{Status: types.Running, ProverId: &request.ProverId}
			return &task, nil
		}
	}

	return nil, nil
}

func (m *TaskRequestHandlerMock) SetTaskResult(_ context.Context, result *types.ProverTaskResult) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entry := m.tasks[result.TaskId]
	entry.Result = result
	m.tasks[result.TaskId] = entry
	return nil
}
