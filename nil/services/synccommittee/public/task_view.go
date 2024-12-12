package public

import (
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type (
	ShardId     = coreTypes.ShardId
	BlockNumber = coreTypes.BlockNumber

	BatchId = types.BatchId
	TaskId  = types.TaskId

	CircuitType    = types.CircuitType
	TaskType       = types.TaskType
	TaskStatus     = types.TaskStatus
	TaskExecutorId = types.TaskExecutorId
)

type TaskView struct {
	Id          TaskId      `json:"id"`
	BatchId     BatchId     `json:"batchId"`
	ShardId     ShardId     `json:"shardId"`
	BlockNumber BlockNumber `json:"blockNumber"`
	BlockHash   common.Hash `json:"blockHash"`
	Type        TaskType    `json:"type"`
	CircuitType CircuitType `json:"circuitType"`

	CreatedAt     time.Time      `json:"createdAt"`
	StartedAt     *time.Time     `json:"startedAt,omitempty"`
	ExecutionTime *time.Duration `json:"executionTime,omitempty"`
	Owner         TaskExecutorId `json:"owner"`
	Status        TaskStatus     `json:"status"`
}

func NewTaskView(taskEntry *types.TaskEntry, currentTime time.Time) *TaskView {
	return &TaskView{
		Id:          taskEntry.Task.Id,
		BatchId:     taskEntry.Task.BatchId,
		ShardId:     taskEntry.Task.ShardId,
		BlockNumber: taskEntry.Task.BlockNum,
		BlockHash:   taskEntry.Task.BlockHash,
		Type:        taskEntry.Task.TaskType,
		CircuitType: taskEntry.Task.CircuitType,

		CreatedAt:     taskEntry.Created,
		StartedAt:     taskEntry.Started,
		ExecutionTime: taskEntry.ExecutionTime(currentTime),
		Owner:         taskEntry.Owner,
		Status:        taskEntry.Status,
	}
}

type TaskResultView struct {
	TaskId    TaskId         `json:"taskId"`
	IsSuccess bool           `json:"isSuccess"`
	ErrorText string         `json:"errorText,omitempty"`
	Sender    TaskExecutorId `json:"sender"`
}

func NewTaskResultView(taskResult *types.TaskResult) *TaskResultView {
	return &TaskResultView{
		TaskId:    taskResult.TaskId,
		IsSuccess: taskResult.IsSuccess,
		ErrorText: taskResult.ErrorText,
		Sender:    taskResult.Sender,
	}
}

// TreeViewDepthLimit defines the maximum depth that for a TaskTreeView object.
const TreeViewDepthLimit = 50

func TreeDepthExceededErr(taskId TaskId) error {
	return fmt.Errorf("task tree depth limit exceeded (%d) for task with id=%s", TreeViewDepthLimit, taskId)
}

// TaskTreeView represents a full hierarchical structure of tasks with dependencies among them.
type TaskTreeView struct {
	Task         TaskView                 `json:"task"`
	Result       *TaskResultView          `json:"taskResult,omitempty"`
	Dependencies map[TaskId]*TaskTreeView `json:"dependencies"`
}

func NewTaskTree(task *TaskView) *TaskTreeView {
	return &TaskTreeView{
		Task:         *task,
		Result:       nil,
		Dependencies: make(map[TaskId]*TaskTreeView),
	}
}

func (t *TaskTreeView) AddDependency(dependency *TaskTreeView) {
	t.Dependencies[dependency.Task.Id] = dependency
}
