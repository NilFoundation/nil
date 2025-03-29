package storage

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

const (
	degreeOfParallelism = 10
)

type TaskStorageSuite struct {
	suite.Suite
	database db.DB
	clock    clockwork.Clock
	ts       *TaskStorage
	ctx      context.Context
}

type newTaskResultFunc func(taskId types.TaskId, executorId types.TaskExecutorId) *types.TaskResult

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

	s.clock = testaide.NewTestClock()
	s.ts = NewTaskStorage(database, s.clock, metricsHandler, logger)
	s.ctx = context.Background()
}

func (s *TaskStorageSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskStorageSuite) Test_Request_And_Process_Result() {
	now := s.clock.Now()

	// Initialize two tasks waiting for input
	lowerPriorityEntry := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)

	higherPriorityEntry := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)

	// Initialize two corresponding dependencies for them which are running
	dependency1 := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	lowerPriorityEntry.AddDependency(dependency1)

	dependency2 := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	higherPriorityEntry.AddDependency(dependency2)

	err := s.ts.AddTaskEntries(s.ctx, lowerPriorityEntry, higherPriorityEntry, dependency1, dependency2)
	s.Require().NoError(err)

	// No available tasks for executor at this point
	task, err := s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Require().Nil(task)

	// Make lower priority task ready for execution
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.NewSuccessProverTaskResult(
			dependency1.Task.Id, dependency1.Owner, types.TaskOutputArtifacts{}, types.TaskResultData{}),
	)
	s.Require().NoError(err)
	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Require().NotNil(task)
	s.Equal(task.Id, lowerPriorityEntry.Task.Id)

	// Make higher priority task ready
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.NewSuccessProverTaskResult(
			dependency2.Task.Id, dependency2.Owner, types.TaskOutputArtifacts{}, types.TaskResultData{}),
	)
	s.Require().NoError(err)

	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Require().NotNil(task)
	s.Equal(task.Id, higherPriorityEntry.Task.Id)
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_Retryable_Error() {
	now := s.clock.Now()
	executorId := testaide.RandomExecutorId()

	parentTask := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)

	fstChildTask := testaide.NewTaskEntry(now, types.Running, executorId)
	parentTask.AddDependency(fstChildTask)

	sndChildTask := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	parentTask.AddDependency(sndChildTask)

	err := s.ts.AddTaskEntries(s.ctx, parentTask, fstChildTask, sndChildTask)
	s.Require().NoError(err)

	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.NewFailureProverTaskResult(
			fstChildTask.Task.Id, executorId,
			types.NewTaskExecError(types.TaskErrRpc, "RPC method failed"),
		),
	)
	s.Require().NoError(err)

	// failed task was rescheduled
	failedFromStorage, err := s.ts.TryGetTaskEntry(s.ctx, fstChildTask.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(failedFromStorage)
	s.Equal(types.WaitingForExecutor, failedFromStorage.Status)
	s.Equal(types.UnknownExecutorId, failedFromStorage.Owner)
	s.Equal(1, failedFromStorage.RetryCount)

	// parentTask and sndChildTask should not be affected by the failure of fstChildTask

	parentFromStorage, err := s.ts.TryGetTaskEntry(s.ctx, parentTask.Task.Id)
	s.Require().NoError(err)
	s.Require().Equal(parentTask, parentFromStorage)

	sndChildFromStorage, err := s.ts.TryGetTaskEntry(s.ctx, sndChildTask.Task.Id)
	s.Require().NoError(err)
	s.Require().Equal(sndChildTask, sndChildFromStorage)
}

