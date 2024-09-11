package synccommittee

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskStorageSuite struct {
	suite.Suite
	ts  ProverTaskStorage
	ctx context.Context

	baseTask      types.ProverTask
	baseTaskEntry types.ProverTaskEntry
}

func (s *TaskStorageSuite) SetupSuite() {
	db, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.ts = NewTaskStorage(db)
	s.ctx = context.Background()

	s.baseTask = types.ProverTask{
		Id:            1,
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

func (s *TaskStorageSuite) TestAddRemove() {
	modifiedEntry := s.baseTaskEntry
	modifiedEntry.Task.Id = 2
	modifiedEntry.Task.BatchNum = 8

	err := s.ts.AddTaskEntry(s.ctx, s.baseTaskEntry)
	s.Require().NoError(err)
	err = s.ts.AddTaskEntry(s.ctx, modifiedEntry)
	s.Require().NoError(err)

	err = s.ts.RemoveTaskEntry(s.ctx, s.baseTask.Id)
	s.Require().NoError(err)

	// Trying to reschedule the removed task must cause an error
	err = s.ts.RescheduleTask(s.ctx, s.baseTask.Id)
	s.Require().Error(err)

	// Remove remaining tasks from db to clean it up for other tests
	err = s.ts.RemoveTaskEntry(s.ctx, modifiedEntry.Task.Id)
	s.Require().NoError(err)
}

func (s *TaskStorageSuite) TestReschedule() {
	runningTaskEntry := s.baseTaskEntry
	runningTaskEntry.Task.Id = 5
	runningTaskEntry.Status = types.Running
	err := s.ts.AddTaskEntry(s.ctx, runningTaskEntry)
	s.Require().NoError(err)

	// Trying to request task for proving must give nothing,
	// since we have only one task and it's running
	task, err := s.ts.RequestTaskToExecute(s.ctx, 99)
	s.Require().NoError(err)
	s.Nil(task)

	// Reschedule task and check if it may be requested for proving now
	err = s.ts.RescheduleTask(s.ctx, runningTaskEntry.Task.Id)
	s.Require().NoError(err)
	t, err := s.ts.RequestTaskToExecute(s.ctx, 99)
	s.Require().NoError(err)
	s.Equal(t.Id, runningTaskEntry.Task.Id)

	// Remove remaining tasks from db to clean it up for other tests
	err = s.ts.RemoveTaskEntry(s.ctx, runningTaskEntry.Task.Id)
	s.Require().NoError(err)
}

func (s *TaskStorageSuite) TestRequestAndProcessResult() {
	// Initialize two tasks waiting for input
	lowerPriorityEntry := s.baseTaskEntry
	lowerPriorityEntry.Task.Id = 11
	lowerPriorityEntry.Task.BlockNum = 222
	lowerPriorityEntry.Status = types.WaitingForInput
	lowerPriorityEntry.Task.DependencyNum = 1

	higherPriorityEntry := s.baseTaskEntry
	higherPriorityEntry.Task.Id = 12
	higherPriorityEntry.Task.BlockNum = 14
	higherPriorityEntry.Status = types.WaitingForInput
	higherPriorityEntry.Task.DependencyNum = 1

	// Initialize two corresponding dependencies for them which are running
	dependency1 := s.baseTaskEntry
	dependency1.Task.Id = 13
	dependency1.PendingDeps = []types.ProverTaskId{lowerPriorityEntry.Task.Id}
	dependency1.Status = types.Running
	dependency2 := s.baseTaskEntry
	dependency2.PendingDeps = []types.ProverTaskId{higherPriorityEntry.Task.Id}
	dependency2.Task.Id = 14
	dependency2.Status = types.Running
	err := s.ts.AddTaskEntry(s.ctx, lowerPriorityEntry)
	s.Require().NoError(err)
	err = s.ts.AddTaskEntry(s.ctx, higherPriorityEntry)
	s.Require().NoError(err)
	err = s.ts.AddTaskEntry(s.ctx, dependency1)
	s.Require().NoError(err)
	err = s.ts.AddTaskEntry(s.ctx, dependency2)
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

	// Make higher priority task ready and reschedule the lower one
	err = s.ts.ProcessTaskResult(
		s.ctx,
		types.SuccessTaskResult(dependency2.Task.Id, dependency2.Owner, types.FriConsistencyProof, "3C4D"),
	)
	s.Require().NoError(err)
	s.Require().NoError(s.ts.RescheduleTask(s.ctx, lowerPriorityEntry.Task.Id))

	// Now both tasks has waitingForProver status, and our request must extract the one with the higher priority
	task, err = s.ts.RequestTaskToExecute(s.ctx, 88)
	s.Require().NoError(err)
	s.Equal(task.Id, higherPriorityEntry.Task.Id)

	// Remove remaining tasks from db to clean it up for other tests
	err = s.ts.RemoveTaskEntry(s.ctx, lowerPriorityEntry.Task.Id)
	s.Require().NoError(err)
	err = s.ts.RemoveTaskEntry(s.ctx, higherPriorityEntry.Task.Id)
	s.Require().NoError(err)
}

func TestTaskStorageSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskStorageSuite))
}
