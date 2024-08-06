package internal

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/stretchr/testify/suite"
)

type SuiteFetchBlock struct {
	suite.Suite

	nShards int
	cfg     Cfg
	context context.Context
	cancel  context.CancelFunc
}

func (suite *SuiteFetchBlock) TestFetchBlock() {
	fetchedBlock, err := suite.cfg.FetchBlock(types.MainShardId, "latest")
	suite.Require().NoError(err, "Failed to fetch last block")

	suite.Require().NotNil(fetchedBlock, "Fetched block is nil")

	blocks, err := suite.cfg.FetchBlocks(types.MainShardId, fetchedBlock.Block.Id, fetchedBlock.Block.Id+1)
	suite.Require().NoError(err, "Failed to fetch block by hash")
	suite.Require().Len(blocks, 1, "Fetched one block")
	suite.Require().Equal(fetchedBlock, blocks[0])
}

func (suite *SuiteFetchBlock) TestFetchShardIdList() {
	shardIds, err := suite.cfg.FetchShards()
	suite.Require().NoError(err, "Failed to fetch shard ids")
	suite.Require().Len(shardIds, suite.nShards-1, "Shard ids length is not equal to expected")
}

func TestSuiteFetchBlock(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteFetchBlock))
}

func (suite *SuiteFetchBlock) SetupSuite() {
	suite.context, suite.cancel = context.WithCancel(context.Background())
	suite.nShards = 4
	port := 8531

	url := "http://127.0.0.1:" + strconv.Itoa(port)
	logger := logging.NewLogger("test_exporter")
	suite.cfg = Cfg{
		Client: rpc.NewClient(url, logger),
	}

	database, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	cfg := &nilservice.Config{
		NShards:              suite.nShards,
		HttpPort:             port,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}
	go nilservice.Run(suite.context, cfg, database, nil)

	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteFetchBlock) TearDownSuite() {
	suite.cancel()
}
