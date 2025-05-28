package public

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	DebugTasksNamespace = "DebugTasks"
	DebugGetTasks       = DebugTasksNamespace + "_getTasks"
	DebugGetTaskTree    = DebugTasksNamespace + "_getTaskTree"
)

type TaskDebugOrder int8

const (
	_ TaskDebugOrder = iota
	OrderByExecutionTime
	OrderByCreatedAt
)

var TaskDebugOrderNames = map[string]TaskDebugOrder{
	"ExecutionTime": OrderByExecutionTime,
	"CreatedAt":     OrderByCreatedAt,
}

func (o *TaskDebugOrder) Set(str string) error {
	value, ok := TaskDebugOrderNames[str]
	if !ok {
		return fmt.Errorf("unknown task order: %s", str)
	}
	*o = value
	return nil
}

func (*TaskDebugOrder) Type() string {
	return "TaskDebugOrder"
}

func (*TaskDebugOrder) PossibleValues() []string {
	return slices.Collect(maps.Keys(TaskDebugOrderNames))
}

const (
	DefaultDebugTaskStatus = types.TaskStatusNone
	DefaultDebugTaskType   = types.TaskTypeNone
	DefaultDebugTaskOwner  = types.UnknownExecutorId
	DefaultDebugTaskOrder  = OrderByCreatedAt
)

type TaskDebugRequest struct {
	ListRequest

	Status TaskStatus     `json:"status,omitempty"`
	Type   TaskType       `json:"type,omitempty"`
	Owner  TaskExecutorId `json:"owner,omitempty"`

	Order     TaskDebugOrder `json:"order"`
	Ascending bool           `json:"ascending,omitempty"`
}

func DefaultTaskDebugRequest() TaskDebugRequest {
	defaultOrder := DefaultDebugTaskOrder
	defaultLimit := DefaultLimit
	return *NewTaskDebugRequest(nil, nil, nil, &defaultOrder, false, &defaultLimit)
}

func NewTaskDebugRequest(
	status *TaskStatus,
	taskType *TaskType,
	owner *TaskExecutorId,
	order *TaskDebugOrder,
	ascending bool,
	limit *int,
) *TaskDebugRequest {
	targetStatus := DefaultDebugTaskStatus
	if status != nil {
		targetStatus = *status
	}

	targetType := DefaultDebugTaskType
	if taskType != nil {
		targetType = *taskType
	}

	targetOwner := DefaultDebugTaskOwner
	if owner != nil {
		targetOwner = *owner
	}

	targetOrder := DefaultDebugTaskOrder
	if order != nil {
		targetOrder = *order
	}

	return &TaskDebugRequest{
		ListRequest: newListRequestCommon(limit),

		Status:    targetStatus,
		Type:      targetType,
		Owner:     targetOwner,
		Order:     targetOrder,
		Ascending: ascending,
	}
}

// TaskDebugApi provides methods to retrieve debug information on tasks.
type TaskDebugApi interface {
	// GetTasks retrieves a list of tasks based on the specified TaskDebugRequest criteria.
	GetTasks(ctx context.Context, request *TaskDebugRequest) ([]*TaskView, error)

	// GetTaskTree retrieves the task tree structure for a specific task identified by taskId
	GetTaskTree(ctx context.Context, taskId TaskId) (*TaskTreeView, error)
}
