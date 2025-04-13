package fetching

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
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

	s.lagTracker = NewLagTracker(
		s.rpcClientMock, s.blockStorage, s.metrics, NewDefaultLagTrackerConfig(), logger,
	)
}

func (s *LagTrackerTestSuite) SetupTest() {
	s.rpcClientMock.ResetCalls()
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
}

func (s *LagTrackerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *LagTrackerTestSuite) Test_GetLagForAllShards_Nothing_Fetched_Yet() {
	batches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	shardLagExpected := s.countBlocks(batches)

	shardLagActual, err := s.lagTracker.getLagForAllShards(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(shardLagExpected, shardLagActual)
}

func (s *LagTrackerTestSuite) Test_GetLagForAllShards_No_Lag() {
	batches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	for _, batch := range batches {
		err := s.blockStorage.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	currentLag, err := s.lagTracker.getLagForAllShards(s.ctx)
	s.Require().NoError(err)

	latestRefs := batches[len(batches)-1].LatestRefs()

	for shardId := range latestRefs {
		shardLag, ok := currentLag[shardId]
		s.True(ok)
		s.Equal(int64(0), shardLag)
	}
}

func (s *LagTrackerTestSuite) Test_GetLagForAllShards_Lagging_Behind() {
	batches := testaide.NewBatchesSequence(5)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)
	alreadyFetched := batches[:3]
	pending := batches[3:]

	for _, batch := range alreadyFetched {
		err := s.blockStorage.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	currentLagActual, err := s.lagTracker.getLagForAllShards(s.ctx)
	s.Require().NoError(err)

	currentLagExpected := s.countBlocks(pending)
	s.Require().Equal(currentLagExpected, currentLagActual)
}

func (s *LagTrackerTestSuite) Test_GetLagForAllShards_Being_Ahead() {
	batches := testaide.NewBatchesSequence(5)
	stateOnL2 := batches[:3]
	testaide.ClientMockSetBatches(s.rpcClientMock, stateOnL2)

	for _, batch := range batches {
		err := s.blockStorage.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	currentLagActual, err := s.lagTracker.getLagForAllShards(s.ctx)
	s.Require().NoError(err)

	diff := batches[3:]
	currentLagExpected := s.countBlocks(diff)

	s.Require().Len(currentLagActual, len(currentLagExpected))
	for shardId, blocksCount := range currentLagExpected {
		shardLagActual, ok := currentLagActual[shardId]
		s.True(ok)
		// Expecting shard lag to be negative (Sync Committee is ahead of L2)
		shardLagExpected := -blocksCount
		s.Equal(shardLagExpected, shardLagActual)
	}
}

func (s *LagTrackerTestSuite) countBlocks(batches []*scTypes.BlockBatch) map[types.ShardId]int64 {
	s.T().Helper()

	countPerShard := make(map[types.ShardId]int64)
	for _, batch := range batches {
		for block := range batch.BlocksIter() {
			countPerShard[block.ShardId]++
		}
	}
	return countPerShard
}
