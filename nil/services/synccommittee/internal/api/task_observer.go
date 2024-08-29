package api

import (
	"context"
)

const (
	TaskObserverNamespace        = "TaskObserver"
	TaskObserverGetTask          = TaskObserverNamespace + "_getTask"
	TaskObserverUpdateTaskStatus = TaskObserverNamespace + "_updateTaskStatus"
)

// todo: Status and task types should be replaced with types from PR #788

type ProverTaskStatus uint8

const (
	_ ProverTaskStatus = iota
	WaitingForProver
	Running
	Failed
	Done
)

type (
	ProverNonceId uint32
	ProverTaskId  uint32
)

type ProverTaskView struct {
	Id ProverTaskId `json:"id"`
	// other fields
}

type TaskRequest struct {
	ProverId ProverNonceId `json:"proverId"`
}

func NewTaskRequest(proverId ProverNonceId) *TaskRequest {
	return &TaskRequest{ProverId: proverId}
}

type TaskStatusUpdateRequest struct {
	ProverId  ProverNonceId    `json:"proverId"`
	TaskId    ProverTaskId     `json:"taskId"`
	NewStatus ProverTaskStatus `json:"newStatus"`
}

func NewTaskStatusUpdateRequest(
	proverId ProverNonceId,
	taskId ProverTaskId,
	newStatus ProverTaskStatus,
) *TaskStatusUpdateRequest {
	return &TaskStatusUpdateRequest{ProverId: proverId, TaskId: taskId, NewStatus: newStatus}
}

type TaskObserver interface {
	GetTask(context context.Context, request *TaskRequest) (*ProverTaskView, error)
	UpdateTaskStatus(context context.Context, request *TaskStatusUpdateRequest) error
}
