package rpc

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type RpcServerTestSuite struct {
	suite.Suite

	context      context.Context
	cancellation context.CancelFunc
	clock        *clockwork.FakeClock

	logger         logging.Logger
	metricsHandler *metrics.SyncCommitteeMetricsHandler

	database db.DB
	storage  *storage.TaskStorage

	serverEndpoint string
}

func (s *RpcServerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.clock = testaide.NewTestClock()
	s.logger = logging.NewLogger("rpc_server_test")

	var err error
	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.metricsHandler, err = metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.storage = storage.NewTaskStorage(s.database, s.clock, s.metricsHandler, s.logger)

	started := make(chan struct{})
	s.serverEndpoint = rpc.GetSockPath(s.T())
	go func() {
		err := s.runRpcServer(started)
		s.NoError(err)
	}()
	err = testaide.WaitFor(s.context, started, 10*time.Second)
	s.Require().NoError(err, "rpc server did not start in time")
}

func (s *RpcServerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *RpcServerTestSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err)
}

func (s *RpcServerTestSuite) runRpcServer(started chan<- struct{}) error {
	noopStateHandler := &api.TaskStateChangeHandlerMock{}
	taskScheduler := scheduler.New(s.storage, noopStateHandler, s.metricsHandler, s.logger)
	taskDebugger := scheduler.NewTaskDebugger(s.storage, s.logger)

	rpcServer := NewServerWithTasks(
		NewServerConfig(s.serverEndpoint),
		s.logger,
		taskScheduler,
		taskDebugger,
	)

	return rpcServer.Run(s.context, started)
}
