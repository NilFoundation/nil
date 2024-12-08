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
	ExecutionTime
	CreatedAt
)

func StringToTaskOrder(s string) (TaskDebugOrder, error) {
	switch strings.ToLower(s) {
	case "executiontime":
		return ExecutionTime, nil
	case "createdat":
		return CreatedAt, nil
	default:
		return 0, fmt.Errorf("unknown task order: %s", s)
	}
}

type TaskDebugSort struct {
	Field     string `json:"field"`
	Ascending bool   `json:"ascending"`
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
	targetOrder := CreatedAt
	if order != nil {
		targetOrder = *order
	}
	targetLimit := 20
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

type TaskDebugApi interface {
	GetTasks(ctx context.Context, request *TaskDebugRequest) ([]*types.TaskEntry, error)
	GetTaskTree(ctx context.Context, taskId types.TaskId) (*types.TaskTree, error)
}
