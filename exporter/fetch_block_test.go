package exporter

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
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
	fetchedBlock, err := suite.cfg.FetchLastBlock(suite.context, types.MasterShardId)
	suite.Require().NoError(err, "Failed to fetch last block")

	suite.Require().NotNil(fetchedBlock, "Fetched block is nil")

	hashBlock, err := suite.cfg.FetchBlockByHash(suite.context, types.MasterShardId, fetchedBlock.Block.Hash())
	suite.Require().NoError(err, "Failed to fetch block by hash")
	suite.Require().NotNil(hashBlock, "Fetched block by hash is nil")

	suite.Require().Equal(fetchedBlock.Block.Id, hashBlock.Block.Id)
	suite.Require().Equal(fetchedBlock.Block.PrevBlock, hashBlock.Block.PrevBlock)
	suite.Require().Equal(fetchedBlock.Block.SmartContractsRoot, hashBlock.Block.SmartContractsRoot)
	suite.Require().Equal(fetchedBlock.Block.InMessagesRoot, hashBlock.Block.InMessagesRoot)
}

func (suite *SuiteFetchBlock) TestFetchShardIdList() {
	shardIds, err := suite.cfg.FetchShards(suite.context)
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
	suite.cfg = Cfg{
		APIEndpoints: []string{"http://127.0.0.1:" + strconv.Itoa(port)},
	}

	database, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	cfg := &nilservice.Config{
		NShards:              suite.nShards,
		HttpPort:             port,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
	}
	go nilservice.Run(suite.context, cfg, database)

	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteFetchBlock) TearDownSuite() {
	suite.cancel()
}
