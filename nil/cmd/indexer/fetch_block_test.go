package main

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteFetchBlock struct {
	suite.Suite

	server  tests.RpcSuite
	nShards uint32
	indexer *indexer.Indexer
	context context.Context
	cancel  context.CancelFunc
}

func (s *SuiteFetchBlock) TestFetchBlock() {
	fetchedBlock, err := s.indexer.FetchBlock(s.context, types.MainShardId, "latest")
	s.Require().NoError(err)
	s.Require().NotNil(fetchedBlock)

	blocks, err := s.indexer.FetchBlocks(s.context, types.MainShardId, fetchedBlock.Id, fetchedBlock.Id+10)
	s.Require().NoError(err)
	s.Require().NotEmpty(blocks)
	s.Require().Equal(fetchedBlock, blocks[0])
}

func (s *SuiteFetchBlock) TestFetchShardIdList() {
	shardIds, err := s.indexer.FetchShards(s.context)
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
	logger := logging.NewLogger("test_indexer")
	s.indexer = indexer.NewIndexerWithClient(rpc.NewClient(url, logger))

	cfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              url,
		CollatorTickPeriodMs: 100,
	}
	s.server.SetT(s.T())
	s.server.ShardsNum = s.nShards
	s.server.Context = s.context
	s.server.CtxCancel = s.cancel
	s.server.Start(cfg)
}

func (s *SuiteFetchBlock) TearDownSuite() {
	s.server.Cancel()
}
