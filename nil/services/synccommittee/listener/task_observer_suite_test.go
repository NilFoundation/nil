package listener

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover"
	"github.com/stretchr/testify/suite"
)

type TaskObserverTestSuite struct {
	suite.Suite
	context        context.Context
	cancellation   context.CancelFunc
	targetObserver *MockTaskObserver
	clientObserver api.TaskObserver
}

func (s *TaskObserverTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	listenerHttpEndpoint := "tcp://127.0.0.1:8530"
	s.targetObserver = NewMockTaskObserver()

	go func() {
		err := runTaskListener(s.context, listenerHttpEndpoint, s.targetObserver)
		s.NoError(err)
	}()

	s.clientObserver = prover.NewRemoteTaskObserver(
		listenerHttpEndpoint,
		logging.NewLogger("task_observer_client"),
	)
}

func (s *TaskObserverTestSuite) TearDownTest() {
	s.targetObserver.Reset()
}

func (s *TaskObserverTestSuite) TearDownSuite() {
	s.cancellation()
}

func runTaskListener(ctx context.Context, httpEndpoint string, observer api.TaskObserver) error {
	taskListener := NewTaskListener(
		&TaskListenerConfig{HttpEndpoint: httpEndpoint},
		observer,
		logging.NewLogger("sync_committee_task_listener"),
	)

	return taskListener.Run(ctx)
}
