//go:build test

package listener

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"

	"github.com/NilFoundation/nil/nil/common/math"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
)

type TaskEntry struct {
	Task     api.ProverTaskView
	Status   api.ProverTaskStatus
	ProverId *api.ProverNonceId
}

type MockTaskObserver struct {
	mutex sync.Mutex
	tasks map[api.ProverTaskId]TaskEntry
}

func NewMockTaskObserver() *MockTaskObserver {
	return &MockTaskObserver{
		tasks: make(map[api.ProverTaskId]TaskEntry),
	}
}

func (m *MockTaskObserver) GetTaskEntry(id api.ProverTaskId) *TaskEntry {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	entry, exists := m.tasks[id]
	if exists {
		return &entry
	}
	return nil
}

func (m *MockTaskObserver) GetAllEntries() *[]TaskEntry {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entries := make([]TaskEntry, 0, len(m.tasks))
	for _, entry := range m.tasks {
		entries = append(entries, entry)
	}

	return &entries
}

func (m *MockTaskObserver) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tasks = make(map[api.ProverTaskId]TaskEntry)
}

func GenerateTask() (*api.ProverTaskView, error) {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		return nil, err
	}

	return &api.ProverTaskView{
		Id: api.ProverTaskId(uint32(bigInt.Uint64())),
	}, nil
}

func (m *MockTaskObserver) AddTask(task *api.ProverTaskView) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.tasks[task.Id] = TaskEntry{Task: *task, Status: api.WaitingForProver, ProverId: nil}
}

func (m *MockTaskObserver) GetTask(_ context.Context, request *api.TaskRequest) (*api.ProverTaskView, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for key, entry := range m.tasks {
		if entry.Status == api.WaitingForProver {
			task := entry.Task
			delete(m.tasks, key)
			m.tasks[task.Id] = TaskEntry{Status: api.Running, ProverId: &request.ProverId}
			return &task, nil
		}
	}

	return nil, nil
}

func (m *MockTaskObserver) UpdateTaskStatus(_ context.Context, request *api.TaskStatusUpdateRequest) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entry := m.tasks[request.TaskId]
	entry.Status = request.NewStatus
	m.tasks[request.TaskId] = entry
	return nil
}
