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
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.requestHandler = newTaskRequestHandlerMock()
	s.taskHandler = &api.TaskHandlerMock{}

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
		SetTaskResultFunc: func(ctx context.Context, result *types.TaskResult) error {
			return nil
		},
	}
}

func TestTaskExecutorSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) Test_TaskExecutor_Executes_Tasks() {
	started := make(chan struct{})
	go func() {
		_ = s.taskExecutor.Run(s.context, started)
	}()
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
			taskHandleCalls := s.taskHandler.HandleCalls()
			return len(taskHandleCalls) >= tasksThreshold
		},
		time.Second,
		10*time.Millisecond,
	)

	for _, call := range s.taskHandler.HandleCalls() {
		s.Require().Equal(s.taskExecutor.Id(), call.ExecutorId,
			"Task executor should have passed its id in the result")
	}
}
