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
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
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

	logger := logging.NewLogger("aggregator_test")
	metricsHandler, err := metrics.NewHandler("aggregator_test")
	s.Require().NoError(err)

	s.client = rpc.NewClient(url, logger)
	s.scDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.storage = storage.NewBlockStorage(s.scDb, metricsHandler, logger)
	s.Require().NoError(err)

	s.aggregator, err = NewAggregator(
		s.client,
		s.storage,
		storage.NewTaskStorage(s.scDb, metricsHandler, logger),
		logger,
		metricsHandler,
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

	latestFetched, err := s.storage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetched)
	s.Require().Greater(latestFetched.Number, coreTypes.BlockNumber(0))
}

func (s *AggregatorTestSuite) TestFetchAndProcessBlocks() {
	latestBlock, err := s.aggregator.fetchLatestBlockRef()
	s.Require().NoError(err)

	blocksRange := scTypes.BlocksRange{End: latestBlock.Number}
	err = s.aggregator.fetchAndProcessBlocks(s.ctx, blocksRange)
	s.Require().NoError(err)

	// Check if blocks were stored
	block, err := s.aggregator.blockStorage.TryGetBlock(s.ctx, scTypes.NewBlockId(coreTypes.MainShardId, latestBlock.Hash))
	s.Require().NoError(err)
	s.Require().NotNil(block)
}

func (s *AggregatorTestSuite) TestValidateAndProcessBlock() {
	latestBlock, err := s.aggregator.fetchLatestBlockRef()
	s.Require().NoError(err)

	// Fetch the latest block
	block, err := s.client.GetBlock(coreTypes.MainShardId, transport.BlockNumber(latestBlock.Number), false)
	s.Require().NoError(err)
	s.Require().NotNil(block)
	block.ChildBlocks = make([]common.Hash, 0)

	// Validate and store the block
	err = s.aggregator.validateAndProcessBlock(s.ctx, block, block.Hash)
	s.Require().NoError(err)

	// Check if the block was stored
	storedBlock, err := s.storage.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
	s.Require().NoError(err)
	s.Require().NotNil(storedBlock)
	s.Require().Equal(block.Number, storedBlock.Number)
	s.Require().Equal(block.Hash, storedBlock.Hash)
}

func TestAggregatorTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(AggregatorTestSuite))
}
