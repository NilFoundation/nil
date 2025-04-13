package fetching

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type AggregatorTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	metrics      *metrics.SyncCommitteeMetricsHandler
	db           db.DB
	blockStorage *storage.BlockStorage
	taskStorage  *storage.TaskStorage

	rpcClientMock *client.ClientMock
	aggregator    *aggregator
}

func TestAggregatorTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AggregatorTestSuite))
}

func (s *AggregatorTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	logger := logging.NewLogger("aggregator_test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.metrics = metricsHandler

	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	clock := clockwork.NewRealClock()
	s.blockStorage = s.newTestBlockStorage(storage.DefaultBlockStorageConfig())
	s.taskStorage = storage.NewTaskStorage(s.db, clock, s.metrics, logger)
	s.rpcClientMock = &client.ClientMock{}

	s.aggregator = s.newTestAggregator(s.blockStorage)
}

func (s *AggregatorTestSuite) newTestAggregator(
	blockStorage AggregatorBlockStorage,
) *aggregator {
	s.T().Helper()

	logger := logging.NewLogger("aggregator_test")
	stateResetter := reset.NewStateResetter(logger, s.blockStorage)
	clock := clockwork.NewRealClock()

	contractWrapperConfig := rollupcontract.WrapperConfig{
		DisableL1: true,
	}
	contractWrapper, err := rollupcontract.NewWrapper(s.ctx, contractWrapperConfig, logger)
	s.Require().NoError(err)

	return NewAggregator(
		s.rpcClientMock,
		blockStorage,
		s.taskStorage,
		stateResetter,
		contractWrapper,
		clock,
		logger,
		s.metrics,
		NewDefaultAggregatorConfig(),
	)
}

func (s *AggregatorTestSuite) newTestBlockStorage(config storage.BlockStorageConfig) *storage.BlockStorage {
	s.T().Helper()
	clock := clockwork.NewRealClock()
	return storage.NewBlockStorage(s.db, config, clock, s.metrics, logging.NewLogger("aggregator_test"))
}

func (s *AggregatorTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.rpcClientMock.ResetCalls()
}

func (s *AggregatorTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *AggregatorTestSuite) Test_No_New_Blocks_To_Fetch() {
	batch := testaide.NewBlockBatch(testaide.ShardsCount)
	err := s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	testaide.ClientMockSetBatches(s.rpcClientMock, []*scTypes.BlockBatch{batch})

	err = s.aggregator.processBlocksAndHandleErr(s.ctx)
	s.Require().NoError(err)

	// latest fetched block ref was not changed
	latestFetched, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	expectedLatest := batch.LatestRefs()
	s.Require().Equal(expectedLatest, latestFetched)

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Main_Parent_Hash_Mismatch() {
	batches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)
	err := s.blockStorage.SetProvedStateRoot(s.ctx, batches[0].FirstMainBlock().ParentHash)
	s.Require().NoError(err)

	// Set first 2 batches as proved
	for _, provedBatch := range batches[:2] {
		err := s.blockStorage.SetBlockBatch(s.ctx, provedBatch)
		s.Require().NoError(err)
		err = s.blockStorage.SetBatchAsProved(s.ctx, provedBatch.Id)
		s.Require().NoError(err)
	}

	// Set first batch as proposed, latestProvedStateRoot value is updated
	err = s.blockStorage.SetBatchAsProposed(s.ctx, batches[0].Id)
	s.Require().NoError(err)
	latestProved, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestProved)
	s.Require().Equal(batches[0].LatestMainBlock().Hash, *latestProved)

	nextMainBlock := batches[2].LatestMainBlock()
	nextMainBlock.ParentHash = testaide.RandomHash()

	err = s.aggregator.processBlocksAndHandleErr(s.ctx)
	s.Require().NoError(err)

	// latest fetched block was reset
	mainRef, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(mainRef)
	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Fetch_At_Zero_State() {
	mainRefs, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(mainRefs)

	batches := testaide.NewBatchesSequence(2)
	err = s.blockStorage.SetProvedStateRoot(s.ctx, batches[0].LatestMainBlock().Hash)
	s.Require().NoError(err)

	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	nextHandledBlock := batches[1].LatestMainBlock()

	err = s.aggregator.processBlocksAndHandleErr(s.ctx)
	s.Require().NoError(err)
	s.requireMainBlockHandled(nextHandledBlock)
}

func (s *AggregatorTestSuite) Test_Fetch_Next_Valid() {
	batches := testaide.NewBatchesSequence(2)
	err := s.blockStorage.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)
	nextMainBlock := batches[1].LatestMainBlock()

	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	err = s.aggregator.processBlocksAndHandleErr(s.ctx)
	s.Require().NoError(err)
	s.requireMainBlockHandled(nextMainBlock)
}

