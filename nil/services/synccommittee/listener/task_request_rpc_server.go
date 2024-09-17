package listener

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type TaskRequestRpcServer struct {
	logger zerolog.Logger
}

func (o TaskRequestRpcServer) GetTask(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
	o.logger.Debug().Msgf("received task request from prover node id=%d", request.ProverId)
	// TODO: will be implemented in https://www.notion.so/nilfoundation/Task-scheduler-4c3db841d14d474f9dddf42a79cdfda0?pvs=4
	return &types.ProverTask{
		Id:            types.NewProverTaskId(),
		BatchNum:      1,
		BlockNum:      1,
		TaskType:      types.Preprocess,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.ProverTaskId]types.ProverTaskResult),
		DependencyNum: 0,
	}, nil
}

func (o TaskRequestRpcServer) SetTaskResult(_ context.Context, result *types.ProverTaskResult) error {
	o.logger.Debug().Msgf("status of task with id %d is updated to %d", result.TaskId, result.Type)
	// TODO: will be implemented in https://www.notion.so/nilfoundation/Task-scheduler-4c3db841d14d474f9dddf42a79cdfda0?pvs=4
	return nil
}

func NewTaskRequestRpcServer() api.TaskRequestHandler {
	return &TaskRequestRpcServer{}
}
