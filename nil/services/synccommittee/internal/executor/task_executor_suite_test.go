package executor

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
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
	taskHandler    *TaskHandlerMock
}

func (s *TestSuite) SetupTest() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.requestHandler = newTaskRequestHandlerMock()
	s.taskHandler = &TaskHandlerMock{}

	config := Config{
		TaskPollingInterval: 10 * time.Millisecond,
	}
	logger := logging.NewLogger("taskExecutor-test")
	taskExecutor, err := New(&config, s.requestHandler, s.taskHandler, logger)
	s.Require().NoError(err)
	s.taskExecutor = taskExecutor
}

func (s *TestSuite) TearDownTest() {
	s.cancellation()
}

func newTaskRequestHandlerMock() *api.TaskRequestHandlerMock {
	return &api.TaskRequestHandlerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.Task, error) {
			task := testaide.GenerateTask()
			return &task, nil
		},
		SetTaskResultFunc: func(ctx context.Context, result *types.TaskResult) error {
			return nil
		},
	}
}
