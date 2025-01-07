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

type TaskViewCommon struct {
	Id          TaskId      `json:"id"`
	Type        TaskType    `json:"type"`
	CircuitType CircuitType `json:"circuitType"`

	ExecutionTime *time.Duration `json:"executionTime,omitempty"`
	Owner         TaskExecutorId `json:"owner"`
	Status        TaskStatus     `json:"status"`
}

func (t *TaskViewCommon) IsFailed() bool {
	return t.Status == types.Failed
}

func makeTaskViewCommon(taskEntry *types.TaskEntry, currentTime time.Time) TaskViewCommon {
	return TaskViewCommon{
		Id:          taskEntry.Task.Id,
		Type:        taskEntry.Task.TaskType,
		CircuitType: taskEntry.Task.CircuitType,

		ExecutionTime: taskEntry.ExecutionTime(currentTime),
		Owner:         taskEntry.Owner,
		Status:        taskEntry.Status,
	}
}

type TaskView struct {
	TaskViewCommon

	BatchId     BatchId     `json:"batchId"`
	ShardId     ShardId     `json:"shardId"`
	BlockNumber BlockNumber `json:"blockNumber"`
	BlockHash   common.Hash `json:"blockHash"`

	CreatedAt time.Time  `json:"createdAt"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

func NewTaskView(taskEntry *types.TaskEntry, currentTime time.Time) *TaskView {
	return &TaskView{
		TaskViewCommon: makeTaskViewCommon(taskEntry, currentTime),

		BatchId:     taskEntry.Task.BatchId,
		ShardId:     taskEntry.Task.ShardId,
		BlockNumber: taskEntry.Task.BlockNum,
		BlockHash:   taskEntry.Task.BlockHash,

		CreatedAt: taskEntry.Created,
		StartedAt: taskEntry.Started,
	}
}

// TreeViewDepthLimit defines the maximum depth that for a TaskTreeView object.
const TreeViewDepthLimit = 50

func TreeDepthExceededErr(taskId TaskId) error {
	return fmt.Errorf("task tree depth limit exceeded (%d) for task with id=%s", TreeViewDepthLimit, taskId)
}

// TaskTreeView represents a full hierarchical structure of tasks with dependencies among them.
type TaskTreeView struct {
	TaskViewCommon
	ResultErrorText string                   `json:"errorText,omitempty"`
	Dependencies    map[TaskId]*TaskTreeView `json:"dependencies"`
}

func NewTaskTreeFromEntry(taskEntry *types.TaskEntry, currentTime time.Time) *TaskTreeView {
	return &TaskTreeView{
		TaskViewCommon: makeTaskViewCommon(taskEntry, currentTime),
		Dependencies:   emptyDependencies(),
	}
}

func NewTaskTreeFromResult(result *types.TaskResultDetails) *TaskTreeView {
	var taskStatus TaskStatus
	if result.IsSuccess {
		taskStatus = types.Completed
	} else {
		taskStatus = types.Failed
	}

	return &TaskTreeView{
		TaskViewCommon: TaskViewCommon{
			Id:          result.TaskId,
			Type:        result.TaskType,
			CircuitType: result.CircuitType,

			ExecutionTime: &result.ExecutionTime,
			Owner:         result.Sender,
			Status:        taskStatus,
		},
		ResultErrorText: result.ErrorText,
		Dependencies:    emptyDependencies(),
	}
}

func (t *TaskTreeView) AddDependency(dependency *TaskTreeView) {
	t.Dependencies[dependency.Id] = dependency
}

func emptyDependencies() map[TaskId]*TaskTreeView {
	return make(map[TaskId]*TaskTreeView)
}
