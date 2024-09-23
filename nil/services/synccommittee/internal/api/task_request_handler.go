package api

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	TaskRequestHandlerNamespace     = "TaskRequestHandler"
	TaskRequestHandlerGetTask       = TaskRequestHandlerNamespace + "_getTask"
	TaskRequestHandlerSetTaskResult = TaskRequestHandlerNamespace + "_setTaskResult"
)

type TaskRequest struct {
	ExecutorId types.TaskExecutorId `json:"proverId"`
}

func NewTaskRequest(executorId types.TaskExecutorId) *TaskRequest {
	return &TaskRequest{ExecutorId: executorId}
}

type TaskRequestHandler interface {
	GetTask(context context.Context, request *TaskRequest) (*types.ProverTask, error)
	SetTaskResult(context context.Context, result *types.TaskResult) error
}
