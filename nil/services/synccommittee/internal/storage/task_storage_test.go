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
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
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
	ts       TaskStorage
	ctx      context.Context

	baseTask      types.Task
	baseTaskEntry types.TaskEntry
}

func TestTaskStorageSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskStorageSuite))
}

func (s *TaskStorageSuite) SetupSuite() {
	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database
	logger := logging.NewLogger("task-storage-test")
	s.ts = NewTaskStorage(database, logger)
	s.ctx = context.Background()

	s.baseTask = types.Task{
		Id:            types.NewTaskId(),
		BatchNum:      1,
		ShardId:       coreTypes.MainShardId,
		BlockNum:      1,
		BlockHash:     common.EmptyHash,
		TaskType:      types.PartialProve,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.TaskId]types.TaskResult),
		DependencyNum: 0,
	}
	s.baseTaskEntry = types.TaskEntry{
		Task:     s.baseTask,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   types.WaitingForExecutor,
	}
}

func (s *TaskStorageSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskStorageSuite) TestAddRemove() {
	modifiedEntry := s.baseTaskEntry
	modifiedEntry.Task.Id = types.NewTaskId()
	modifiedEntry.Task.BatchNum = 8

	err := s.ts.AddSingleTaskEntry(s.ctx, s.baseTaskEntry)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, modifiedEntry)
	s.Require().NoError(err)

	err = s.ts.RemoveTaskEntry(s.ctx, s.baseTask.Id)
	s.Require().NoError(err)
}

func (s *TaskStorageSuite) TestRequestAndProcessResult() {
	// Initialize two tasks waiting for input
	lowerPriorityEntry := s.baseTaskEntry
	lowerPriorityEntry.Task.Id = types.NewTaskId()
	lowerPriorityEntry.Task.BlockNum = 222
	lowerPriorityEntry.Status = types.WaitingForInput
	lowerPriorityEntry.Task.DependencyNum = 1

	higherPriorityEntry := s.baseTaskEntry
	higherPriorityEntry.Task.Id = types.NewTaskId()
	higherPriorityEntry.Task.BlockNum = 14
	higherPriorityEntry.Status = types.WaitingForInput
	higherPriorityEntry.Task.DependencyNum = 1

	// Initialize two corresponding dependencies for them which are running
	dependency1 := s.baseTaskEntry
	dependency1.Task.Id = types.NewTaskId()
	dependency1.PendingDeps = []types.TaskId{lowerPriorityEntry.Task.Id}
	dependency1.Status = types.Running
	dependency2 := s.baseTaskEntry
	dependency2.PendingDeps = []types.TaskId{higherPriorityEntry.Task.Id}
	dependency2.Task.Id = types.NewTaskId()
	dependency2.Status = types.Running
	err := s.ts.AddSingleTaskEntry(s.ctx, lowerPriorityEntry)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, higherPriorityEntry)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, dependency1)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, dependency2)
	s.Require().NoError(err)

	// No available tasks for executor at this point
	task, err := s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Nil(task)

	// Make lower priority task ready for execution
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.SuccessProverTaskResult(dependency1.Task.Id, dependency1.Owner, types.PartialProve, types.TaskResultFiles{}),
	)
	s.Require().NoError(err)
	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Equal(task.Id, lowerPriorityEntry.Task.Id)

	// Make higher priority task ready
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.SuccessProverTaskResult(dependency2.Task.Id, dependency2.Owner, types.PartialProve, types.TaskResultFiles{}),
	)
	s.Require().NoError(err)

	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Equal(task.Id, higherPriorityEntry.Task.Id)
}

func (s *TaskStorageSuite) TestTaskRescheduling_NoEntries() {
	executionTimeout := time.Minute
	err := s.ts.RescheduleHangingTasks(s.ctx, time.Now(), executionTimeout)
	s.Require().NoError(err)

	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) TestTaskRescheduling_NoActiveTasks() {
	currentTime := time.Now()
	executionTimeout := time.Minute

	entries := []*types.TaskEntry{
		testaide.GenerateTaskEntry(currentTime.Add(-time.Second), types.WaitingForExecutor, types.UnknownExecutorId),
		testaide.GenerateTaskEntry(currentTime.Add(-time.Hour*24), types.WaitingForExecutor, types.UnknownExecutorId),
	}

	err := s.ts.AddTaskEntries(s.ctx, entries)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, currentTime, executionTimeout)
	s.Require().NoError(err)

	// All existing tasks are still available for execution
	for range entries {
		taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
		s.Require().NoError(err)
		s.Require().NotNil(taskToExecute)
	}
}

