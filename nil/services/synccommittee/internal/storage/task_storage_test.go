package storage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

const (
	degreeOfParallelism = 10
)

type TaskStorageSuite struct {
	suite.Suite
	database db.DB
	timer    common.Timer
	ts       TaskStorage
	ctx      context.Context
}

func TestTaskStorageSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskStorageSuite))
}

func (s *TaskStorageSuite) SetupSuite() {
	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database
	logger := logging.NewLogger("task_storage_test")

	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	s.timer = testaide.NewTestTimer()
	s.ts = NewTaskStorage(database, s.timer, metricsHandler, logger)
	s.ctx = context.Background()
}

func (s *TaskStorageSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskStorageSuite) TestRequestAndProcessResult() {
	now := s.timer.NowTime()

	// Initialize two tasks waiting for input
	lowerPriorityEntry := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)
	lowerPriorityEntry.Task.BlockNum = 222

	higherPriorityEntry := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)
	higherPriorityEntry.Task.BlockNum = 14

	// Initialize two corresponding dependencies for them which are running
	dependency1 := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	lowerPriorityEntry.AddDependency(dependency1)

	dependency2 := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	higherPriorityEntry.AddDependency(dependency2)

	err := s.ts.AddTaskEntries(s.ctx, []*types.TaskEntry{
		lowerPriorityEntry,
		higherPriorityEntry,
		dependency1,
		dependency2,
	})

	s.Require().NoError(err)

	// No available tasks for executor at this point
	task, err := s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Nil(task)

	// Make lower priority task ready for execution
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.NewSuccessProverTaskResult(dependency1.Task.Id, dependency1.Owner, types.TaskOutputArtifacts{}, types.TaskResultData{}),
	)
	s.Require().NoError(err)
	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Equal(task.Id, lowerPriorityEntry.Task.Id)

	// Make higher priority task ready
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.NewSuccessProverTaskResult(dependency2.Task.Id, dependency2.Owner, types.TaskOutputArtifacts{}, types.TaskResultData{}),
	)
	s.Require().NoError(err)

	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Equal(task.Id, higherPriorityEntry.Task.Id)
}

func (s *TaskStorageSuite) TestTaskRescheduling_NoEntries() {
	executionTimeout := time.Minute
	err := s.ts.RescheduleHangingTasks(s.ctx, executionTimeout)
	s.Require().NoError(err)

	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) TestTaskRescheduling_NoActiveTasks() {
	now := s.timer.NowTime()
	executionTimeout := time.Minute

	entries := []*types.TaskEntry{
		testaide.NewTaskEntry(now.Add(-time.Second), types.WaitingForExecutor, types.UnknownExecutorId),
		testaide.NewTaskEntry(now.Add(-time.Hour*24), types.WaitingForExecutor, types.UnknownExecutorId),
	}

	err := s.ts.AddTaskEntries(s.ctx, entries)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, executionTimeout)
	s.Require().NoError(err)

	// All existing tasks are still available for execution
	for range entries {
		taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
		s.Require().NoError(err)
		s.Require().NotNil(taskToExecute)
	}
}

func (s *TaskStorageSuite) TestTaskRescheduling_SingleActiveTask() {
	now := s.timer.NowTime()
	executionTimeout := time.Minute

	activeEntry := testaide.NewTaskEntry(now.Add(-time.Second), types.Running, testaide.RandomExecutorId())

	err := s.ts.AddSingleTaskEntry(s.ctx, *activeEntry)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, executionTimeout)
	s.Require().NoError(err)

	// Active task wasn't rescheduled
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) TestTaskRescheduling_MultipleTasks() {
	now := s.timer.NowTime()
	executionTimeout := time.Minute

	outdatedEntry := testaide.NewTaskEntry(now.Add(-executionTimeout*2), types.Running, testaide.RandomExecutorId())

	err := s.ts.AddTaskEntries(s.ctx, []*types.TaskEntry{
		outdatedEntry,
		testaide.NewTaskEntry(now.Add(-time.Second), types.Running, testaide.RandomExecutorId()),
		testaide.NewTaskEntry(now.Add(-time.Second*2), types.Running, testaide.RandomExecutorId()),
		testaide.NewTaskEntry(now.Add(-time.Hour*2), types.Failed, testaide.RandomExecutorId()),
	})
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, executionTimeout)
	s.Require().NoError(err)

	// Outdated task was rescheduled and became available for execution
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(outdatedEntry.Task, *taskToExecute)

	// Active and failed tasks weren't rescheduled
	taskToExecute, err = s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) Test_AddSingleTaskEntry_Concurrently() {
	now := s.timer.NowTime()

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			entry := testaide.NewTaskEntry(now, types.WaitingForExecutor, types.UnknownExecutorId)
			err := s.ts.AddSingleTaskEntry(s.ctx, *entry)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()

	s.requireExactTasksCount(degreeOfParallelism)
}

