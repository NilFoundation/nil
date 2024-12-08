package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type TaskDebugOrder int8

const (
	DebugNamespace   = "Debug"
	DebugGetTasks    = DebugNamespace + "_getTasks"
	DebugGetTaskTree = DebugNamespace + "_getTaskTree"
)

const (
	_ TaskDebugOrder = iota
	OrderByExecutionTime
	OrderByCreatedAt
)

const (
	DefaultDebugTaskOrder = OrderByCreatedAt
	DefaultDebugTaskLimit = 20
)

func StringToTaskOrder(s string) (TaskDebugOrder, error) {
	switch strings.ToLower(s) {
	case "executiontime":
		return OrderByExecutionTime, nil
	case "createdat":
		return OrderByCreatedAt, nil
	default:
		return 0, fmt.Errorf("unknown task order: %s", s)
	}
}

type TaskDebugRequest struct {
	Status   *types.TaskStatus     `json:"status,omitempty"`
	Type     *types.TaskType       `json:"type,omitempty"`
	Executor *types.TaskExecutorId `json:"executor,omitempty"`

	Order     TaskDebugOrder `json:"order"`
	Ascending bool           `json:"ascending"`
	Limit     int            `json:"limit"`
}

func NewTaskDebugRequest(
	status *types.TaskStatus,
	taskType *types.TaskType,
	executor *types.TaskExecutorId,
	order *TaskDebugOrder,
	ascending bool,
	limit *int,
) *TaskDebugRequest {
	targetOrder := DefaultDebugTaskOrder
	if order != nil {
		targetOrder = *order
	}
	targetLimit := DefaultDebugTaskLimit
	if limit != nil {
		targetLimit = *limit
	}

	return &TaskDebugRequest{
		Status:    status,
		Type:      taskType,
		Executor:  executor,
		Order:     targetOrder,
		Ascending: ascending,
		Limit:     targetLimit,
	}
}

// TaskDebugApi provides methods to retrieve debug information on tasks.
type TaskDebugApi interface {
	// GetTasks retrieves a list of tasks based on the specified TaskDebugRequest criteria.
	GetTasks(ctx context.Context, request *TaskDebugRequest) ([]*types.TaskEntry, error)

	// GetTaskTree retrieves the task tree structure for a specific task identified by taskId
	GetTaskTree(ctx context.Context, taskId types.TaskId) (*types.TaskTree, error)
}
