package rpc

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type ServerTestSuite struct {
	suite.Suite

	context      context.Context
	cancellation context.CancelFunc
	clock        *clockwork.FakeClock

	logger         logging.Logger
	metricsHandler *metrics.SyncCommitteeMetricsHandler

	database db.DB

	serverEndpoint string
}

func (s *ServerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.clock = testaide.NewTestClock()
	s.logger = logging.NewLogger("rpc_server_test")

	var err error
	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.metricsHandler, err = metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	s.serverEndpoint = rpc.GetSockPath(s.T())
}

func (s *ServerTestSuite) RunRpcServer(handler Handler) {
	s.T().Helper()

	started := make(chan struct{})
	go func() {
		rpcServer := NewServer(
			NewServerConfig(s.serverEndpoint),
			s.logger,
			handler,
		)

		err := rpcServer.Run(s.context, started)
		s.NoError(err)
	}()

	err := testaide.WaitFor(s.context, started, 10*time.Second)
	s.Require().NoError(err, "rpc server did not start in time")
}

func (s *ServerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *ServerTestSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err)
}