func (s *AggregatorTestSuite) Test_Block_Storage_Capacity_Exceeded() {
	// only one batch can fit in the storage
	storageConfig := storage.NewBlockStorageConfig(1)
	blockStorage := s.newTestBlockStorage(storageConfig)

	batches := testaide.NewBatchesSequence(2)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	err := blockStorage.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)

	agg := s.newTestAggregator(blockStorage)

	latestFetchedBeforeNext, err := blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetchedBeforeNext)

	err = agg.processBlockRange(s.ctx)
	s.Require().ErrorIs(err, storage.ErrCapacityLimitReached)

	latestFetchedAfterNext, err := blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Equal(latestFetchedBeforeNext, latestFetchedAfterNext)

	// nextBatch should not be handled by Aggregator due to storage capacity limit
	nextBatch := batches[1]

	for block := range nextBatch.BlocksIter() {
		storedBlock, err := s.blockStorage.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(storedBlock)
	}

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_State_Root_Is_Not_Initialized() {
	batches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	err := s.aggregator.processBlockRange(s.ctx)
	s.Require().ErrorIs(err, storage.ErrStateRootNotInitialized)

	latestFetched, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(latestFetched)

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Latest_Fetched_Does_Not_Exist_On_Chain() {
	batches := testaide.NewBatchesSequence(3)

	err := s.blockStorage.SetProvedStateRoot(s.ctx, batches[0].LatestMainBlock().Hash)
	s.Require().NoError(err)

	testaide.ClientMockSetBatches(s.rpcClientMock, batches)

	err = s.aggregator.processBlockRange(s.ctx)
	s.Require().NoError(err)

	latestFetched, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetched)
	s.Require().Equal(batches[len(batches)-1].LatestMainBlock().Hash, latestFetched.TryGetMain().Hash)

	// emulating L2 reset
	newBatches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, newBatches)

	err = s.aggregator.processBlockRange(s.ctx)
	s.Require().ErrorIs(err, scTypes.ErrBlockMismatch)
	s.Require().ErrorContains(err, "block not found in chain")
}

// requireNoNewTasks asserts that there are no new tasks available for execution
func (s *AggregatorTestSuite) requireNoNewTasks() {
	s.T().Helper()
	task, err := s.taskStorage.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(task, "expected no new tasks available for execution, but got one")
}

func (s *AggregatorTestSuite) requireMainBlockHandled(mainBlock *scTypes.Block) {
	s.T().Helper()

	// latest fetched block was updated
	latestFetched, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetched)
	mainRef := latestFetched.TryGetMain()
	s.Require().True(latestFetched.TryGetMain().Equals(mainRef))

	// main + exec block were saved to the storage
	s.requireBlockStored(scTypes.IdFromBlock(mainBlock))
	childIds, err := scTypes.ChildBlockIds(mainBlock)
	s.Require().NoError(err)
	for _, childId := range childIds {
		s.requireBlockStored(childId)
	}

	// one ProofBatch task created
	taskToExecute, err := s.taskStorage.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(scTypes.ProofBatch, taskToExecute.TaskType)
}

func (s *AggregatorTestSuite) requireBlockStored(blockId scTypes.BlockId) {
	s.T().Helper()
	storedBlock, err := s.blockStorage.TryGetBlock(s.ctx, blockId)
	s.Require().NoError(err)
	s.Require().NotNil(storedBlock)
}
