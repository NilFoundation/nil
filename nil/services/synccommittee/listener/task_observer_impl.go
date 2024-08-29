package listener

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/rs/zerolog"
)

type TaskObserverImpl struct {
	logger zerolog.Logger
}

func (o TaskObserverImpl) GetTask(_ context.Context, request *api.TaskRequest) (*api.ProverTaskView, error) {
	o.logger.Debug().Msgf("received task request from prover node id=%d", request.ProverId)
	return &api.ProverTaskView{Id: 0}, nil
}

func (o TaskObserverImpl) UpdateTaskStatus(_ context.Context, request *api.TaskStatusUpdateRequest) error {
	o.logger.Debug().Msgf("status of task with id %d is updated to %d", request.TaskId, request.NewStatus)
	return nil
}

func NewTaskObserver() api.TaskObserver {
	return &TaskObserverImpl{}
}
