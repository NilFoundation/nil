package synccommittee

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	rpctest "github.com/NilFoundation/nil/nil/tests/rpc_server"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type AggregatorTestSuite struct {
	suite.Suite

	nShards    uint32
	client     *rpc.Client
	logger     zerolog.Logger
	aggregator *Aggregator
	db         db.DB
	cancel     context.CancelFunc
	doneChan   chan interface{} // to track when nilservice has finished
}

func (s *AggregatorTestSuite) SetupSuite() {
	s.nShards = 4

	url := rpctest.GetSockPath(s.T())
	s.logger = logging.NewLogger("test_aggregator")

	s.client = rpc.NewClient(url, s.logger)
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              url,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}
	var nilContext context.Context
	nilContext, s.cancel = context.WithCancel(context.Background())
	s.doneChan = make(chan interface{})
	go func() {
		nilservice.Run(nilContext, nilserviceCfg, s.db, nil)
		s.doneChan <- nil
	}()

	s.Require().Eventually(func() bool {
		shardIdList, err := s.client.GetShardIdList()
		return err == nil && len(shardIdList) > 0
	}, 5*time.Second, 200*time.Millisecond)
}

func (s *AggregatorTestSuite) TearDownSuite() {
	s.cancel()
	<-s.doneChan // Wait for nilservice to shutdown
	s.db.Close()
}

func (s *AggregatorTestSuite) SetupTest() {
	var err error
	s.aggregator, err = NewAggregator(s.client, s.logger)
	s.Require().NoError(err)
}

func (s *AggregatorTestSuite) TestProcessNewBlocks() {
	err := s.aggregator.ProcessNewBlocks(context.Background())
	s.Require().NoError(err)

	// Check if blocks were fetched and stored for each shard
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		lastFetchedBlockNum := s.aggregator.storage.GetLastFetchedBlockNum(shardId)
		s.Require().Greater(lastFetchedBlockNum, types.BlockNumber(0))
	}
}

func (s *AggregatorTestSuite) TestFetchAndStoreBlocks() {
	shardIdList, err := s.aggregator.getShardIdList()
	s.Require().NoError(err)

	latestBlocks, err := s.aggregator.fetchLatestBlocks(shardIdList)
	s.Require().NoError(err)

	for _, shardId := range shardIdList {
		latestBlockForShardNumber := latestBlocks[shardId].Number
		err := s.aggregator.fetchAndStoreBlocks(context.Background(), shardId, 0, latestBlockForShardNumber)
		s.Require().NoError(err)

		// Check if blocks were stored
		for blockNum := types.BlockNumber(0); blockNum <= latestBlockForShardNumber; blockNum++ {
			block := s.aggregator.storage.GetBlock(shardId, blockNum)
			s.Require().NotNil(block)
			s.Require().Equal(blockNum, block.Number)
		}
	}
}

func (s *AggregatorTestSuite) TestValidateAndStoreBlock() {
	shardIdList, err := s.aggregator.getShardIdList()
	s.Require().NoError(err)

	latestBlocks, err := s.aggregator.fetchLatestBlocks(shardIdList)
	s.Require().NoError(err)

	for _, shardId := range shardIdList {
		latestBlock := latestBlocks[shardId]
		s.Require().NotNil(latestBlock)

		// Validate and store the block
		err = s.aggregator.validateAndStoreBlock(context.Background(), latestBlock)
		s.Require().NoError(err)

		// Check if the block was stored
		storedBlock := s.aggregator.storage.GetBlock(shardId, latestBlock.Number)
		s.Require().NotNil(storedBlock)
		s.Require().Equal(latestBlock.Number, storedBlock.Number)
		s.Require().Equal(latestBlock.Hash, storedBlock.Hash)

		// Check tasks for provers
		// if shardId == types.MainShardId {
		// TODO add check when it was impelmented
		// }
	}
}

func (s *AggregatorTestSuite) TestValidateAndStoreBlockMismatch() {
	block1 := jsonrpc.RPCBlock{ShardId: types.MainShardId, Number: 0, Hash: common.HexToHash("1")}
	// Set wrong parent hash
	block2 := jsonrpc.RPCBlock{ShardId: types.MainShardId, Number: 1, ParentHash: common.HexToHash("2")}

	// Store blocks
	s.aggregator.storage.SetBlock(&block1)
	s.aggregator.storage.SetBlock(&block2)

	// Try to validate and store the second block
	err := s.aggregator.validateAndStoreBlock(context.Background(), &block2)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "block hash mismatch")
}

func (s *AggregatorTestSuite) TestProofThresholdMet() {
	// Case 1: Threshold not met
	s.aggregator.storage.SetLastProvedBlockNum(types.MainShardId, 100)
	s.aggregator.storage.SetBlock(&jsonrpc.RPCBlock{ShardId: types.MainShardId, Number: 100})
	s.Require().False(s.aggregator.proofThresholdMet())

	// Case 2: Threshold met
	s.aggregator.storage.SetBlock(&jsonrpc.RPCBlock{ShardId: types.MainShardId, Number: 101})
	s.Require().True(s.aggregator.proofThresholdMet())
}

func (s *AggregatorTestSuite) TestUpdateLastProvedBlockNumForAllShards() {
	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		blockNum := types.BlockNumber(200 + int64(shardId))
		s.aggregator.storage.SetBlock(&jsonrpc.RPCBlock{ShardId: shardId, Number: blockNum})
	}

	err := s.aggregator.updateLastProvedBlockNumForAllShards()
	s.Require().NoError(err)

	for shardId := types.ShardId(0); shardId < types.ShardId(s.nShards); shardId++ {
		blockNum := types.BlockNumber(200 + int64(shardId))
		s.Require().Equal(blockNum, s.aggregator.storage.GetLastProvedBlockNum(shardId))
	}
}

func TestAggregatorTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AggregatorTestSuite))
}
