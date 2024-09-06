package listener

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover"
	"github.com/stretchr/testify/suite"
)

type TaskRequestHandlerTestSuite struct {
	suite.Suite
	context       context.Context
	cancellation  context.CancelFunc
	targetHandler *TaskRequestHandlerMock
	clientHandler api.TaskRequestHandler
}

func (s *TaskRequestHandlerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	listenerHttpEndpoint := "tcp://127.0.0.1:8530"
	s.targetHandler = NewTaskRequestHandlerMock()

	go func() {
		err := runTaskListener(s.context, listenerHttpEndpoint, s.targetHandler)
		s.NoError(err)
	}()

	s.clientHandler = prover.NewTaskRequestRpcClient(
		listenerHttpEndpoint,
		logging.NewLogger("task-request-rpc-client"),
	)
}

func (s *TaskRequestHandlerTestSuite) TearDownTest() {
	s.targetHandler.Reset()
}

func (s *TaskRequestHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func runTaskListener(ctx context.Context, httpEndpoint string, handler api.TaskRequestHandler) error {
	taskListener := NewTaskListener(
		&TaskListenerConfig{HttpEndpoint: httpEndpoint},
		handler,
		logging.NewLogger("sync-committee-task-listener"),
	)

	return taskListener.Run(ctx)
}
