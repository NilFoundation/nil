package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskRequestHandlerTestSuite struct {
	suite.Suite
	context       context.Context
	cancellation  context.CancelFunc
	clientHandler api.TaskRequestHandler
	targetHandler *api.TaskRequestHandlerMock
}

func (s *TaskRequestHandlerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	listenerHttpEndpoint := "tcp://127.0.0.1:8530"
	s.targetHandler = newTaskRequestHandlerMock()

	go func() {
		err := runTaskListener(s.context, listenerHttpEndpoint, s.targetHandler)
		s.NoError(err)
	}()

	s.clientHandler = NewTaskRequestRpcClient(
		listenerHttpEndpoint,
		logging.NewLogger("task-request-rpc-client"),
	)
}

func (s *TaskRequestHandlerTestSuite) TearDownSubTest() {
	s.targetHandler.ResetCalls()
}

func (s *TaskRequestHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func runTaskListener(ctx context.Context, httpEndpoint string, handler api.TaskRequestHandler) error {
	taskListener := NewTaskListener(
		&TaskListenerConfig{HttpEndpoint: httpEndpoint},
		handler,
		logging.NewLogger("sync-committee-task-rpc"),
	)

	return taskListener.Run(ctx)
}

func newTaskRequestHandlerMock() *api.TaskRequestHandlerMock {
	return &api.TaskRequestHandlerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
			predefinedTask := tasksForProvers[request.ExecutorId]
			return predefinedTask, nil
		},
		SetTaskResultFunc: func(ctx context.Context, result *types.TaskResult) error {
			return nil
		},
	}
}

var (
	firstProverId          = types.TaskExecutorId(1)
	secondProverId         = types.TaskExecutorId(2)
	firstDependencyTaskId  = types.NewProverTaskId()
	secondDependencyTaskId = types.NewProverTaskId()
)

var tasksForProvers = map[types.TaskExecutorId]*types.ProverTask{
	firstProverId: {
		Id:            types.NewProverTaskId(),
		BatchNum:      1,
		BlockNum:      1,
		TaskType:      types.Preprocess,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.ProverTaskId]types.TaskResult),
		DependencyNum: 0,
	},
	secondProverId: {
		Id:          types.NewProverTaskId(),
		BatchNum:    1234,
		BlockNum:    10,
		TaskType:    types.AggregatedFRI,
		CircuitType: types.ReadWrite,
		Dependencies: map[types.ProverTaskId]types.TaskResult{
			firstDependencyTaskId: types.SuccessTaskResult(
				firstDependencyTaskId,
				testaide.GenerateRandomProverId(),
				types.FinalProof,
				"2B3C4D5E",
			),
			secondDependencyTaskId: types.SuccessTaskResult(
				secondDependencyTaskId,
				testaide.GenerateRandomProverId(),
				types.Commitment,
				"3C4D5E6F",
			),
		},
		DependencyNum: 2,
	},
}
