package core

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
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
	timer        common.Timer
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

	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	logger := logging.NewLogger("block_tasks_test_suite")

	s.timer = testaide.NewTestTimer()
	s.taskStorage = storage.NewTaskStorage(s.db, s.timer, metricsHandler, logger)
	s.blockStorage = storage.NewBlockStorage(s.db, s.timer, metricsHandler, logger)

	s.scheduler = scheduler.New(
		s.taskStorage,
		newTaskStateChangeHandler(s.blockStorage, logger),
		metricsHandler,
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
	batch := testaide.NewBlockBatch(1)
	err := s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	proofTasks, err := batch.CreateProofTasks(s.timer.NowTime())
	s.Require().NoError(err)

	err = s.taskStorage.AddTaskEntries(s.ctx, proofTasks)
	s.Require().NoError(err)

	executorId := testaide.RandomExecutorId()

	// requesting next task for execution
	taskToExecute, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(types.ProofBlock, taskToExecute.TaskType)

	// no new tasks available yet
	nonAvailableTask, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().Nil(nonAvailableTask)

	// successfully completing child block proof
	blockProofResult := newTestSuccessProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, blockProofResult)
	s.Require().NoError(err)

	// proposal data should not be available yet
	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	// requesting next task for execution
	taskToExecute, err = s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(types.AggregateProofs, taskToExecute.TaskType)

	// completing top-level aggregate proofs task
	aggregateProofsResult := newTestSuccessProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, aggregateProofsResult)
	s.Require().NoError(err)

	// once top-level task is completed, proposal data for the main block should become available
	proposalData, err = s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(proposalData)
	s.Require().Equal(batch.MainShardBlock.Hash, proposalData.MainShardBlockHash)
}

func (s *BlockTasksIntegrationTestSuite) Test_Provide_Tasks_And_Handle_Failure_Result() {
	batch := testaide.NewBlockBatch(1)
	err := s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	proofTasks, err := batch.CreateProofTasks(s.timer.NowTime())
	s.Require().NoError(err)

	err = s.taskStorage.AddTaskEntries(s.ctx, proofTasks)
	s.Require().NoError(err)

	executorId := testaide.RandomExecutorId()

	// requesting next task for execution
	taskToExecute, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(types.ProofBlock, taskToExecute.TaskType)

	// successfully completing child block proof
	blockProofResult := newTestSuccessProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, blockProofResult)
	s.Require().NoError(err)

	// requesting next task for execution
	taskToExecute, err = s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(types.AggregateProofs, taskToExecute.TaskType)

	// setting top-level task as failed
	aggregateProofsFailed := types.NewFailureProviderTaskResult(
		taskToExecute.Id,
		executorId,
		errors.New("something went wrong"),
	)

	err = s.scheduler.SetTaskResult(s.ctx, aggregateProofsFailed)
	s.Require().NoError(err)

	// proposal data should not become available
	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	// status for AggregateProofs task should be updated
	aggregateEntry, err := s.taskStorage.TryGetTaskEntry(s.ctx, taskToExecute.Id)
	s.Require().NoError(err)
	s.Require().NotNil(aggregateEntry)
	s.Require().Equal(types.Failed, aggregateEntry.Status)
}

func newTestSuccessProviderResult(taskToExecute *types.Task, executorId types.TaskExecutorId) *types.TaskResult {
	return types.NewSuccessProviderTaskResult(
		taskToExecute.Id,
		executorId,
		types.TaskOutputArtifacts{},
		types.TaskResultData{},
	)
}
