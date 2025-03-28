package fetching

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type LagTrackerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	metrics      *metrics.SyncCommitteeMetricsHandler
	db           db.DB
	blockStorage *storage.BlockStorage

	rpcClientMock *client.ClientMock
	lagTracker    *lagTracker
}

func TestLagTrackerTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(LagTrackerTestSuite))
}

func (s *LagTrackerTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	logger := logging.NewLogger("lag_fetcher_test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.metrics = metricsHandler

	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	clock := clockwork.NewRealClock()
	s.blockStorage = storage.NewBlockStorage(s.db, storage.DefaultBlockStorageConfig(), clock, s.metrics, logger)
	s.rpcClientMock = &client.ClientMock{}

	s.lagTracker = NewLagTracker(s.rpcClientMock, s.blockStorage, s.metrics, logger)
}

func (s *LagTrackerTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.rpcClientMock.ResetCalls()
}

func (s *LagTrackerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *LagTrackerTestSuite) Test_Get_Lag_Nothing_Fetched_Yet() {
	// TODO: implement me
}
