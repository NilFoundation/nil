package rpc

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/rpc"
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
	s.scheduler = newTaskSchedulerMock()

	started := make(chan struct{})
	listenerHttpEndpoint := rpc.GetSockPath(s.T())
	go func() {
		err := runTaskListener(s.context, s.scheduler, listenerHttpEndpoint, started)
		s.NoError(err)
	}()
	err := testaide.WaitFor(s.context, started, 10*time.Second)
	s.Require().NoError(err, "task listener did not start in time")

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

func runTaskListener(
	ctx context.Context,
	scheduler scheduler.TaskScheduler,
	listenerHttpEndpoint string,
	started chan<- struct{},
) error {
	taskListener := NewTaskListener(
		&TaskListenerConfig{HttpEndpoint: listenerHttpEndpoint},
		scheduler,
		logging.NewLogger("sync-committee-task-rpc"),
	)

	return taskListener.Run(ctx, started)
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
	firstExecutorId  = types.TaskExecutorId(1)
	secondExecutorId = types.TaskExecutorId(2)

	firstDepResult = types.NewSuccessProverTaskResult(
		types.NewTaskId(),
		testaide.RandomExecutorId(),
		nil,
		nil,
	)

	secondDepResult = types.NewSuccessProverTaskResult(
		types.NewTaskId(),
		testaide.RandomExecutorId(),
		nil,
		nil,
	)
)

var tasksForExecutors = map[types.TaskExecutorId]*types.Task{
	firstExecutorId: {
		Id:          types.NewTaskId(),
		BatchId:     types.NewBatchId(),
		TaskType:    types.PartialProve,
		CircuitType: types.CircuitBytecode,
	},
	secondExecutorId: {
		Id:          types.NewTaskId(),
		BatchId:     types.NewBatchId(),
		TaskType:    types.AggregatedFRI,
		CircuitType: types.CircuitReadWrite,
		DependencyResults: map[types.TaskId]types.TaskResultDetails{
			firstDepResult.TaskId:  {TaskResult: *firstDepResult},
			secondDepResult.TaskId: {TaskResult: *secondDepResult},
		},
	},
}