func (s *TaskStorageSuite) Test_AddTaskEntries_Concurrently() {
	now := s.timer.NowTime()

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)
	const tasksPerWorker = 3

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			var entries []*types.TaskEntry
			for range tasksPerWorker {
				randomEntry := testaide.NewTaskEntry(now, types.WaitingForExecutor, types.UnknownExecutorId)
				entries = append(entries, randomEntry)
			}
			err := s.ts.AddTaskEntries(s.ctx, entries)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()

	s.requireExactTasksCount(degreeOfParallelism * tasksPerWorker)
}

func (s *TaskStorageSuite) requireExactTasksCount(tasksCount int) {
	s.T().Helper()

	// All added tasks became available
	for range tasksCount {
		task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
		s.Require().NoError(err)
		s.Require().NotNil(task)
	}

	// There no more tasks left
	task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_RequestTaskToExecute_Concurrently() {
	now := s.timer.NowTime()

	entry := testaide.NewTaskEntry(now, types.WaitingForExecutor, types.UnknownExecutorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *entry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	receivedTaskCount := atomic.Uint32{}

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
			s.NoError(err)

			if task != nil {
				s.Equal(entry.Task, *task)
				receivedTaskCount.Add(1)
			}
		}()
	}

	waitGroup.Wait()
	s.Require().Equal(uint32(1), receivedTaskCount.Load(), "expected only one executor to receive task")
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_Concurrently() {
	now := s.timer.NowTime()

	executorId := testaide.RandomExecutorId()
	runningEntry := testaide.NewTaskEntry(now, types.Running, executorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *runningEntry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			err := s.ts.ProcessTaskResult(
				s.ctx,
				types.NewSuccessProverTaskResult(runningEntry.Task.Id, executorId, types.TaskOutputArtifacts{}, types.TaskResultData{}),
			)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()

	// Task was successfully completed and was removed from the storage
	task, err := s.ts.RequestTaskToExecute(s.ctx, executorId)
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_InvalidStateChange() {
	testCases := []struct {
		name      string
		oldStatus types.TaskStatus
	}{
		{"WaitingForInput", types.WaitingForInput},
		{"WaitingForExecutor", types.WaitingForExecutor},
		{"Failed", types.Failed},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name+"_TrySetSuccess", func() {
			s.tryToChangeStatus(testCase.oldStatus, true, false, types.ErrTaskInvalidStatus)
		})
		s.Run(testCase.name+"_TrySetFailure", func() {
			s.tryToChangeStatus(testCase.oldStatus, false, false, types.ErrTaskInvalidStatus)
		})
	}
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_WrongExecutor() {
	s.Run("TrySetSuccess", func() {
		s.tryToChangeStatus(types.Running, true, true, types.ErrTaskWrongExecutor)
	})
	s.Run("TrySetFailure", func() {
		s.tryToChangeStatus(types.Running, false, true, types.ErrTaskWrongExecutor)
	})
}

func (s *TaskStorageSuite) tryToChangeStatus(
	oldStatus types.TaskStatus,
	trySetSuccess bool,
	useDifferentExecutorId bool,
	expectedError error,
) {
	s.T().Helper()

	now := s.timer.NowTime()
	executorId := testaide.RandomExecutorId()
	taskEntry := testaide.NewTaskEntry(now, oldStatus, executorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *taskEntry)
	s.Require().NoError(err)

	if useDifferentExecutorId {
		executorId = testaide.RandomExecutorId()
	}

	var taskResult *types.TaskResult
	if trySetSuccess {
		taskResult = types.NewSuccessProverTaskResult(taskEntry.Task.Id, executorId, types.TaskOutputArtifacts{}, types.TaskResultData{})
	} else {
		taskResult = types.NewFailureProverTaskResult(taskEntry.Task.Id, executorId, errors.New("some error"))
	}

	err = s.ts.ProcessTaskResult(s.ctx, taskResult)
	s.Require().ErrorIs(err, expectedError)
}
