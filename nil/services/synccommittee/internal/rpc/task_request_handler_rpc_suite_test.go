package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
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
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.Task, error) {
			predefinedTask := tasksForExecutors[request.ExecutorId]
			return predefinedTask, nil
		},
		SetTaskResultFunc: func(ctx context.Context, result *types.TaskResult) error {
			return nil
		},
	}
}

var (
	firstExecutorId        = types.TaskExecutorId(1)
	secondExecutorId       = types.TaskExecutorId(2)
	firstDependencyTaskId  = types.NewTaskId()
	secondDependencyTaskId = types.NewTaskId()
)

var tasksForExecutors = map[types.TaskExecutorId]*types.Task{
	firstExecutorId: {
		Id:            types.NewTaskId(),
		BatchNum:      1,
		ShardId:       coreTypes.MainShardId,
		BlockNum:      1,
		BlockHash:     common.EmptyHash,
		TaskType:      types.PartialProve,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.TaskId]types.TaskResult),
		DependencyNum: 0,
	},
	secondExecutorId: {
		Id:          types.NewTaskId(),
		BatchNum:    1234,
		BlockNum:    10,
		TaskType:    types.AggregatedFRI,
		CircuitType: types.ReadWrite,
		Dependencies: map[types.TaskId]types.TaskResult{
			firstDependencyTaskId: types.SuccessProverTaskResult(
				firstDependencyTaskId,
				testaide.GenerateRandomExecutorId(),
				types.MergeProof,
				types.TaskResultAddresses{},
			),
			secondDependencyTaskId: types.SuccessProverTaskResult(
				secondDependencyTaskId,
				testaide.GenerateRandomExecutorId(),
				types.PartialProve,
				types.TaskResultAddresses{},
			),
		},
		DependencyNum: 2,
	},
}
