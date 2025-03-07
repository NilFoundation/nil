package core

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
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
	timer := common.NewTimer()
	s.blockStorage = s.newTestBlockStorage(storage.DefaultBlockStorageConfig())
	s.taskStorage = storage.NewTaskStorage(s.db, timer, s.metrics, logger)
	s.rpcClientMock = &client.ClientMock{}

	s.aggregator = s.newTestAggregator(s.blockStorage)
}

func (s *AggregatorTestSuite) newTestAggregator(
	blockStorage AggregatorBlockStorage,
) *aggregator {
	s.T().Helper()

	logger := logging.NewLogger("aggregator_test")
	stateResetter := reset.NewStateResetter(logger, s.blockStorage)
	timer := common.NewTimer()

	return NewAggregator(
		s.rpcClientMock,
		blockStorage,
		s.taskStorage,
		stateResetter,
		timer,
		logger,
		s.metrics,
		NewDefaultAggregatorConfig(),
	)
}

func (s *AggregatorTestSuite) newTestBlockStorage(config storage.BlockStorageConfig) *storage.BlockStorage {
	s.T().Helper()
	timer := common.NewTimer()
	return storage.NewBlockStorage(s.db, config, timer, s.metrics, logging.NewLogger("aggregator_test"))
}

func (s *AggregatorTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
	s.rpcClientMock.ResetCalls()
}

