package core

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type BlockTasksIntegrationTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	db           db.DB
	taskStorage  storage.TaskStorage
	blockStorage storage.BlockStorage

	scheduler scheduler.TaskScheduler
}

func TestBlockTasksTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BlockTasksIntegrationTestSuite))
}

func (s *BlockTasksIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	logger := logging.NewLogger("block_tasks_test_suite")

	s.taskStorage = storage.NewTaskStorage(s.db, logger)
	s.blockStorage = storage.NewBlockStorage(s.db, logger)

	s.scheduler = scheduler.New(
		s.taskStorage,
		newTaskStateChangeHandler(s.blockStorage, logger),
		logger,
	)
}

func (s *BlockTasksIntegrationTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *BlockTasksIntegrationTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")

	err = s.blockStorage.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err, "failed to set proved root in SetUpTest")
}

func (s *BlockTasksIntegrationTestSuite) Test_Provide_Tasks_And_Handle_Success_Result() {
	mainBlock, childBlocks := testaide.GenerateBlockBatch(1)
	for _, block := range []*jsonrpc.RPCBlock{childBlocks[0], mainBlock} {
		err := s.blockStorage.SetBlock(s.ctx, block, mainBlock.Hash)
		s.Require().NoError(err)
	}

	batchId, err := s.blockStorage.GetOrCreateBatchId(s.ctx, mainBlock.Hash)
	s.Require().NoError(err)
	aggregateProofsTask, blockProofTask := s.generateAndSaveTasks(mainBlock, childBlocks, batchId)
	executorId := testaide.RandomExecutorId()

	// requesting next task for execution
	taskToExecute, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(blockProofTask.Task, *taskToExecute)

	// no new tasks available yet
	nonAvailableTask, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().Nil(nonAvailableTask)

	// successfully completing child block proof
	blockProofResult := successProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, &blockProofResult)
	s.Require().NoError(err)

	// proposal data should not be available yet
	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	// requesting next task for execution
	taskToExecute, err = s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(aggregateProofsTask.Task.TaskType, taskToExecute.TaskType)
	s.Require().Equal(aggregateProofsTask.Task.Id, taskToExecute.Id)

	// completing top-level aggregate proofs task
	aggregateProofsResult := successProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, &aggregateProofsResult)
	s.Require().NoError(err)

	// once top-level task is completed, proposal data for the main block should become available
	proposalData, err = s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(proposalData)
	s.Require().Equal(mainBlock.Hash, proposalData.MainShardBlockHash)
}

func (s *BlockTasksIntegrationTestSuite) Test_Provide_Tasks_And_Handle_Failure_Result() {
	mainBlock, childBlocks := testaide.GenerateBlockBatch(1)
	for _, block := range []*jsonrpc.RPCBlock{childBlocks[0], mainBlock} {
		err := s.blockStorage.SetBlock(s.ctx, block, mainBlock.Hash)
		s.Require().NoError(err)
	}

	batchId, err := s.blockStorage.GetOrCreateBatchId(s.ctx, mainBlock.Hash)
	s.Require().NoError(err)
	aggregateProofsTask, blockProofTask := s.generateAndSaveTasks(mainBlock, childBlocks, batchId)
	executorId := testaide.RandomExecutorId()

	// requesting next task for execution
	taskToExecute, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(blockProofTask.Task, *taskToExecute)

	// successfully completing child block proof
	blockProofResult := successProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, &blockProofResult)
	s.Require().NoError(err)

	// requesting next task for execution
	taskToExecute, err = s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(aggregateProofsTask.Task.TaskType, taskToExecute.TaskType)
	s.Require().Equal(aggregateProofsTask.Task.Id, taskToExecute.Id)

	// setting top-level task as failed
	aggregateProofsFailed := types.FailureProviderTaskResult(
		taskToExecute.Id,
		executorId,
		errors.New("something went wrong"),
	)

	err = s.scheduler.SetTaskResult(s.ctx, &aggregateProofsFailed)
	s.Require().NoError(err)

	// proposal data should not become available
	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	// status for AggregateProofs task should be updated
	aggregateEntry, err := s.taskStorage.TryGetTaskEntry(s.ctx, aggregateProofsTask.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(aggregateEntry)
	s.Require().Equal(types.Failed, aggregateEntry.Status)
}

func successProviderResult(taskToExecute *types.Task, executorId types.TaskExecutorId) types.TaskResult {
	return types.SuccessProviderTaskResult(
		taskToExecute.Id,
		executorId,
		taskToExecute.TaskType,
		types.TaskResultAddresses{},
		types.TaskResultData{},
	)
}

func (s *BlockTasksIntegrationTestSuite) generateAndSaveTasks(
	mainBlock *jsonrpc.RPCBlock,
	childBlocks []*jsonrpc.RPCBlock,
	batchId types.BatchId,
) (aggregateProofsTask *types.TaskEntry, blockProofTask *types.TaskEntry) {
	s.T().Helper()

	aggregateProofsTask = types.NewAggregateBlockProofsTaskEntry(
		batchId, coreTypes.MainShardId, mainBlock.Number, mainBlock.Hash, 1,
	)
	aggregateProofsTask.Task.DependencyNum = 1
	aggregateProofsTask.Status = types.WaitingForInput

	blockProofTask = types.NewBlockProofTaskEntry(
		batchId, &aggregateProofsTask.Task.Id, childBlocks[0].Hash,
	)
	blockProofTask.PendingDeps = []types.TaskId{aggregateProofsTask.Task.Id}

	err := s.taskStorage.AddTaskEntries(s.ctx, []*types.TaskEntry{aggregateProofsTask, blockProofTask})
	s.Require().NoError(err)
	return aggregateProofsTask, blockProofTask
}
