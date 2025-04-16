package scheduler

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskCancelCheckerSuite struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	database    db.DB
	taskStorage *storage.TaskStorage

	logger logging.Logger
}

func TestTaskCancelCheckerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskCancelCheckerSuite))
}

func (s *TaskCancelCheckerSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database

	logger := logging.NewLogger("task_cancel_checker_test")

	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	clock := testaide.NewTestClock()

	s.taskStorage = storage.NewTaskStorage(database, clock, metricsHandler, logger)
}

func (s *TaskCancelCheckerSuite) TearDownSuite() {
	s.cancel()
}

func (s *TaskCancelCheckerSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskCancelCheckerSuite) Test_Check_Empty_Storage() {
	handlerMock := &api.TaskRequestHandlerMock{}
	cancelChecker := s.newTestTaskCancelChecker(handlerMock)

	err := cancelChecker.processRunningTasks(s.ctx)
	s.Require().NoError(err)
	s.Require().Zero(handlerMock.CheckIfTaskExistsCalls())
}

func (s *TaskCancelCheckerSuite) Test_Check_Alive_Task() {
	parentTaskId := types.NewTaskId()
	task := types.Task{
		Id:           types.NewTaskId(),
		ParentTaskId: &parentTaskId,
	}
	expectedTaskEntry := types.TaskEntry{
		Task:   task,
		Status: types.WaitingForExecutor,
	}
	err := s.taskStorage.AddTaskEntries(s.ctx, &expectedTaskEntry)
	s.Require().NoError(err)

	handlerMock := &api.TaskRequestHandlerMock{
		CheckIfTaskExistsFunc: func(contextMoqParam context.Context, request *api.TaskCheckRequest) (bool, error) {
			return true, nil
		},
	}

	cancelChecker := s.newTestTaskCancelChecker(handlerMock)

	err = cancelChecker.processRunningTasks(s.ctx)
	s.Require().NoError(err)

	// Parent task was checked
	checkTaskCall := handlerMock.CheckIfTaskExistsCalls()
	s.Require().Len(checkTaskCall, 1)
	s.Require().Equal(parentTaskId, checkTaskCall[0].Request.TaskId)
	// Checked task was not removed
	taskEntry, err := s.taskStorage.TryGetTaskEntry(s.ctx, task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskEntry)
	s.Require().Equal(taskEntry.Task.Id, expectedTaskEntry.Task.Id)
}

func (s *TaskCancelCheckerSuite) Test_Check_Dead_Task() {
	parentTaskId := types.NewTaskId()
	task := types.Task{
		Id:           types.NewTaskId(),
		ParentTaskId: &parentTaskId,
	}
	expectedTaskEntry := types.TaskEntry{
		Task:   task,
		Status: types.WaitingForExecutor,
	}
	err := s.taskStorage.AddTaskEntries(s.ctx, &expectedTaskEntry)
	s.Require().NoError(err)

	handlerMock := &api.TaskRequestHandlerMock{
		CheckIfTaskExistsFunc: func(contextMoqParam context.Context, request *api.TaskCheckRequest) (bool, error) {
			return false, nil
		},
	}

	cancelChecker := s.newTestTaskCancelChecker(handlerMock)

	err = cancelChecker.processRunningTasks(s.ctx)
	s.Require().NoError(err)

	// Parent task was checked
	checkTaskCall := handlerMock.CheckIfTaskExistsCalls()
	s.Require().Len(checkTaskCall, 1)
	s.Require().Equal(parentTaskId, checkTaskCall[0].Request.TaskId)
	// Checked task was not removed
	taskEntry, err := s.taskStorage.TryGetTaskEntry(s.ctx, task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskEntry)
	s.Require().Equal(taskEntry.Task.Id, expectedTaskEntry.Task.Id)
	s.Require().Equal(types.Failed, taskEntry.Status)
}

func (s *TaskCancelCheckerSuite) newTestTaskCancelChecker(handler api.TaskRequestHandler) *TaskCancelChecker {
	s.T().Helper()
	checker := &TaskCancelChecker{
		requestHandler: handler,
		taskSource:     s.taskStorage,
		config:         MakeDefaultCheckerConfig(),
	}

	checker.WorkerLoop = srv.NewWorkerLoop(
		"task_cancel_checker_test",
		checker.config.UpdateInterval,
		checker.runIteration,
	)
	checker.logger = srv.WorkerLogger(s.logger, checker)
	return checker
}
