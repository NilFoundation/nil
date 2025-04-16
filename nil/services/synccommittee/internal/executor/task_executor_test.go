package executor

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	context        context.Context
	cancellation   context.CancelFunc
	taskExecutor   TaskExecutor
	requestHandler *api.TaskRequestHandlerMock
	taskHandler    *api.TaskHandlerMock
}

func (s *TestSuite) SetupTest() {
	s.context, s.cancellation = context.WithCancel(s.T().Context())
	s.requestHandler = newTaskRequestHandlerMock()
	s.taskHandler = &api.TaskHandlerMock{
		IsReadyToHandleFunc: func(ctx context.Context) (bool, error) { return true, nil },
	}

	config := Config{
		TaskPollingInterval: 10 * time.Millisecond,
	}
	logger := logging.NewLogger("task-executor-test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	taskExecutor, err := New(&config, s.requestHandler, s.taskHandler, metricsHandler, logger)
	s.Require().NoError(err)
	s.taskExecutor = taskExecutor
}

func (s *TestSuite) TearDownTest() {
	s.cancellation()
}

func newTaskRequestHandlerMock() *api.TaskRequestHandlerMock {
	return &api.TaskRequestHandlerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.Task, error) {
			return testaide.NewTask(), nil
		},
		SetTaskResultFunc: func(_ context.Context, result *types.TaskResult) error {
			return nil
		},
	}
}

func TestTaskExecutorSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) Test_TaskExecutor_Executes_Tasks() {
	taskHandler := s.taskHandler
	started, cancelFn := s.runTaskExecutor(s.context)
	defer cancelFn()
	err := testaide.WaitFor(s.context, started, 10*time.Second)
	s.Require().NoError(err, "task executor did not start in time")

	expectedTaskRequest := api.NewTaskRequest(s.taskExecutor.Id())
	const tasksThreshold = 5

	s.Require().Eventually(
		func() bool {
			getTaskCalls := s.requestHandler.GetTaskCalls()
			return len(getTaskCalls) >= tasksThreshold
		},
		time.Second,
		10*time.Millisecond,
	)

	for _, call := range s.requestHandler.GetTaskCalls() {
		s.Require().Equal(expectedTaskRequest, call.Request,
			"Task executor should have passed its id to the target handler")
	}

	s.Require().Eventually(
		func() bool {
			taskHandleCalls := taskHandler.HandleCalls()
			return len(taskHandleCalls) >= tasksThreshold
		},
		time.Second,
		10*time.Millisecond,
	)

	for _, call := range taskHandler.HandleCalls() {
		s.Require().Equal(s.taskExecutor.Id(), call.ExecutorId,
			"Task executor should have passed its id in the result")
	}
}

func (s *TestSuite) runTaskExecutor(ctx context.Context) (chan struct{}, func()) {
	s.T().Helper()

	ctx, cancelFunc := context.WithCancel(ctx)
	started := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		_ = s.taskExecutor.Run(ctx, started)
		stopped <- struct{}{}
	}()

	return started, func() {
		cancelFunc()
		<-stopped
	}
}

func (s *TestSuite) Test_TaskExecutor_Busy_Handler() {
	taskHandler := s.taskHandler
	// Make task handler not ready for tasks and check that Handle() was not called
	taskHandler.IsReadyToHandleFunc = func(ctx context.Context) (bool, error) {
		return false, nil
	}
	_, cancelFn := s.runTaskExecutor(s.context)
	defer cancelFn()

	s.Require().Never(
		func() bool {
			return len(taskHandler.HandleCalls()) != 0
		},
		time.Second,
		100*time.Millisecond,
	)
}