func (s *TaskStorageSuite) Test_TaskRescheduling_NoEntries() {
	executionTimeout := time.Minute
	err := s.ts.RescheduleHangingTasks(s.ctx, executionTimeout)
	s.Require().NoError(err)

	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) Test_TaskRescheduling_NoActiveTasks() {
	now := s.clock.Now()
	executionTimeout := time.Minute

	entries := []*types.TaskEntry{
		testaide.NewTaskEntry(now.Add(-time.Second), types.WaitingForExecutor, types.UnknownExecutorId),
		testaide.NewTaskEntry(now.Add(-time.Hour*24), types.WaitingForExecutor, types.UnknownExecutorId),
	}

	err := s.ts.AddTaskEntries(s.ctx, entries...)
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

func (s *TaskStorageSuite) Test_TaskRescheduling_SingleActiveTask() {
	now := s.clock.Now()
	executionTimeout := time.Minute

	activeEntry := testaide.NewTaskEntry(now.Add(-time.Second), types.Running, testaide.RandomExecutorId())

	err := s.ts.AddTaskEntries(s.ctx, activeEntry)
	s.Require().NoError(err)

	err = s.ts.RescheduleHangingTasks(s.ctx, executionTimeout)
	s.Require().NoError(err)

	// Active task wasn't rescheduled
	taskToExecute, err := s.ts.RequestTaskToExecute(s.ctx, testaide.RandomExecutorId())
	s.Require().NoError(err)
	s.Require().Nil(taskToExecute)
}

func (s *TaskStorageSuite) Test_TaskRescheduling_MultipleTasks() {
	now := s.clock.Now()
	executionTimeout := time.Minute

	outdatedEntry := testaide.NewTaskEntry(now.Add(-executionTimeout*2), types.Running, testaide.RandomExecutorId())

	err := s.ts.AddTaskEntries(s.ctx,
		outdatedEntry,
		testaide.NewTaskEntry(now.Add(-time.Second), types.Running, testaide.RandomExecutorId()),
		testaide.NewTaskEntry(now.Add(-time.Second*2), types.Running, testaide.RandomExecutorId()),
		testaide.NewTaskEntry(now.Add(-time.Hour*2), types.Failed, testaide.RandomExecutorId()),
	)
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
	now := s.clock.Now()

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			entry := testaide.NewTaskEntry(now, types.WaitingForExecutor, types.UnknownExecutorId)
			err := s.ts.AddTaskEntries(s.ctx, entry)
			s.NoError(err)
		}()
	}

	waitGroup.Wait()

	s.requireExactTasksCount(degreeOfParallelism)
}

