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
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SyncCommitteeTestSuite struct {
	suite.Suite

	nShards       uint32
	syncCommittee *SyncCommittee
	nilCancel     context.CancelFunc
	doneChan      chan interface{} // to track when nilservice has finished
	ctx           context.Context
	nilDb         db.DB
	scDb          db.DB
}

func (s *SyncCommitteeTestSuite) waitZerostrate(endpoint string) {
	s.T().Helper()
	client := rpc.NewClient(endpoint, zerolog.Nop())
	const (
		zeroStateWaitTimeout  = 5 * time.Second
		zeroStatePollInterval = time.Second
	)
	for i := range s.nShards {
		s.Require().Eventually(func() bool {
			block, err := client.GetBlock(types.ShardId(i), transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, zeroStateWaitTimeout, zeroStatePollInterval)
	}
}

func (s *SyncCommitteeTestSuite) SetupSuite() {
	s.nShards = 4
	s.ctx = context.Background()

	url := rpctest.GetSockPath(s.T())

	var err error
	s.nilDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	// Setup nilservice
	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              url,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}
	var nilContext context.Context
	nilContext, s.nilCancel = context.WithCancel(context.Background())
	s.doneChan = make(chan interface{})
	go func() {
		nilservice.Run(nilContext, nilserviceCfg, s.nilDb, nil)
		s.doneChan <- nil
	}()

	s.waitZerostrate(url)

	cfg := &Config{
		RpcEndpoint:       url,
		PollingDelay:      time.Second,
		GracefulShutdown:  true,
		L1Endpoint:        "http://rpc2.sepolia.org",
		L1ChainId:         "11155111",
		PrivateKey:        "0000000000000000000000000000000000000000000000000000000000000001",
		L1ContractAddress: "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4",
	}

	s.scDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.syncCommittee, err = New(cfg, s.scDb)
	s.Require().NoError(err)
}

func (s *SyncCommitteeTestSuite) TearDownSuite() {
	s.nilCancel()
	<-s.doneChan // Wait for nilservice to shutdown
	s.nilDb.Close()
	s.scDb.Close()
}

func (s *SyncCommitteeTestSuite) SetupTest() {
	err := s.scDb.DropAll()
	s.Require().NoError(err)
}

func (s *SyncCommitteeTestSuite) TestProofThresholdMet() {
	// Case 1: Threshold not met
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		err := s.syncCommittee.aggregator.blockStorage.SetLastProvedBlockNum(s.ctx, shardId, 100)
		s.Require().NoError(err)
	}
	err := s.syncCommittee.aggregator.blockStorage.SetBlock(s.ctx, types.MainShardId, 100, &jsonrpc.RPCBlock{Number: 100})
	s.Require().NoError(err)
	proofThresholdMet, err := s.syncCommittee.aggregator.proofThresholdMet(s.ctx)
	s.Require().NoError(err)
	s.Require().False(proofThresholdMet)

	// Case 2: Threshold met
	err = s.syncCommittee.aggregator.blockStorage.SetBlock(s.ctx, types.MainShardId, 101, &jsonrpc.RPCBlock{Number: 101})
	s.Require().NoError(err)

	proofThresholdMet, err = s.syncCommittee.aggregator.proofThresholdMet(s.ctx)
	s.Require().NoError(err)
	s.Require().True(proofThresholdMet)
}

func (s *SyncCommitteeTestSuite) TestCreateProofTasks() {
	err := s.syncCommittee.aggregator.blockStorage.SetLastProvedBlockNum(s.ctx, types.MainShardId, 100)
	s.Require().NoError(err)
	err = s.syncCommittee.aggregator.blockStorage.SetBlock(s.ctx, types.MainShardId, types.BlockNumber(101), &jsonrpc.RPCBlock{Number: 101})
	s.Require().NoError(err)
	err = s.syncCommittee.aggregator.blockStorage.SetBlock(s.ctx, types.MainShardId, types.BlockNumber(102), &jsonrpc.RPCBlock{Number: 102})
	s.Require().NoError(err)

	err = s.syncCommittee.aggregator.createProofTasks(s.ctx, &jsonrpc.RPCBlock{Number: 102})
	s.Require().NoError(err)

	lastProvedBlkNum, err := s.syncCommittee.aggregator.blockStorage.GetLastProvedBlockNum(s.ctx, types.MainShardId)
	s.Require().NoError(err)
	s.Require().Equal(types.BlockNumber(100), lastProvedBlkNum)
}

func (s *SyncCommitteeTestSuite) TestUpdateLastProvedBlockNumForAllShards() {
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		blockNum := types.BlockNumber(100 + int64(shardId))
		err := s.syncCommittee.aggregator.blockStorage.SetBlock(s.ctx, shardId, blockNum, &jsonrpc.RPCBlock{Number: blockNum})
		s.Require().NoError(err)
	}

	err := s.syncCommittee.aggregator.updateLastProvedBlockNumForAllShards(s.ctx)
	s.Require().NoError(err)

	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		blockNum := types.BlockNumber(100 + int64(shardId))
		lastProvedBlkNum, err := s.syncCommittee.aggregator.blockStorage.GetLastProvedBlockNum(s.ctx, shardId)
		s.Require().NoError(err)
		s.Require().Equal(blockNum, lastProvedBlkNum)
	}
}

func (s *SyncCommitteeTestSuite) waitForAllShardsToProcess() {
	for i := range s.nShards {
		shardId := types.ShardId(i)
		s.Require().Eventually(
			func() bool {
				lastFetchedBlockNum, err := s.syncCommittee.aggregator.blockStorage.GetLastFetchedBlockNum(context.Background(), shardId)
				if err != nil || lastFetchedBlockNum == 0 {
					return false
				}
				lastProvedBlockNum, err := s.syncCommittee.aggregator.blockStorage.GetLastProvedBlockNum(context.Background(), shardId)
				if err != nil || lastProvedBlockNum == 0 {
					return false
				}
				return true
			},
			5*time.Second,
			100*time.Millisecond,
		)
	}
}

func (s *SyncCommitteeTestSuite) TestProcessingLoop() {
	// Run processing loop for a short time
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- s.syncCommittee.processingLoop(ctx)
	}()

	s.waitForAllShardsToProcess()

	cancel() // to avoid waiting without reason
	s.Require().NoError(<-errCh)
}

func (s *SyncCommitteeTestSuite) TestRun() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- s.syncCommittee.Run(ctx)
	}()

	s.waitForAllShardsToProcess()

	cancel() // to avoid waiting without reason
	s.Require().NoError(<-errCh)
}

func TestSyncCommitteeTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SyncCommitteeTestSuite))
}
