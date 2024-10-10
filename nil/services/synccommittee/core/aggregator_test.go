package core

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type AggregatorTestSuite struct {
	suite.Suite

	nShards    uint32
	client     *rpc.Client
	storage    storage.BlockStorage
	aggregator *Aggregator
	nilDb      db.DB
	scDb       db.DB
	nilCancel  context.CancelFunc
	ctx        context.Context
	doneChan   chan interface{} // to track when nilservice has finished
}

func (s *AggregatorTestSuite) waitTwoBlocks(endpoint string) {
	s.T().Helper()
	client := rpc.NewClient(endpoint, zerolog.Nop())
	const (
		waitTimeout  = 5 * time.Second
		pollInterval = time.Second
	)
	for i := range s.nShards {
		s.Require().Eventually(func() bool {
			block, err := client.GetBlock(coreTypes.ShardId(i), transport.BlockNumber(1), false)
			return err == nil && block != nil
		}, waitTimeout, pollInterval)
	}
}

func (s *AggregatorTestSuite) SetupSuite() {
	s.nShards = 4
	s.ctx = context.Background()

	url := rpctest.GetSockPath(s.T())

	var err error
	s.nilDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

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

	s.waitTwoBlocks(url)

	logger := logging.NewLogger("test_block_aggregator")
	s.client = rpc.NewClient(url, logger)
	s.scDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	metrics, err := NewMetricsHandler("github.com/NilFoundation/nil/nil/services/sync_committee")
	s.Require().NoError(err)
	s.storage = storage.NewBlockStorage(s.scDb, logger)
	s.Require().NoError(err)
	s.aggregator, err = NewAggregator(
		s.client,
		s.storage,
		storage.NewTaskStorage(s.scDb, logger),
		logger,
		metrics,
		time.Second,
	)
	s.Require().NoError(err)
}

func (s *AggregatorTestSuite) TearDownSuite() {
	s.nilCancel()
	<-s.doneChan // Wait for nilservice to shutdown
	s.nilDb.Close()
	s.scDb.Close()
}

func (s *AggregatorTestSuite) SetupTest() {
	err := s.scDb.DropAll()
	s.Require().NoError(err)
}

func (s *AggregatorTestSuite) TestProcessNewBlocks() {
	err := s.aggregator.processNewBlocks(context.Background())
	s.Require().NoError(err)

	// Check if blocks were fetched and stored for each shard
	for shardId := coreTypes.ShardId(0); shardId < coreTypes.ShardId(s.nShards); shardId++ {
		lastFetchedBlockNum, err := s.storage.GetLastFetchedBlockNum(s.ctx, shardId)
		s.Require().NoError(err)
		s.Require().Greater(lastFetchedBlockNum, coreTypes.BlockNumber(0))
	}
}

func (s *AggregatorTestSuite) TestFetchAndProcessBlocks() {
	shardIdList, err := s.aggregator.getShardIdList()
	s.Require().NoError(err)

	latestBlocks, err := s.aggregator.fetchLatestBlocks(shardIdList)
	s.Require().NoError(err)

	for _, shardId := range shardIdList {
		latestBlockForShard := latestBlocks[shardId].Number
		err := s.aggregator.fetchAndProcessBlocks(s.ctx, shardId, 0, latestBlockForShard)
		s.Require().NoError(err)

		// Check if blocks were stored
		for blkNum := range latestBlockForShard {
			block, err := s.aggregator.blockStorage.GetBlock(s.ctx, shardId, blkNum)
			s.Require().NoError(err)
			s.Require().Equal(blkNum, block.Number)
		}
	}
}

func (s *AggregatorTestSuite) TestValidateAndProcessBlock() {
	shardIdList, err := s.aggregator.getShardIdList()
	s.Require().NoError(err)

	latestBlocks, err := s.aggregator.fetchLatestBlocks(shardIdList)
	s.Require().NoError(err)

	for _, shardId := range shardIdList {
		latestBlock := latestBlocks[shardId]
		s.Require().NotNil(latestBlock)

		// Fetch the latest block
		block, err := s.client.GetBlock(shardId, transport.BlockNumber(latestBlock.Number), false)
		s.Require().NoError(err)
		s.Require().NotNil(block)

		// Validate and store the block
		err = s.aggregator.validateAndProcessBlock(context.Background(), block)
		s.Require().NoError(err)

		// Check if the block was stored
		storedBlock, err := s.storage.GetBlock(s.ctx, shardId, block.Number)
		s.Require().NoError(err)
		s.Require().NotNil(storedBlock)
		s.Require().Equal(block.Number, storedBlock.Number)
		s.Require().Equal(block.Hash, storedBlock.Hash)
	}
}

func (s *AggregatorTestSuite) TestValidateAndStoreBlockMismatch() {
	shardIdList, err := s.aggregator.getShardIdList()
	s.Require().NoError(err)

	latestBlocks, err := s.aggregator.fetchLatestBlocks(shardIdList)
	s.Require().NoError(err)

	for _, shardId := range shardIdList {
		latestBlock := latestBlocks[shardId]
		s.Require().NotNil(latestBlock)

		// Fetch two consecutive blocks
		block1, err := s.client.GetBlock(shardId, transport.BlockNumber(latestBlock.Number-1), false)
		s.Require().NoError(err)
		s.Require().NotNil(block1)

		block2, err := s.client.GetBlock(shardId, transport.BlockNumber(latestBlock.Number), false)
		s.Require().NoError(err)
		s.Require().NotNil(block2)

		// Store the first block
		err = s.storage.SetBlock(s.ctx, shardId, block1.Number, block1)
		s.Require().NoError(err)

		// Modify the parent hash of the second block to create a mismatch
		block2.ParentHash = common.EmptyHash

		// Try to validate and store the second block
		err = s.aggregator.validateAndProcessBlock(s.ctx, block2)
		s.Require().Error(err)
		s.ErrorIs(err, ErrBlockHashMismatch)
	}
}

func TestAggregatorTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AggregatorTestSuite))
}
