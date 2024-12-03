package rpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskRequestHandlerTestSuite struct {
	suite.Suite
	context       context.Context
	cancellation  context.CancelFunc
	clientHandler api.TaskRequestHandler
	scheduler     *scheduler.TaskSchedulerMock
}

func (s *TaskRequestHandlerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	listenerHttpEndpoint := "tcp://127.0.0.1:8530"
	s.scheduler = newTaskSchedulerMock()

	go func() {
		err := runTaskListener(s.context, listenerHttpEndpoint, s.scheduler)
		s.NoError(err)
	}()

	s.clientHandler = NewTaskRequestRpcClient(
		listenerHttpEndpoint,
		logging.NewLogger("task-request-rpc-client"),
	)
}

func (s *TaskRequestHandlerTestSuite) TearDownSubTest() {
	s.scheduler.ResetCalls()
}

func (s *TaskRequestHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func runTaskListener(ctx context.Context, httpEndpoint string, scheduler scheduler.TaskScheduler) error {
	taskListener := NewTaskListener(
		&TaskListenerConfig{HttpEndpoint: httpEndpoint},
		scheduler,
		logging.NewLogger("sync-committee-task-rpc"),
	)

	return taskListener.Run(ctx)
}

func newTaskSchedulerMock() *scheduler.TaskSchedulerMock {
	return &scheduler.TaskSchedulerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.Task, error) {
			predefinedTask := tasksForExecutors[request.ExecutorId]
			return predefinedTask, nil
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
		BatchId:       types.NewBatchId(),
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
		BatchId:     types.NewBatchId(),
		BlockNum:    10,
		TaskType:    types.AggregatedFRI,
		CircuitType: types.ReadWrite,
		Dependencies: map[types.TaskId]types.TaskResult{
			firstDependencyTaskId: types.SuccessProverTaskResult(
				firstDependencyTaskId,
				testaide.RandomExecutorId(),
				types.MergeProof,
				types.TaskResultAddresses{},
				types.TaskResultData{},
			),
			secondDependencyTaskId: types.SuccessProverTaskResult(
				secondDependencyTaskId,
				testaide.RandomExecutorId(),
				types.PartialProve,
				types.TaskResultAddresses{},
				types.TaskResultData{},
			),
		},
		DependencyNum: 2,
	},
}
