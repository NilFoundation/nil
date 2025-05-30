package fetching

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/constraints"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/syncer"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
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

	batchConstraints := constraints.NewBatchConstraints(time.Minute, 100)
	s.aggregator = s.newTestAggregator(s.blockStorage, batchConstraints)
}

func (s *AggregatorTestSuite) newTestAggregator(
	blockStorage *storage.BlockStorage,
	batchConstraints constraints.BatchConstraints,
) *aggregator {
	s.T().Helper()

	logger := logging.NewLogger("aggregator_test")
	clock := clockwork.NewRealClock()

	contractWrapperConfig := rollupcontract.WrapperConfig{
		DisableL1: true,
	}
	contractWrapper, err := rollupcontract.NewWrapper(s.ctx, contractWrapperConfig, logger)
	s.Require().NoError(err)

	committer := batches.NewCommitter(
		contractWrapper, clock, batches.DefaultCommitConfig(), s.metrics, logger,
	)

	fetcher := NewFetcher(s.rpcClientMock, logger)
	stateRootSyncer := syncer.NewStateRootSyncer(
		fetcher, contractWrapper, s.blockStorage, logger, syncer.NewDefaultConfig(),
	)
	resetLauncher := reset.NewResetLauncher(s.blockStorage, stateRootSyncer, nil, logger)

	batchChecker := constraints.NewChecker(
		batchConstraints,
		clock,
		logger,
	)

	return NewAggregator(
		fetcher,
		batchChecker,
		blockStorage,
		s.taskStorage,
		committer,
		resetLauncher,
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
	batch, err := testaide.NewBlockBatch(testaide.ShardsCount).Seal(testaide.NewDataProofs(), testaide.Now)
	s.Require().NoError(err)
	err = s.blockStorage.PutBlockBatch(s.ctx, batch)
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
	batchesSeq := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, batchesSeq)
	err := s.blockStorage.SetProvedStateRoot(s.ctx, batchesSeq[0].EarliestMainBlock().ParentHash)
	s.Require().NoError(err)

	// Set first 2 batches as sealed and proved
	for _, provedBatch := range batchesSeq[:2] {
		sealedBatch, err := provedBatch.Seal(testaide.NewDataProofs(), testaide.Now)
		s.Require().NoError(err)

		err = s.blockStorage.PutBlockBatch(s.ctx, sealedBatch)
		s.Require().NoError(err)
		err = s.blockStorage.SetBatchAsProved(s.ctx, sealedBatch.Id)
		s.Require().NoError(err)
	}

	// Set first batch as proposed, latestProvedStateRoot value is updated
	err = s.blockStorage.SetBatchAsProposed(s.ctx, batchesSeq[0].Id)
	s.Require().NoError(err)
	latestProved, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestProved)
	s.Require().Equal(batchesSeq[0].LatestMainBlock().Hash, *latestProved)

	nextMainBlock := batchesSeq[2].LatestMainBlock()
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

	batchesSeq := testaide.NewBatchesSequence(2)
	err = s.blockStorage.SetProvedStateRoot(s.ctx, batchesSeq[0].LatestMainBlock().Hash)
	s.Require().NoError(err)

	testaide.ClientMockSetBatches(s.rpcClientMock, batchesSeq)

	// batch is expected to be sealed
	batchConstraints := constraints.DefaultBatchConstraints()
	maxBlocksCount := uint32(batchesSeq[0].Blocks.BlocksCount() + batchesSeq[1].Blocks.BlocksCount() - 1)
	batchConstraints.MaxBlocksCount = maxBlocksCount
	agg := s.newTestAggregator(s.blockStorage, batchConstraints)

	err = agg.processBlocksAndHandleErr(s.ctx)
	s.Require().NoError(err)
	s.requireBatchHandled(batchesSeq[1])
}

