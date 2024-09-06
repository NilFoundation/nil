package prover

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/listener"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	context       context.Context
	cancellation  context.CancelFunc
	prover        *Prover
	targetHandler *listener.TaskRequestHandlerMock
}

func (s *TestSuite) SetupTest() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.targetHandler = listener.NewTaskRequestHandlerMock()
	proverConfig := Config{
		TaskPollingInterval: 10 * time.Millisecond,
	}
	logger := logging.NewLogger("prover-test")
	newProver, err := NewProver(&proverConfig, s.targetHandler, NewTaskHandler(logger), logger)
	s.Require().NoError(err)
	s.prover = newProver
}

func (s *TestSuite) TearDownTest() {
	s.cancellation()
}
