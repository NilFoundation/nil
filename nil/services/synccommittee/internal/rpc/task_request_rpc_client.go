package rpc

import (
	"context"
	"encoding/json"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type TaskRequestRpcClient struct {
	client client.RawClient
}

func NewTaskRequestRpcClient(syncCommitteeApiEndpoint string, logger zerolog.Logger) *TaskRequestRpcClient {
	return &TaskRequestRpcClient{
		client: rpc.NewRawClient(syncCommitteeApiEndpoint, logger),
	}
}

func (r *TaskRequestRpcClient) GetTask(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
	response, err := r.client.RawCall(api.TaskRequestHandlerGetTask, request)
	if err != nil {
		return nil, err
	}

	var proverTask *types.ProverTask
	if err = json.Unmarshal(response, &proverTask); err != nil {
		return nil, err
	}

	return proverTask, nil
}

func (r *TaskRequestRpcClient) SetTaskResult(_ context.Context, result *types.ProverTaskResult) error {
	_, err := r.client.RawCall(api.TaskRequestHandlerSetTaskResult, result)
	return err
}