func (s *AggregatorTestSuite) Test_Fetch_Next_Valid() {
	batchesSeq := testaide.NewBatchesSequence(2)

	sealedBatch, err := batchesSeq[0].Seal(testaide.NewDataProofs(), testaide.Now)
	s.Require().NoError(err)
	err = s.blockStorage.PutBlockBatch(s.ctx, sealedBatch)
	s.Require().NoError(err)

	testaide.ClientMockSetBatches(s.rpcClientMock, batchesSeq)

	batchConstraints := constraints.DefaultBatchConstraints()
	batchConstraints.MaxBlocksCount = uint32(batchesSeq[1].Blocks.BlocksCount())
	agg := s.newTestAggregator(s.blockStorage, batchConstraints)

	err = agg.processBlocksAndHandleErr(s.ctx)
	s.Require().NoError(err)
	s.requireBatchHandled(batchesSeq[1])
}

func (s *AggregatorTestSuite) Test_Block_Storage_Capacity_Exceeded() {
	// only one batch can fit in the storage
	storageConfig := storage.NewBlockStorageConfig(1)
	blockStorage := s.newTestBlockStorage(storageConfig)

	batchesSeq := testaide.NewBatchesSequence(2)
	testaide.ClientMockSetBatches(s.rpcClientMock, batchesSeq)

	// sealed batch cannot be further extended
	sealedBatch, err := batchesSeq[0].Seal(testaide.NewDataProofs(), testaide.Now)
	s.Require().NoError(err)
	err = blockStorage.PutBlockBatch(s.ctx, sealedBatch)
	s.Require().NoError(err)

	agg := s.newTestAggregator(blockStorage, constraints.DefaultBatchConstraints())

	latestFetchedBeforeNext, err := blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetchedBeforeNext)

	err = agg.processBlockRange(s.ctx)
	s.Require().NoError(err)

	latestFetchedAfterNext, err := blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Equal(latestFetchedBeforeNext, latestFetchedAfterNext)

	// nextBatch should not be handled by Aggregator due to storage capacity limit
	nextBatch := batchesSeq[1]

	for block := range nextBatch.BlocksIter() {
		storedBlock, err := s.blockStorage.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(storedBlock)
	}

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_State_Root_Is_Not_Initialized() {
	err := s.aggregator.processBlockRange(s.ctx)
	s.Require().ErrorIs(err, scTypes.ErrLocalStateRootNotInitialized)

	latestFetched, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(latestFetched)

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Latest_Fetched_Does_Not_Exist_On_Chain() {
	batchesSeq := testaide.NewBatchesSequence(3)

	err := s.blockStorage.SetProvedStateRoot(s.ctx, batchesSeq[0].LatestMainBlock().Hash)
	s.Require().NoError(err)

	testaide.ClientMockSetBatches(s.rpcClientMock, batchesSeq)

	err = s.aggregator.processBlockRange(s.ctx)
	s.Require().NoError(err)

	latestFetched, err := s.blockStorage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetched)
	s.Require().Equal(batchesSeq[len(batchesSeq)-1].LatestMainBlock().Hash, latestFetched.TryGetMain().Hash)

	// emulating L2 reset
	newBatches := testaide.NewBatchesSequence(3)
	testaide.ClientMockSetBatches(s.rpcClientMock, newBatches)

	err = s.aggregator.processBlockRange(s.ctx)
	s.Require().ErrorIs(err, scTypes.ErrBlockMismatch)
	s.Require().ErrorContains(err, "block not found on L2 side")
}

// requireNoNewTasks asserts that there are no new tasks available for execution
func (s *AggregatorTestSuite) requireNoNewTasks() {
	s.T().Helper()
	task, err := s.taskStorage.RequestTaskToExecute(s.ctx, scTypes.NewRandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(task, "expected no new tasks available for execution, but got one")
}

func (s *AggregatorTestSuite) requireBatchHandled(batch *scTypes.BlockBatch) {
	s.T().Helper()
	mainBlock := batch.LatestMainBlock()

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
	taskToExecute, err := s.taskStorage.RequestTaskToExecute(s.ctx, scTypes.NewRandomExecutorId())
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
