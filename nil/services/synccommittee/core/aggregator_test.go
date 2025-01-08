package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
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

	db           db.DB
	blockStorage storage.BlockStorage
	taskStorage  storage.TaskStorage

	rpcClientMock *client.ClientMock
	aggregator    *Aggregator
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

	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	timer := common.NewTimer()
	s.blockStorage = storage.NewBlockStorage(s.db, timer, metricsHandler, logger)
	s.taskStorage = storage.NewTaskStorage(s.db, timer, metricsHandler, logger)

	s.rpcClientMock = &client.ClientMock{}

	s.aggregator, err = NewAggregator(
		s.rpcClientMock,
		s.blockStorage,
		s.taskStorage,
		timer,
		logger,
		metricsHandler,
		time.Second,
	)
	s.Require().NoError(err)
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
	mainBlock := testaide.NewMainShardBlock()
	mainBlock.ChildBlocks[1] = common.EmptyHash

	s.rpcClientMock.GetBlockFunc = blockGenerator(mainBlock)

	s.rpcClientMock.GetBlocksRangeFunc = func(_ context.Context, _ types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int) ([]*jsonrpc.RPCBlock, error) {
		if from == mainBlock.Number && to == mainBlock.Number+1 {
			return []*jsonrpc.RPCBlock{mainBlock}, nil
		}

		return nil, errors.New("unexpected call of GetBlocksRange")
	}

	err := s.aggregator.processNewBlocks(s.ctx)
	s.Require().ErrorIs(err, scTypes.ErrBatchNotReady)

	// latest fetched block was not updated
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(mainRef)

	s.requireNoNewTasks()
}

func (s *AggregatorTestSuite) Test_Main_Parent_Hash_Mismatch() {
	batches := testaide.NewBatchesSequence(2)
	err := s.blockStorage.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)

	nextMainBlock := batches[1].MainShardBlock
	nextMainBlock.ParentHash = testaide.RandomHash()

	s.rpcClientMock.GetBlockFunc = blockGenerator(nextMainBlock)

	s.rpcClientMock.GetBlocksRangeFunc = func(_ context.Context, _ types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int) ([]*jsonrpc.RPCBlock, error) {
		return []*jsonrpc.RPCBlock{nextMainBlock}, nil
	}

	err = s.aggregator.processNewBlocks(s.ctx)
	s.Require().ErrorIs(err, scTypes.ErrBlockMismatch)

	// latest fetched block was not updated
	mainRef, err := s.blockStorage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().True(mainRef.Equals(batches[0].MainShardBlock))

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

	s.rpcClientMock.GetBlockFunc = blockGenerator(nextMainBlock)

	s.rpcClientMock.GetBlocksRangeFunc = func(_ context.Context, _ types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int) ([]*jsonrpc.RPCBlock, error) {
		if from == nextMainBlock.Number && to == nextMainBlock.Number+1 {
			return []*jsonrpc.RPCBlock{nextMainBlock}, nil
		}

		return nil, errors.New("unexpected call of GetBlocksRange")
	}

	err = s.aggregator.processNewBlocks(s.ctx)
	s.Require().NoError(err)
	s.requireMainBlockHandled(nextMainBlock)
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
	s.Require().Nil(task)
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
