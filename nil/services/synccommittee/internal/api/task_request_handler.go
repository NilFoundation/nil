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
	ProverId types.ProverId `json:"proverId"`
}

func NewTaskRequest(proverId types.ProverId) *TaskRequest {
	return &TaskRequest{ProverId: proverId}
}

type TaskRequestHandler interface {
	GetTask(context context.Context, request *TaskRequest) (*types.ProverTask, error)
	SetTaskResult(context context.Context, result *types.ProverTaskResult) error
}
