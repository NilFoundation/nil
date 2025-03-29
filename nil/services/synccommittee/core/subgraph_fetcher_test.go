package core

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type SubgraphFetcherTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	logger        logging.Logger
	rpcClientMock *client.ClientMock
	fetcher       *subgraphFetcher
}

func TestSubgraphFetcherTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SubgraphFetcherTestSuite))
}

func (s *SubgraphFetcherTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())
	s.logger = logging.NewLogger("subgraph_fetcher_test")
}

func (s *SubgraphFetcherTestSuite) SetupTest() {
	s.rpcClientMock = &client.ClientMock{}
	s.fetcher = newSubgraphFetcher(s.rpcClientMock, s.logger)
}

func (s *SubgraphFetcherTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *SubgraphFetcherTestSuite) Test_Child_Does_Not_Exist() {
	batches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	unknownMainBlock := testaide.NewMainShardBlock()
	emptyRefs := make(types.BlockRefs)

	subgraph, err := s.fetcher.FetchSubgraph(s.ctx, unknownMainBlock, emptyRefs)
	s.Require().ErrorIs(err, types.ErrBlockNotFound)
	s.Require().Nil(subgraph)
}

func (s *SubgraphFetcherTestSuite) Test_Child_Fetched_Mismatch() {
	batches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)
	targetBatch := batches[len(batches)-1]

	mainBlock := targetBatch.LatestMainBlock()

	// emulating SyncCommittee getting ahead of the cluster in shard 1
	refs := targetBatch.LatestRefs()
	refs[1] = types.NewBlockRef(refs[1].ShardId, refs[1].Hash, refs[1].Number+10)

	subgraph, err := s.fetcher.FetchSubgraph(s.ctx, mainBlock, refs)
	s.Require().ErrorIs(err, types.ErrBlockMismatch)
	s.Require().Nil(subgraph)
}

func (s *SubgraphFetcherTestSuite) Test_Fetch_No_Latest_Refs() {
	batches := testaide.NewBatchesSequence(1)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)
	targetBatch := batches[len(batches)-1]

	mainBlock := targetBatch.LatestMainBlock()
	emptyRefs := make(types.BlockRefs)

	subgraph, err := s.fetcher.FetchSubgraph(s.ctx, mainBlock, emptyRefs)
	s.Require().NoError(err)
	s.Require().NotNil(subgraph)

	expectedSubgraph := targetBatch.Subgraphs[len(targetBatch.Subgraphs)-1]
	s.Require().Equal(expectedSubgraph, *subgraph)
}
