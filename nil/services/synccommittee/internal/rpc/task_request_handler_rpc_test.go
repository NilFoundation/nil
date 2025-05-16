package rpc

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskRequestHandlerSuite struct {
	ServerTestSuite
	storage   *storage.TaskStorage
	rpcClient api.TaskRequestHandler
}

func (s *TaskRequestHandlerSuite) SetupSuite() {
	s.ServerTestSuite.SetupSuite()

	s.storage = storage.NewTaskStorage(s.database, s.clock, s.metricsHandler, s.logger)
	noopStateHandler := &api.TaskStateChangeHandlerMock{}
	taskScheduler := scheduler.New(s.storage, noopStateHandler, s.metricsHandler, s.logger)

	handler := TaskRequestServerHandler(taskScheduler)
	s.RunRpcServer(handler)

	s.rpcClient = NewTaskRequestRpcClient(s.serverEndpoint, s.logger)
}

func TestTaskRequestHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskRequestHandlerSuite))
}

func (s *TaskRequestHandlerSuite) Test_GetTask_Without_Deps() {
	executor := types.NewRandomExecutorId()
	currentTime := s.clock.Now()

	// Prepare a task without dependencies
	task := testaide.NewTaskEntry(currentTime, types.WaitingForExecutor, types.UnknownExecutorId)
	err := s.storage.AddTaskEntries(s.context, task)
	s.Require().NoError(err)

	// Make the request
	request := api.NewTaskRequest(executor)
	receivedTask, err := s.rpcClient.GetTask(s.context, request)
	s.Require().NoError(err)

	// Validate the response
	s.Require().NotNil(receivedTask)
	s.Equal(task.Task.Id, receivedTask.Id)
	s.Equal(task.Task.TaskType, receivedTask.TaskType)

	// Verify task status was updated
	updatedTask, err := s.storage.TryGetTaskEntry(s.context, task.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(updatedTask)
	s.Equal(types.Running, updatedTask.Status)
	s.Equal(executor, updatedTask.Owner)
}

func (s *TaskRequestHandlerSuite) Test_GetTask_With_Deps() {
	executor := types.NewRandomExecutorId()

	dependencyExecutor := types.NewRandomExecutorId()
	dependency := testaide.NewTaskEntry(s.clock.Now(), types.Running, dependencyExecutor)
	depResult := testaide.NewSuccessTaskResult(dependency.Task.Id, dependencyExecutor)

	// Create a task with dependency
	taskEntry := testaide.NewTaskEntry(s.clock.Now(), types.WaitingForExecutor, types.UnknownExecutorId)
	taskEntry.AddDependency(dependency)

	s.clock.Advance(time.Minute)
	details := types.NewTaskResultDetails(depResult, dependency, s.clock.Now())
	err := taskEntry.AddDependencyResult(*details)
	s.Require().NoError(err)

	err = s.storage.AddTaskEntries(s.context, taskEntry)
	s.Require().NoError(err)

	// Make the request
	request := api.NewTaskRequest(executor)
	receivedTask, err := s.rpcClient.GetTask(s.context, request)
	s.Require().NoError(err)

	// Validate the response
	s.Require().NotNil(receivedTask)
	s.Require().Equal(taskEntry.Task, *receivedTask)
}

func (s *TaskRequestHandlerSuite) Test_GetTask_Returns_Nil_When_No_Tasks_Available() {
	executor := types.NewRandomExecutorId()

	request := api.NewTaskRequest(executor)

	receivedTask, err := s.rpcClient.GetTask(s.context, request)
	s.Require().NoError(err)
	s.Nil(receivedTask)
}

func (s *TaskRequestHandlerSuite) Test_UpdateTaskStatus_Success() {
	currentTime := s.clock.Now()
	executor := types.NewRandomExecutorId()

	// Create and add a task
	task := testaide.NewTaskEntry(currentTime, types.Running, executor)
	err := s.storage.AddTaskEntries(s.context, task)
	s.Require().NoError(err)

	// Create a success result
	result := testaide.NewSuccessTaskResult(task.Task.Id, executor)

	// Update task status
	err = s.rpcClient.SetTaskResult(s.context, result)
	s.Require().NoError(err)

	// Verify the task was completed
	completedTask, err := s.storage.TryGetTaskEntry(s.context, task.Task.Id)
	s.Require().NoError(err)
	s.Require().Nil(completedTask)
}

func (s *TaskRequestHandlerSuite) Test_UpdateTaskStatus_Failure() {
	currentTime := s.clock.Now()
	executor := types.NewRandomExecutorId()

	// Create and add a task
	task := testaide.NewTaskEntry(currentTime, types.Running, executor)
	err := s.storage.AddTaskEntries(s.context, task)
	s.Require().NoError(err)

	// Create failure result
	result := types.NewFailureProverTaskResult(
		task.Task.Id,
		executor,
		types.NewTaskExecError(types.TaskErrProofGenerationFailed, "something went wrong"),
	)

	// Update task status
	err = s.rpcClient.SetTaskResult(s.context, result)
	s.Require().NoError(err)

	// Verify task was marked as failed
	failedTask, err := s.storage.TryGetTaskEntry(s.context, task.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(failedTask)
	s.Equal(types.Failed, failedTask.Status)
}
