package prover

import (
	"context"
	"encoding/json"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/rs/zerolog"
)

type RemoteTaskObserver struct {
	client client.RawClient
}

func NewRemoteTaskObserver(syncCommitteeApiEndpoint string, logger zerolog.Logger) *RemoteTaskObserver {
	return &RemoteTaskObserver{
		client: rpc.NewRawClient(syncCommitteeApiEndpoint, logger),
	}
}

func (r *RemoteTaskObserver) GetTask(_ context.Context, request *api.TaskRequest) (*api.ProverTaskView, error) {
	response, err := r.client.RawCall(api.TaskObserverGetTask, request)
	if err != nil {
		return nil, err
	}

	var taskView api.ProverTaskView
	if err = json.Unmarshal(response, &taskView); err != nil {
		return nil, err
	}

	return &taskView, nil
}

func (r *RemoteTaskObserver) UpdateTaskStatus(_ context.Context, request *api.TaskStatusUpdateRequest) error {
	_, err := r.client.RawCall(api.TaskObserverUpdateTaskStatus, request)
	return err
}