func (s *AggregatorTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *AggregatorTestSuite) Test_No_New_Block_To_Fetch() {
	batch := testaide.NewBlockBatch(testaide.ShardsCount)
	err := s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	s.rpcClientMock.GetBlockFunc = func(_ context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
		if shardId == types.MainShardId {
			return batch.MainShardBlock, nil
		}

		return nil, errors.New("unexpected call of GetBlock")
	}

	err = s.aggregator.processNewBlocks(s.ctx)
	s.Require().NoError(err)

	// latest fetched block ref was not changed
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().True(mainRef.Equals(batch.MainShardBlock))

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Fetched_Not_Ready_Batch() {
	nextMainBlock := testaide.NewMainShardBlock()
	nextMainBlock.ChildBlocks[1] = common.EmptyHash

	s.setBlockGeneratorTo(nextMainBlock)

	err := s.aggregator.processNewBlocks(s.ctx)
	s.Require().NoError(err)

	// latest fetched block was not updated
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(mainRef)

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Main_Parent_Hash_Mismatch() {
	batches := testaide.NewBatchesSequence(3)

	// Set first 2 batches as proved
	for _, provedBatch := range batches[:2] {
		err := s.blockStorage.SetBlockBatch(s.ctx, provedBatch)
		s.Require().NoError(err)
		err = s.blockStorage.SetBatchAsProved(s.ctx, provedBatch.Id)
		s.Require().NoError(err)
	}

	// Set first batch as proposed, latestProvedStateRoot value is updated
	err := s.blockStorage.SetBatchAsProposed(s.ctx, batches[0].Id)
	s.Require().NoError(err)
	latestProved, err := s.blockStorage.TryGetProvedStateRoot(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestProved)

	nextMainBlock := batches[2].MainShardBlock
	nextMainBlock.ParentHash = testaide.RandomHash()
	s.rpcClientMock.GetBlockFunc = blockGenerator(nextMainBlock)

	s.rpcClientMock.GetBlocksRangeFunc = func(_ context.Context, _ types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int) ([]*jsonrpc.RPCBlock, error) {
		return []*jsonrpc.RPCBlock{nextMainBlock}, nil
	}

	err = s.aggregator.processNewBlocks(s.ctx)
	s.Require().NoError(err)

	// latest fetched block was reset
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(mainRef)
	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Fetch_At_Zero_State() {
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(mainRef)

	mainBlock := testaide.NewMainShardBlock()

	s.rpcClientMock.GetBlockFunc = blockGenerator(mainBlock)

	s.rpcClientMock.GetBlocksRangeFunc = func(_ context.Context, _ types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int) ([]*jsonrpc.RPCBlock, error) {
		if from == mainBlock.Number && to == mainBlock.Number+1 {
			return []*jsonrpc.RPCBlock{mainBlock}, nil
		}

		return nil, errors.New("unexpected call of GetBlocksRange")
	}

	err = s.aggregator.processNewBlocks(s.ctx)
	s.Require().NoError(err)
	s.requireMainBlockHandled(mainBlock)
}

func (s *AggregatorTestSuite) Test_Fetch_Next_Valid() {
	batches := testaide.NewBatchesSequence(2)
	err := s.blockStorage.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)
	nextMainBlock := batches[1].MainShardBlock

	s.setBlockGeneratorTo(nextMainBlock)

	err = s.aggregator.processNewBlocks(s.ctx)
	s.Require().NoError(err)
	s.requireMainBlockHandled(nextMainBlock)
}

func (s *AggregatorTestSuite) Test_Block_Storage_Capacity_Exceeded() {
	// only one test batch can fit in the storage
	storageConfig := storage.NewBlockStorageConfig(1)
	blockStorage := s.newTestBlockStorage(storageConfig)

	batches := testaide.NewBatchesSequence(2)
	nextMainBlock := batches[0].MainShardBlock

	s.setBlockGeneratorTo(nextMainBlock)

	agg := s.newTestAggregator(blockStorage)

	err := agg.processNewBlocks(s.ctx)
	s.Require().NoError(err)
	s.requireMainBlockHandled(nextMainBlock)

	latestFetchedBeforeNext, err := blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)

	// nextBatch should not be handled by Aggregator due to storage capacity limit
	nextBatch := batches[1]
	s.setBlockGeneratorTo(nextBatch.MainShardBlock)
	err = agg.processNewBlocks(s.ctx)
	s.Require().NoError(err)

	latestFetchedAfterNext, err := blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Equal(latestFetchedBeforeNext, latestFetchedAfterNext)

	for _, block := range nextBatch.AllBlocks() {
		storedBlock, err := s.blockStorage.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(storedBlock)
	}

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) setBlockGeneratorTo(nextMainBlock *jsonrpc.RPCBlock) {
	s.T().Helper()

	s.rpcClientMock.GetBlockFunc = blockGenerator(nextMainBlock)

	s.rpcClientMock.GetBlocksRangeFunc = func(_ context.Context, _ types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int) ([]*jsonrpc.RPCBlock, error) {
		if from == nextMainBlock.Number && to == nextMainBlock.Number+1 {
			return []*jsonrpc.RPCBlock{nextMainBlock}, nil
		}

		return nil, errors.New("unexpected call of GetBlocksRange")
	}
}

func blockGenerator(mainBlock *jsonrpc.RPCBlock) func(context.Context, types.ShardId, any, bool) (*jsonrpc.RPCBlock, error) {
	return func(_ context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error) {
		if shardId == types.MainShardId {
			return mainBlock, nil
		}

		blockHash, ok := blockId.(common.Hash)
		if !ok {
			return nil, errors.New("unexpected blockId type")
		}

		if blockHash.Empty() {
			return nil, nil
		}

		execShardBlock := testaide.NewExecutionShardBlock()
		execShardBlock.ShardId = shardId
		execShardBlock.Hash = blockHash
		return execShardBlock, nil
	}
}

// requireNoNewTasks asserts that there are no new tasks available for execution
func (s *AggregatorTestSuite) requireNoNewTasks() {
	s.T().Helper()
	task, err := s.taskStorage.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(task, "expected no new tasks available for execution, but got one")
}

func (s *AggregatorTestSuite) requireMainBlockHandled(mainBlock *jsonrpc.RPCBlock) {
	s.T().Helper()

	// latest fetched block was updated
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(mainRef)
	s.Require().True(mainRef.Equals(mainBlock))

	// main + exec block were saved to the storage
	s.requireBlockStored(scTypes.IdFromBlock(mainBlock))
	childIds, err := scTypes.ChildBlockIds(mainBlock)
	s.Require().NoError(err)
	for _, childId := range childIds {
		s.requireBlockStored(childId)
	}

	var parentTaskId scTypes.TaskId

	// one ProofBlock task per exec block was created
	for range childIds {
		taskToExecute, err := s.taskStorage.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
		s.Require().NoError(err)
		s.Require().NotNil(taskToExecute)
		s.Require().Equal(scTypes.ProofBlock, taskToExecute.TaskType)
		s.Require().NotNil(taskToExecute.ParentTaskId)
		parentTaskId = *taskToExecute.ParentTaskId
	}

	// root AggregateProofs was created
	parentTask, err := s.taskStorage.TryGetTaskEntry(s.ctx, parentTaskId)
	s.Require().NoError(err)
	s.Require().NotNil(parentTask)
	s.Require().Equal(scTypes.AggregateProofs, parentTask.Task.TaskType)
}

func (s *AggregatorTestSuite) requireBlockStored(blockId scTypes.BlockId) {
	s.T().Helper()
	storedBlock, err := s.blockStorage.TryGetBlock(s.ctx, blockId)
	s.Require().NoError(err)
	s.Require().NotNil(storedBlock)
}
