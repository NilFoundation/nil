package exporter

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
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
	fetchedBlock, err := suite.cfg.FetchLastBlock(types.MasterShardId)
	suite.Require().NoError(err, "Failed to fetch last block")

	suite.Require().NotNil(fetchedBlock, "Fetched block is nil")

	hashBlock, err := suite.cfg.FetchBlockByHash(types.MasterShardId, fetchedBlock.Block.Hash())
	suite.Require().NoError(err, "Failed to fetch block by hash")
	suite.Require().NotNil(hashBlock, "Fetched block by hash is nil")

	suite.Require().Equal(fetchedBlock, hashBlock)
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
	go nilservice.Run(suite.context, cfg, database)

	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteFetchBlock) TearDownSuite() {
	suite.cancel()
}
