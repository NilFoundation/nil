package core

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type BlockTasksIntegrationTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	db    db.DB
	clock clockwork.Clock

	taskStorage  *storage.TaskStorage
	blockStorage *storage.BlockStorage

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

	s.clock = testaide.NewTestClock()
	s.taskStorage = storage.NewTaskStorage(s.db, s.clock, metricsHandler, logger)
	s.blockStorage = storage.NewBlockStorage(s.db, storage.DefaultBlockStorageConfig(), s.clock, metricsHandler, logger)

	s.scheduler = scheduler.New(
		s.taskStorage,
		newTaskStateChangeHandler(s.blockStorage, &StateResetLauncherMock{}, logger),
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
}

func (s *BlockTasksIntegrationTestSuite) Test_Provide_Tasks_And_Handle_Success_Result() {
	batch := testaide.NewBlockBatch(1)

	err := s.blockStorage.SetProvedStateRoot(s.ctx, batch.EarliestMainBlock().ParentHash)
	s.Require().NoError(err)

	err = s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	proofTask, err := batch.CreateProofTask(s.clock.Now())
	s.Require().NoError(err)

	err = s.taskStorage.AddTaskEntries(s.ctx, proofTask)
	s.Require().NoError(err)

	executorId := testaide.RandomExecutorId()

	// requesting batch proof task for execution
	taskToExecute, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(types.ProofBatch, taskToExecute.TaskType)

	// no new tasks available yet
	nonAvailableTask, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().Nil(nonAvailableTask)

	// successfully completing batch proof task
	batchProofResult := newTestSuccessProviderResult(taskToExecute, executorId)
	err = s.scheduler.SetTaskResult(s.ctx, batchProofResult)
	s.Require().NoError(err)

	// once top-level task is completed, proposal data for the main block should become available
	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(proposalData)
	s.Require().Equal(batch.LatestMainBlock().Hash, proposalData.NewProvedStateRoot)
}

func (s *BlockTasksIntegrationTestSuite) Test_Provide_Tasks_And_Handle_Failure_Result() {
	batch := testaide.NewBlockBatch(1)

	err := s.blockStorage.SetProvedStateRoot(s.ctx, batch.EarliestMainBlock().ParentHash)
	s.Require().NoError(err)

	err = s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	proofTask, err := batch.CreateProofTask(s.clock.Now())
	s.Require().NoError(err)

	err = s.taskStorage.AddTaskEntries(s.ctx, proofTask)
	s.Require().NoError(err)

	executorId := testaide.RandomExecutorId()

	// requesting batch proof task
	taskToExecute, err := s.scheduler.GetTask(s.ctx, api.NewTaskRequest(executorId))
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(types.ProofBatch, taskToExecute.TaskType)

	// setting batch proof task as failed
	batchProofFailed := types.NewFailureProviderTaskResult(
		taskToExecute.Id,
		executorId,
		types.NewTaskExecError(types.TaskErrProofGenerationFailed, "batch proof generation failed"),
	)

	err = s.scheduler.SetTaskResult(s.ctx, batchProofFailed)
	s.Require().NoError(err)

	// proposal data should not become available
	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	// status for ProofBatch task should be updated
	batchProofEntry, err := s.taskStorage.TryGetTaskEntry(s.ctx, taskToExecute.Id)
	s.Require().NoError(err)
	s.Require().NotNil(batchProofEntry)
	s.Require().Equal(types.Failed, batchProofEntry.Status)
}

func newTestSuccessProviderResult(taskToExecute *types.Task, executorId types.TaskExecutorId) *types.TaskResult {
	return types.NewSuccessProviderTaskResult(
		taskToExecute.Id,
		executorId,
		types.TaskOutputArtifacts{},
		types.TaskResultData{},
	)
}
