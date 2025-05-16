package rpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/stretchr/testify/suite"
)

type BlockDebugRpcTestSuite struct {
	ServerTestSuite
	l1Client  rollupcontract.WrapperMock
	storage   *storage.BlockStorage
	rpcClient public.BlockDebugApi
}

func TestBlockDebugRpcTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BlockDebugRpcTestSuite))
}

func (s *BlockDebugRpcTestSuite) SetupSuite() {
	s.ServerTestSuite.SetupSuite()

	s.l1Client = rollupcontract.WrapperMock{}
	cfg := storage.DefaultBlockStorageConfig()
	s.storage = storage.NewBlockStorage(s.database, cfg, s.clock, s.metricsHandler, s.logger)

	blockDebugger := debug.NewBlockDebugger(&s.l1Client, s.storage)
	handler := DebugBlocksServerHandler(blockDebugger)
	s.RunRpcServer(handler)

	s.rpcClient = NewBlockDebugRpcClient(s.serverEndpoint, s.logger)
}

func (s *BlockDebugRpcTestSuite) Test_GetLatestFetched_Empty() {
	refs, err := s.rpcClient.GetLatestFetched(s.context)
	s.Require().NoError(err)
	s.Require().Empty(refs)
}

func (s *BlockDebugRpcTestSuite) Test_GetLatestFetched() {
	batch := testaide.NewBlockBatch(testaide.ShardsCount)
	err := s.storage.PutBlockBatch(s.context, batch)
	s.Require().NoError(err)

	refs, err := s.rpcClient.GetLatestFetched(s.context)
	s.Require().NoError(err)
	s.Require().NotEmpty(refs)
	s.Require().Equal(batch.LatestRefs(), refs)
}

func (s *BlockDebugRpcTestSuite) Test_GetStateRootData_Empty_Local() {
	l1StateRoot := testaide.RandomHash()
	s.l1Client.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return l1StateRoot, nil
	}

	stateRootData, err := s.rpcClient.GetStateRootData(s.context)
	s.Require().NoError(err)
	s.Require().NotNil(stateRootData)

	s.Equal(l1StateRoot, stateRootData.L1StateRoot)
	s.Nil(stateRootData.LocalStateRoot)
}

func (s *BlockDebugRpcTestSuite) Test_GetStateRootData() {
	l1StateRoot := testaide.RandomHash()
	s.l1Client.GetLatestFinalizedStateRootFunc = func(ctx context.Context) (common.Hash, error) {
		return l1StateRoot, nil
	}

	localStateRoot := testaide.RandomHash()
	err := s.storage.SetProvedStateRoot(s.context, localStateRoot)
	s.Require().NoError(err)

	stateRootData, err := s.rpcClient.GetStateRootData(s.context)
	s.Require().NoError(err)
	s.Require().NotNil(stateRootData)

	s.Equal(l1StateRoot, stateRootData.L1StateRoot)
	s.Equal(&localStateRoot, stateRootData.LocalStateRoot)
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchView_NotExists() {
	batchId := types.NewBatchId()
	view, err := s.rpcClient.GetBatchView(s.context, batchId)
	s.Require().NoError(err)
	s.Require().Nil(view)
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchView() {
	batch := testaide.NewBlockBatch(testaide.ShardsCount)
	err := s.storage.PutBlockBatch(s.context, batch)
	s.Require().NoError(err)

	view, err := s.rpcClient.GetBatchView(s.context, batch.Id)
	s.Require().NoError(err)
	s.Require().NotNil(view)
	s.requireBatchViewEqual(batch, view)
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchViews_Empty() {
	request := public.DefaultBatchDebugRequest()
	views, err := s.rpcClient.GetBatchViews(s.context, request)
	s.Require().NoError(err)
	s.Require().Empty(views)
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchViews() {
	batches := testaide.NewBatchesSequence(3)
	for _, batch := range batches {
		err := s.storage.PutBlockBatch(s.context, batch)
		s.Require().NoError(err)
	}

	request := public.DefaultBatchDebugRequest()
	views, err := s.rpcClient.GetBatchViews(s.context, request)
	s.Require().NoError(err)
	s.Require().Len(views, len(batches))

	s.requireViewsEqualToLatest(views, batches)
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchViews_Limit() {
	batches := testaide.NewBatchesSequence(10)
	for _, batch := range batches {
		err := s.storage.PutBlockBatch(s.context, batch)
		s.Require().NoError(err)
	}

	limit := 5
	request := public.NewBatchDebugRequest(&limit)
	views, err := s.rpcClient.GetBatchViews(s.context, *request)
	s.Require().NoError(err)
	s.Require().Len(views, limit)

	s.requireViewsEqualToLatest(views, batches)
}

func (s *BlockDebugRpcTestSuite) requireViewsEqualToLatest(
	views []*public.BatchViewCompact, batches []*types.BlockBatch,
) {
	s.T().Helper()

	// Views are expected to be in reverse order (the latest batch comes first)

	for i, view := range views {
		batch := batches[len(batches)-1-i]
		s.requireBatchViewCompactEqual(batch, view)
	}
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchStats_Empty() {
	stats, err := s.rpcClient.GetBatchStats(s.context)
	s.Require().NoError(err)
	s.Require().Equal(public.NewBatchStats(0, 0, 0), stats)
}

func (s *BlockDebugRpcTestSuite) Test_GetBatchStats() {
	batches := testaide.NewBatchesSequence(3)

	for _, batch := range batches {
		err := s.storage.PutBlockBatch(s.context, batch)
		s.Require().NoError(err)
	}

	stats, err := s.rpcClient.GetBatchStats(s.context)
	s.Require().NoError(err)
	s.Require().Equal(public.NewBatchStats(len(batches), 0, 0), stats)

	// Seal the first two batches

	fstSealed, err := batches[0].Seal(testaide.NewDataProofs(), s.clock.Now())
	s.Require().NoError(err)
	err = s.storage.PutBlockBatch(s.context, fstSealed)
	s.Require().NoError(err)

	sndSealed, err := batches[1].Seal(testaide.NewDataProofs(), s.clock.Now())
	s.Require().NoError(err)
	err = s.storage.PutBlockBatch(s.context, sndSealed)
	s.Require().NoError(err)

	// Mark the first batch as proved

	err = s.storage.SetBatchAsProved(s.context, batches[0].Id)
	s.Require().NoError(err)

	stats, err = s.rpcClient.GetBatchStats(s.context)
	s.Require().NoError(err)
	s.Require().Equal(public.NewBatchStats(len(batches), 2, 1), stats)
}

func (s *BlockDebugRpcTestSuite) requireBatchViewEqual(
	expected *types.BlockBatch, actual *public.BatchViewDetailed,
) {
	s.T().Helper()
	s.Require().Equal(expected.Id, actual.Id)
	s.Require().Equal(expected.ParentId, actual.ParentId)
	s.Require().Equal(expected.IsSealed, actual.IsSealed)
	s.Require().Equal(expected.CreatedAt, actual.CreatedAt)
	s.Require().Equal(expected.UpdatedAt, actual.UpdatedAt)
}

func (s *BlockDebugRpcTestSuite) requireBatchViewCompactEqual(
	expected *types.BlockBatch, actual *public.BatchViewCompact,
) {
	s.T().Helper()
	s.Require().Equal(expected.Id, actual.Id)
	s.Require().Equal(expected.ParentId, actual.ParentId)
	s.Require().Equal(expected.IsSealed, actual.IsSealed)
	s.Require().Equal(expected.CreatedAt, actual.CreatedAt)
}
