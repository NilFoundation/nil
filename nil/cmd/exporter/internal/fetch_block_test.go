package internal

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteFetchBlock struct {
	suite.Suite

	nShards uint32
	cfg     Cfg
	context context.Context
	cancel  context.CancelFunc
}

func (s *SuiteFetchBlock) TestFetchBlock() {
	fetchedBlock, err := s.cfg.FetchBlock(s.context, types.MainShardId, "latest")
	s.Require().NoError(err, "Failed to fetch last block")

	s.Require().NotNil(fetchedBlock, "Fetched block is nil")

	blocks, err := s.cfg.FetchBlocks(s.context, types.MainShardId, fetchedBlock.Block.Id, fetchedBlock.Block.Id+1)
	s.Require().NoError(err, "Failed to fetch block by hash")
	s.Require().Len(blocks, 1, "Fetched one block")
	s.Require().Equal(fetchedBlock, blocks[0])
}

func (s *SuiteFetchBlock) TestFetchShardIdList() {
	shardIds, err := s.cfg.FetchShards(s.context)
	s.Require().NoError(err, "Failed to fetch shard ids")
	s.Require().Len(shardIds, int(s.nShards-1), "Shard ids length is not equal to expected")
}

func TestSuiteFetchBlock(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteFetchBlock))
}

func (s *SuiteFetchBlock) SetupSuite() {
	s.context, s.cancel = context.WithCancel(context.Background())
	s.nShards = 4

	url := rpctest.GetSockPath(s.T())
	logger := logging.NewLogger("test_exporter")
	s.cfg = Cfg{
		Client: rpc.NewClient(url, logger),
	}

	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	cfg := nilservice.NewDefaultConfig()
	cfg.NShards = s.nShards
	cfg.HttpUrl = url
	cfg.CollatorTickPeriodMs = 100
	go nilservice.Run(s.context, cfg, database, nil)
	tests.WaitBlock(s.T(), s.context, s.cfg.Client, types.MainShardId, 1)
}

func (s *SuiteFetchBlock) TearDownSuite() {
	s.cancel()
}
