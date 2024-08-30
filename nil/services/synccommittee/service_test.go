package synccommittee

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/tests/rpc_server"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SyncCommitteeTestSuite struct {
	suite.Suite

	nShards       uint32
	database      db.DB
	syncCommittee *SyncCommittee
	cancel        context.CancelFunc
	doneChan      chan interface{} // to track when nilservice has finished
	rpcURL        string
}

func (s *SyncCommitteeTestSuite) SetupSuite() {
	s.nShards = 4

	s.rpcURL = rpctest.GetSockPath(s.T())

	var err error
	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	// Setup nilservice
	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              s.rpcURL,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}
	var nilContext context.Context
	nilContext, s.cancel = context.WithCancel(context.Background())
	s.doneChan = make(chan interface{})
	go func() {
		nilservice.Run(nilContext, nilserviceCfg, s.database, nil)
		s.doneChan <- nil
	}()

	client := rpc.NewClient(s.rpcURL, zerolog.Nop())

	s.Require().Eventually(func() bool {
		shardIdList, err := client.GetShardIdList()
		return err == nil && len(shardIdList) > 0
	}, 5*time.Second, 200*time.Millisecond)
}

func (s *SyncCommitteeTestSuite) TearDownSuite() {
	s.cancel()
	<-s.doneChan // Wait for nilservice to shutdown
	s.database.Close()
}

func (s *SyncCommitteeTestSuite) SetupTest() {
	// for each test we recreate synccommittee so storage would be empty
	cfg := &Config{
		RpcEndpoint:      s.rpcURL,
		PollingDelay:     time.Second,
		GracefulShutdown: true,
	}

	var err error
	s.syncCommittee, err = New(cfg, s.database)
	s.Require().NoError(err)
}

func (s *SyncCommitteeTestSuite) TestNew() {
	s.Require().NotNil(s.syncCommittee)
	s.Require().Equal(s.database, s.syncCommittee.database)
	s.Require().NotNil(s.syncCommittee.logger)
	s.Require().NotNil(s.syncCommittee.client)
	s.Require().NotNil(s.syncCommittee.aggregator)
}

func (s *SyncCommitteeTestSuite) TestProcessingLoop() {
	// Set up initial state
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		s.syncCommittee.aggregator.storage.SetLastProvedBlockNum(shardId, 0)
	}

	// Run processing loop for a short time
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go s.syncCommittee.processingLoop(ctx)

	// Wait for processing to occur
	time.Sleep(time.Second * 5)

	// Check that blocks were processed
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		lastFetchedBlockNum := s.syncCommittee.aggregator.storage.GetLastFetchedBlockNum(shardId)
		s.Require().Greater(lastFetchedBlockNum, types.BlockNumber(0))

		lastProvedBlockNum := s.syncCommittee.aggregator.storage.GetLastProvedBlockNum(shardId)
		s.Require().Greater(lastProvedBlockNum, types.BlockNumber(0))
	}
}

func (s *SyncCommitteeTestSuite) TestRun() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- s.syncCommittee.Run(ctx)
	}()

	select {
	case err := <-errChan:
		s.Require().NoError(err)
	case <-ctx.Done():
		// Run completed without error
	}

	// Check that processing occurred
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		lastFetchedBlockNum := s.syncCommittee.aggregator.storage.GetLastFetchedBlockNum(shardId)
		s.Require().Greater(lastFetchedBlockNum, types.BlockNumber(0))
	}
}

func TestSyncCommitteeTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SyncCommitteeTestSuite))
}
