package storage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
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
	ts       ProverTaskStorage
	ctx      context.Context

	baseTask      types.ProverTask
	baseTaskEntry types.ProverTaskEntry
}

func TestTaskStorageSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskStorageSuite))
}

func (s *TaskStorageSuite) SetupSuite() {
	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database
	logger := logging.NewLogger("prover-task-storage-test")
	s.ts = NewTaskStorage(database, logger)
	s.ctx = context.Background()

	s.baseTask = types.ProverTask{
		Id:            types.NewProverTaskId(),
		BatchNum:      1,
		BlockNum:      1,
		TaskType:      types.Preprocess,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.ProverTaskId]types.ProverTaskResult),
		DependencyNum: 0,
	}
	s.baseTaskEntry = types.ProverTaskEntry{
		Task:     s.baseTask,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   types.WaitingForProver,
	}
}

func (s *TaskStorageSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskStorageSuite) TestAddRemove() {
	modifiedEntry := s.baseTaskEntry
	modifiedEntry.Task.Id = types.NewProverTaskId()
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
	lowerPriorityEntry.Task.Id = types.NewProverTaskId()
	lowerPriorityEntry.Task.BlockNum = 222
	lowerPriorityEntry.Status = types.WaitingForInput
	lowerPriorityEntry.Task.DependencyNum = 1

	higherPriorityEntry := s.baseTaskEntry
	higherPriorityEntry.Task.Id = types.NewProverTaskId()
	higherPriorityEntry.Task.BlockNum = 14
	higherPriorityEntry.Status = types.WaitingForInput
	higherPriorityEntry.Task.DependencyNum = 1

	// Initialize two corresponding dependencies for them which are running
	dependency1 := s.baseTaskEntry
	dependency1.Task.Id = types.NewProverTaskId()
	dependency1.PendingDeps = []types.ProverTaskId{lowerPriorityEntry.Task.Id}
	dependency1.Status = types.Running
	dependency2 := s.baseTaskEntry
	dependency2.PendingDeps = []types.ProverTaskId{higherPriorityEntry.Task.Id}
	dependency2.Task.Id = types.NewProverTaskId()
	dependency2.Status = types.Running
	err := s.ts.AddSingleTaskEntry(s.ctx, lowerPriorityEntry)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, higherPriorityEntry)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, dependency1)
	s.Require().NoError(err)
	err = s.ts.AddSingleTaskEntry(s.ctx, dependency2)
	s.Require().NoError(err)

	// No available tasks for prover at this point
	task, err := s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Nil(task)

	// Make lower priority task ready for execution
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.SuccessTaskResult(dependency1.Task.Id, dependency1.Owner, types.Commitment, "1A2B"),
	)
	s.Require().NoError(err)
	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Equal(task.Id, lowerPriorityEntry.Task.Id)

	// Make higher priority task ready
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.SuccessTaskResult(dependency2.Task.Id, dependency2.Owner, types.FriConsistencyProof, "3C4D"),
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

	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) TestTaskRescheduling_NoActiveTasks() {
	currentTime := time.Now()
	executionTimeout := time.Minute

	entries := []*types.ProverTaskEntry{
		testaide.GenerateTaskEntry(currentTime.Add(-time.Second), types.WaitingForProver, types.UnknownProverId),
		testaide.GenerateTaskEntry(currentTime.Add(-time.Hour*24), types.WaitingForProver, types.UnknownProverId),
	}

	err := s.ts.AddTaskEntries(s.ctx, entries)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, currentTime, executionTimeout)
	s.Require().NoError(err)

	// All existing tasks are still available for execution
	for range entries {
		taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
		s.Require().NoError(err)
		s.Require().NotNil(taskToExecute)
	}
}

