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
	context       context.Context
	cancellation  context.CancelFunc
	taskExecutor  TaskExecutor
	targetHandler *api.TaskRequestHandlerMock
}

func (s *TestSuite) SetupTest() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.targetHandler = newTaskRequestHandlerMock()
	proverConfig := Config{
		TaskPollingInterval: 10 * time.Millisecond,
	}
	logger := logging.NewLogger("taskExecutor-test")
	taskExecutor, err := New(&proverConfig, s.targetHandler, &taskHandler{}, logger)
	s.Require().NoError(err)
	s.taskExecutor = taskExecutor
}

func (s *TestSuite) TearDownTest() {
	s.cancellation()
}

func newTaskRequestHandlerMock() *api.TaskRequestHandlerMock {
	return &api.TaskRequestHandlerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
			task := testaide.GenerateTask()
			return &task, nil
		},
		SetTaskResultFunc: func(ctx context.Context, result *types.TaskResult) error {
			return nil
		},
	}
}

type taskHandler struct{}

func (h *taskHandler) HandleTask(_ context.Context, _ *types.ProverTask) (TaskHandleResult, error) {
	return TaskHandleResult{Type: types.FinalProof}, nil
}
