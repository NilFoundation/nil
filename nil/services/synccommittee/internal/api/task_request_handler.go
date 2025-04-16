package api

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	TaskRequestHandlerNamespace         = "TaskRequestHandler"
	TaskRequestHandlerGetTask           = TaskRequestHandlerNamespace + "_getTask"
	TaskRequestHandlerCheckIfTaskExists = TaskRequestHandlerNamespace + "_checkIfTaskExists"
	TaskRequestHandlerSetTaskResult     = TaskRequestHandlerNamespace + "_setTaskResult"
)

type TaskRequest struct {
	ExecutorId types.TaskExecutorId `json:"executorId"`
}

func NewTaskRequest(executorId types.TaskExecutorId) *TaskRequest {
	return &TaskRequest{ExecutorId: executorId}
}

type TaskCheckRequest struct {
	TaskId     types.TaskId         `json:"taskId"`
	ExecutorId types.TaskExecutorId `json:"executorId"`
}

func NewTaskCheckRequest(taskId types.TaskId, executorId types.TaskExecutorId) *TaskCheckRequest {
	return &TaskCheckRequest{
		TaskId:     taskId,
		ExecutorId: executorId,
	}
}

type TaskRequestHandler interface {
	GetTask(context context.Context, request *TaskRequest) (*types.Task, error)
	CheckIfTaskExists(context context.Context, request *TaskCheckRequest) (bool, error)
	SetTaskResult(context context.Context, result *types.TaskResult) error
}

//go:generate bash ../scripts/generate_mock.sh TaskRequestHandler
