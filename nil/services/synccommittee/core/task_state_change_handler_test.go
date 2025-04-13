package core

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type TaskStateChangeHandlerTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	db           db.DB
	blockStorage *storage.BlockStorage

	resetLauncher *StateResetLauncherMock
	handler       api.TaskStateChangeHandler
}

func TestTaskStateChangeHandlerTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskStateChangeHandlerTestSuite))
}

func (s *TaskStateChangeHandlerTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	logger := logging.NewLogger("task_state_change_handler_test")
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	clock := clockwork.NewRealClock()
	s.blockStorage = storage.NewBlockStorage(s.db, storage.DefaultBlockStorageConfig(), clock, metricsHandler, logger)

	s.resetLauncher = &StateResetLauncherMock{}
	s.handler = newTaskStateChangeHandler(s.blockStorage, s.resetLauncher, logger)
}

func (s *TaskStateChangeHandlerTestSuite) SetupTest() {
	s.resetLauncher.ResetCalls()
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
}

func (s *TaskStateChangeHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *TaskStateChangeHandlerTestSuite) Test_OnTaskTerminated_Success() {
	task, batch := s.batchTaskSetUp()
	result := testaide.NewSuccessTaskResult(task.Id, testaide.RandomExecutorId())

	err := s.handler.OnTaskTerminated(s.ctx, task, result)
	s.Require().NoError(err)

	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(batch.LatestMainBlock().Hash, proposalData.NewProvedStateRoot)

	resetCalls := s.resetLauncher.LaunchPartialResetWithSuspensionCalls()
	s.Require().Empty(resetCalls)
}

func (s *TaskStateChangeHandlerTestSuite) Test_OnTaskTerminated_Retryable_Error() {
	task, _ := s.batchTaskSetUp()
	result := testaide.NewRetryableErrorTaskResult(task.Id, testaide.RandomExecutorId())

	err := s.handler.OnTaskTerminated(s.ctx, task, result)
	s.Require().NoError(err)

	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	resetCalls := s.resetLauncher.LaunchPartialResetWithSuspensionCalls()
	s.Require().Empty(resetCalls)
}

func (s *TaskStateChangeHandlerTestSuite) Test_OnTaskTerminated_Non_Retryable_Error() {
	task, _ := s.batchTaskSetUp()
	result := testaide.NewNonRetryableErrorTaskResult(task.Id, testaide.RandomExecutorId())

	err := s.handler.OnTaskTerminated(s.ctx, task, result)
	s.Require().NoError(err)

	proposalData, err := s.blockStorage.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(proposalData)

	resetCalls := s.resetLauncher.LaunchPartialResetWithSuspensionCalls()
	s.Require().Len(resetCalls, 1)
}

func (s *TaskStateChangeHandlerTestSuite) batchTaskSetUp() (*types.Task, *types.BlockBatch) {
	s.T().Helper()

	batch := testaide.NewBlockBatch(10)

	err := s.blockStorage.SetProvedStateRoot(s.ctx, batch.FirstMainBlock().ParentHash)
	s.Require().NoError(err)

	err = s.blockStorage.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	task := testaide.NewTaskOfType(types.ProofBatch)
	task.BatchId = batch.Id
	return task, batch
}

func (s *TaskStateChangeHandlerTestSuite) Test_OnTaskTerminated_Unknown_Batch() {
	task := testaide.NewTaskOfType(types.ProofBatch)
	task.BatchId = types.NewBatchId()
	executor := testaide.RandomExecutorId()

	testCases := []struct {
		name   string
		result *types.TaskResult
	}{
		{
			"Success", testaide.NewSuccessTaskResult(task.Id, executor),
		},
		{
			"Retryable_Error", testaide.NewRetryableErrorTaskResult(task.Id, executor),
		},
		{
			"Critical_Error", testaide.NewNonRetryableErrorTaskResult(task.Id, executor),
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			err := s.handler.OnTaskTerminated(s.ctx, task, testCase.result)
			s.Require().NoError(err)

			resetCalls := s.resetLauncher.LaunchPartialResetWithSuspensionCalls()
			s.Require().Empty(resetCalls)
		})
	}
}
