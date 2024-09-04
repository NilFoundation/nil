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
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
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
		s.syncCommittee.aggregator.blockStorage.SetLastProvedBlockNum(shardId, 0)
	}

	// Run processing loop for a short time
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		err := s.syncCommittee.processingLoop(ctx)
		s.NoError(err)
	}()

	s.Require().Eventually(
		func() bool {
			for id := range s.nShards {
				shardId := types.ShardId(id)
				lastFetchedBlockNum := s.syncCommittee.aggregator.blockStorage.GetLastFetchedBlockNum(shardId)
				if lastFetchedBlockNum == 0 {
					return false
				}
				lastProvedBlockNum := s.syncCommittee.aggregator.blockStorage.GetLastProvedBlockNum(shardId)
				if lastProvedBlockNum == 0 {
					return false
				}
			}
			return true
		},
		5*time.Second,
		100*time.Millisecond,
	)
}

func (s *SyncCommitteeTestSuite) TestRun() {
	ctx := context.Background()

	go func() {
		err := s.syncCommittee.Run(ctx)
		s.NoError(err)
	}()

	s.Require().Eventually(
		func() bool {
			for id := range s.nShards {
				shardId := types.ShardId(id)
				lastFetchedBlockNum := s.syncCommittee.aggregator.blockStorage.GetLastFetchedBlockNum(shardId)
				if lastFetchedBlockNum == 0 {
					return false
				}
			}
			return true
		},
		5*time.Second,
		100*time.Millisecond,
	)
}

func TestSyncCommitteeTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SyncCommitteeTestSuite))
}