func (s *TaskStorageSuite) Test_AddTaskEntries_Concurrently() {
	now := s.clock.Now()

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
			err := s.ts.AddTaskEntries(s.ctx, entries...)
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
	now := s.clock.Now()

	entry := testaide.NewTaskEntry(now, types.WaitingForExecutor, types.UnknownExecutorId)
	err := s.ts.AddTaskEntries(s.ctx, entry)
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
	now := s.clock.Now()

	executorId := testaide.RandomExecutorId()
	runningEntry := testaide.NewTaskEntry(now, types.Running, executorId)
	err := s.ts.AddTaskEntries(s.ctx, runningEntry)
	s.Require().NoError(err)

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(degreeOfParallelism)

	for range degreeOfParallelism {
		go func() {
			defer waitGroup.Done()
			err := s.ts.ProcessTaskResult(
				s.ctx,
				types.NewSuccessProverTaskResult(
					runningEntry.Task.Id, executorId, types.TaskOutputArtifacts{}, types.TaskResultData{}),
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
	taskStatuses := []types.TaskStatus{
		types.WaitingForInput,
		types.WaitingForExecutor,
		types.Failed,
	}

	taskResults := getTestTaskResults(false)

	for _, taskStatus := range taskStatuses {
		for _, result := range taskResults {
			s.Run(taskStatus.String()+"_"+result.name, func() {
				s.tryToChangeStatus(taskStatus, result.factory, types.ErrTaskInvalidStatus)
			})
		}
	}
}

func (s *TaskStorageSuite) Test_ProcessTaskResult_WrongExecutor() {
	taskResults := getTestTaskResults(true)

	for _, result := range taskResults {
		s.Run(result.name, func() {
			s.tryToChangeStatus(types.Running, result.factory, types.ErrTaskWrongExecutor)
		})
	}
}

func getTestTaskResults(useDifferentExecutorId bool) []struct {
	name    string
	factory newTaskResultFunc
} {
	substituteId := func(execId types.TaskExecutorId) types.TaskExecutorId {
		if useDifferentExecutorId {
			return testaide.RandomExecutorId()
		}
		return execId
	}

	return []struct {
		name    string
		factory newTaskResultFunc
	}{
		{
			"Success",
			func(taskId types.TaskId, executorId types.TaskExecutorId) *types.TaskResult {
				return testaide.NewSuccessTaskResult(taskId, substituteId(executorId))
			},
		},
		{
			"RetryableError",
			func(taskId types.TaskId, executorId types.TaskExecutorId) *types.TaskResult {
				return testaide.NewRetryableErrorTaskResult(taskId, substituteId(executorId))
			},
		},
		{
			"NonRetryableError",
			func(taskId types.TaskId, executorId types.TaskExecutorId) *types.TaskResult {
				return testaide.NewNonRetryableErrorTaskResult(taskId, substituteId(executorId))
			},
		},
	}
}

func (s *TaskStorageSuite) tryToChangeStatus(
	oldStatus types.TaskStatus,
	resultToSubmit newTaskResultFunc,
	expectedError error,
) {
	s.T().Helper()

	now := s.clock.Now()
	executorId := testaide.RandomExecutorId()
	taskEntry := testaide.NewTaskEntry(now, oldStatus, executorId)
	err := s.ts.AddTaskEntries(s.ctx, taskEntry)
	s.Require().NoError(err)

	taskResult := resultToSubmit(taskEntry.Task.Id, executorId)
	err = s.ts.ProcessTaskResult(s.ctx, taskResult)
	s.Require().ErrorIs(err, expectedError)
}

func (s *TaskStorageSuite) Test_AddSingleTaskEntry_Fail_If_Already_Exists() {
	now := s.clock.Now()
	entry := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)
	err := s.ts.AddTaskEntries(s.ctx, entry)
	s.Require().NoError(err)

	err = s.ts.AddTaskEntries(s.ctx, entry)
	s.Require().ErrorIs(err, ErrTaskAlreadyExists)
}

func (s *TaskStorageSuite) Test_AddTaskEntries_Fail_If_Already_Exists() {
	now := s.clock.Now()
	entries := testaide.NewPendingTaskEntries(now, 3)

	err := s.ts.AddTaskEntries(s.ctx, entries...)
	s.Require().NoError(err)

	newTask := testaide.NewTaskEntry(now, types.WaitingForInput, types.UnknownExecutorId)

	err = s.ts.AddTaskEntries(s.ctx, newTask, entries[0])
	s.Require().ErrorIs(err, ErrTaskAlreadyExists)

	entryFromStorage, err := s.ts.TryGetTaskEntry(s.ctx, newTask.Task.Id)
	s.Require().NoError(err)
	s.Require().Nil(entryFromStorage)
}

func (s *TaskStorageSuite) Test_GetStats() {
	stats, err := s.ts.GetTaskStats(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(stats)

	s.Require().Empty(stats.CountPerType)
	s.Require().Empty(stats.CountPerExecutor)

	now := s.clock.Now()
	entry := testaide.NewTaskEntry(now, types.WaitingForExecutor, types.UnknownExecutorId)
	err = s.ts.AddTaskEntries(s.ctx, entry)
	s.Require().NoError(err)

	stats, err = s.ts.GetTaskStats(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(stats)

	s.Require().Len(stats.CountPerType, 1)
	s.Equal(uint32(1), stats.CountPerType[entry.Task.TaskType].PendingCount)
	s.Equal(uint32(0), stats.CountPerType[entry.Task.TaskType].ActiveCount)
	s.Empty(stats.CountPerExecutor)

	executor := testaide.RandomExecutorId()
	_, err = s.ts.RequestTaskToExecute(s.ctx, executor)
	s.Require().NoError(err)

	stats, err = s.ts.GetTaskStats(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(stats)

	s.Require().Len(stats.CountPerType, 1)
	s.Equal(uint32(0), stats.CountPerType[entry.Task.TaskType].PendingCount)
	s.Equal(uint32(1), stats.CountPerType[entry.Task.TaskType].ActiveCount)
	s.Require().Len(stats.CountPerExecutor, 1)
	s.Equal(uint32(1), stats.CountPerExecutor[executor])

	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.NewSuccessProverTaskResult(entry.Task.Id, executor, types.TaskOutputArtifacts{}, types.TaskResultData{}),
	)
	s.Require().NoError(err)

	stats, err = s.ts.GetTaskStats(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(stats)

	s.Empty(stats.CountPerType)
	s.Empty(stats.CountPerExecutor)
}