func (s *TaskStorageSuite) TestTaskRescheduling_SingleActiveTask() {
	currentTime := time.Now()
	executionTimeout := time.Minute

	activeEntry := testaide.GenerateTaskEntry(currentTime.Add(-time.Second), types.Running, testaide.GenerateRandomExecutorId())

	err := s.ts.AddSingleTaskEntry(s.ctx, *activeEntry)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, currentTime, executionTimeout)
	s.Require().NoError(err)

	// Active task wasn't rescheduled
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) TestTaskRescheduling_MultipleTasks() {
	currentTime := time.Now()
	executionTimeout := time.Minute

	outdatedEntry := testaide.GenerateTaskEntry(currentTime.Add(-executionTimeout*2), types.Running, testaide.GenerateRandomExecutorId())

	err := s.ts.AddTaskEntries(s.ctx, []*types.TaskEntry{
		outdatedEntry,
		testaide.GenerateTaskEntry(currentTime.Add(-time.Second), types.Running, testaide.GenerateRandomExecutorId()),
		testaide.GenerateTaskEntry(currentTime.Add(-time.Second*2), types.Running, testaide.GenerateRandomExecutorId()),
		testaide.GenerateTaskEntry(currentTime.Add(-time.Hour*2), types.Failed, testaide.GenerateRandomExecutorId()),
	})
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, currentTime, executionTimeout)
	s.Require().NoError(err)

	// Outdated task was rescheduled and became available for execution
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(outdatedEntry.Task, *taskToExecute)

	// Active and failed tasks weren't rescheduled
	taskToExecute, err = s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) Test_AddSingleTaskEntry_Concurrently() {
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			entry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForExecutor, types.UnknownExecutorId)
			err := s.ts.AddSingleTaskEntry(s.ctx, *entry)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()

	s.requireExactTasksCount(degreeOfParallelism)
}

func (s *TaskStorageSuite) Test_AddTaskEntries_Concurrently() {
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)
	const tasksPerWorker = 3

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			var entries []*types.TaskEntry
			for range tasksPerWorker {
				randomEntry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForExecutor, types.UnknownExecutorId)
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
		task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
		s.Require().NoError(err)
		s.Require().NotNil(task)
	}

	// There no more tasks left
	task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_RemoveTaskEntry_Concurrently() {
	entry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForExecutor, types.UnknownExecutorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *entry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			err := s.ts.RemoveTaskEntry(s.ctx, entry.Task.Id)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()
	task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_RequestTaskToExecute_Concurrently() {
	entry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForExecutor, types.UnknownExecutorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *entry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	receivedTaskCount := atomic.Uint32{}

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomExecutorId())
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
	executorId := testaide.GenerateRandomExecutorId()
	runningEntry := testaide.GenerateTaskEntry(time.Now(), types.Running, executorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *runningEntry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			err := s.ts.ProcessTaskResult(
				s.ctx,
				types.SuccessProverTaskResult(runningEntry.Task.Id, executorId, types.PartialProve, types.TaskResultFiles{}),
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
			s.tryToChangeStatus(testCase.oldStatus, true, false, ErrTaskInvalidStatus)
		})
		s.Run(testCase.name+"_TrySetFailure", func() {
			s.tryToChangeStatus(testCase.oldStatus, false, false, ErrTaskInvalidStatus)
		})
	}
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_WrongExecutor() {
	s.Run("TrySetSuccess", func() {
		s.tryToChangeStatus(types.Running, true, true, ErrTaskWrongExecutor)
	})
	s.Run("TrySetFailure", func() {
		s.tryToChangeStatus(types.Running, false, true, ErrTaskWrongExecutor)
	})
}

func (s *TaskStorageSuite) tryToChangeStatus(
	oldStatus types.TaskStatus,
	trySetSuccess bool,
	useDifferentExecutorId bool,
	expectedError error,
) {
	s.T().Helper()

	executorId := testaide.GenerateRandomExecutorId()
	taskEntry := testaide.GenerateTaskEntry(time.Now(), oldStatus, executorId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *taskEntry)
	s.Require().NoError(err)

	if useDifferentExecutorId {
		executorId = testaide.GenerateRandomExecutorId()
	}

	var taskResult types.TaskResult
	if trySetSuccess {
		taskResult = types.SuccessProverTaskResult(taskEntry.Task.Id, executorId, types.PartialProve, types.TaskResultFiles{})
	} else {
		taskResult = types.FailureProverTaskResult(taskEntry.Task.Id, executorId, errors.New("some error"))
	}

	err = s.ts.ProcessTaskResult(s.ctx, taskResult)
	s.Require().ErrorIs(err, expectedError)
}
