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
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear database in SetUpTest")
}

func (s *TaskStateChangeHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *TaskStateChangeHandlerTestSuite) Test_Handle_Unknown_Batch() {
	task := testaide.NewTaskOfType(types.ProofBatch)
	task.BatchId = types.NewBatchId()
	result := testaide.NewSuccessTaskResult(task.Id, testaide.RandomExecutorId())

	err := s.handler.OnTaskTerminated(s.ctx, task, result)
	s.Require().NoError(err)

	resetCalls := s.resetLauncher.LaunchPartialResetWithSuspensionCalls()
	s.Require().Empty(resetCalls)
}