func (s *TaskStorageSuite) TestTaskRescheduling_SingleActiveTask() {
	currentTime := time.Now()
	executionTimeout := time.Minute

	activeEntry := testaide.GenerateTaskEntry(currentTime.Add(-time.Second), types.Running, testaide.GenerateRandomProverId())

	err := s.ts.AddSingleTaskEntry(s.ctx, *activeEntry)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, currentTime, executionTimeout)
	s.Require().NoError(err)

	// Active task wasn't rescheduled
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) TestTaskRescheduling_MultipleTasks() {
	currentTime := time.Now()
	executionTimeout := time.Minute

	outdatedEntry := testaide.GenerateTaskEntry(currentTime.Add(-executionTimeout*2), types.Running, testaide.GenerateRandomProverId())

	err := s.ts.AddTaskEntries(s.ctx, []*types.ProverTaskEntry{
		outdatedEntry,
		testaide.GenerateTaskEntry(currentTime.Add(-time.Second), types.Running, testaide.GenerateRandomProverId()),
		testaide.GenerateTaskEntry(currentTime.Add(-time.Second*2), types.Running, testaide.GenerateRandomProverId()),
		testaide.GenerateTaskEntry(currentTime.Add(-time.Hour*2), types.Failed, testaide.GenerateRandomProverId()),
	})
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, currentTime, executionTimeout)
	s.Require().NoError(err)

	// Outdated task was rescheduled and became available for execution
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
	s.Require().NoError(err)
	s.Require().NotNil(taskToExecute)
	s.Require().Equal(outdatedEntry.Task, *taskToExecute)

	// Active and failed tasks weren't rescheduled
	taskToExecute, err = s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) Test_AddSingleTaskEntry_Concurrently() {
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			entry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForProver, types.UnknownProverId)
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
			var entries []*types.ProverTaskEntry
			for range tasksPerWorker {
				randomEntry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForProver, types.UnknownProverId)
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
		task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
		s.Require().NoError(err)
		s.Require().NotNil(task)
	}

	// There no more tasks left
	task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_RemoveTaskEntry_Concurrently() {
	entry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForProver, types.UnknownProverId)
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
	task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_RequestTaskToExecute_Concurrently() {
	entry := testaide.GenerateTaskEntry(time.Now(), types.WaitingForProver, types.UnknownProverId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *entry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	receivedTaskCount := atomic.Uint32{}

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			task, err := s.ts.RequestTaskToExecute(s.ctx, testaide.GenerateRandomProverId())
			s.NoError(err)

			if task != nil {
				s.Equal(entry.Task, *task)
				receivedTaskCount.Add(1)
			}
		}()
	}

	waitGroup.Wait()
	s.Require().Equal(uint32(1), receivedTaskCount.Load(), "expected only one prover to receive task")
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_Concurrently() {
	proverId := testaide.GenerateRandomProverId()
	runningEntry := testaide.GenerateTaskEntry(time.Now(), types.Running, proverId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *runningEntry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			err := s.ts.ProcessTaskResult(
				s.ctx,
				types.SuccessTaskResult(runningEntry.Task.Id, proverId, types.Commitment, "1A2B"),
			)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()

	// Task was successfully completed and was removed from the storage
	task, err := s.ts.RequestTaskToExecute(s.ctx, proverId)
	s.Require().NoError(err)
	s.Require().Nil(task)
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_InvalidStateChange() {
	testCases := []struct {
		name      string
		oldStatus types.ProverTaskStatus
	}{
		{"WaitingForInput", types.WaitingForInput},
		{"WaitingForProver", types.WaitingForProver},
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

func (s *TaskStorageSuite) Test_ProcessTaskResult_WrongProver() {
	s.Run("TrySetSuccess", func() {
		s.tryToChangeStatus(types.Running, true, true, ErrTaskWrongProver)
	})
	s.Run("TrySetFailure", func() {
		s.tryToChangeStatus(types.Running, false, true, ErrTaskWrongProver)
	})
}

func (s *TaskStorageSuite) tryToChangeStatus(
	oldStatus types.ProverTaskStatus,
	trySetSuccess bool,
	useDifferentProverId bool,
	expectedError error,
) {
	s.T().Helper()

	proverId := testaide.GenerateRandomProverId()
	taskEntry := testaide.GenerateTaskEntry(time.Now(), oldStatus, proverId)
	err := s.ts.AddSingleTaskEntry(s.ctx, *taskEntry)
	s.Require().NoError(err)

	if useDifferentProverId {
		proverId = testaide.GenerateRandomProverId()
	}

	var taskResult types.ProverTaskResult
	if trySetSuccess {
		taskResult = types.SuccessTaskResult(taskEntry.Task.Id, proverId, types.Commitment, "1A2B")
	} else {
		taskResult = types.FailureTaskResult(taskEntry.Task.Id, proverId, errors.New("some error"))
	}

	err = s.ts.ProcessTaskResult(s.ctx, taskResult)
	s.Require().ErrorIs(err, expectedError)
}
