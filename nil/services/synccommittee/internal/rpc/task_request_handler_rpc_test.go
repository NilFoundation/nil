package rpc

import (
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

// Check TaskRequestHandler API calls from Prover to SyncCommittee
func TestTaskRequestHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskRequestHandlerTestSuite))
}

func (s *TaskRequestHandlerTestSuite) Test_TaskRequestHandler_GetTask() {
	testCases := []struct {
		name       string
		executorId types.TaskExecutorId
	}{
		{"returns task without deps", firstExecutorId},
		{"returns task with deps", secondExecutorId},
		{"returns nil", testaide.RandomExecutorId()},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.testGetTask(testCase.executorId)
		})
	}
}

func (s *TaskRequestHandlerTestSuite) testGetTask(executorId types.TaskExecutorId) {
	s.T().Helper()

	request := api.NewTaskRequest(executorId)
	receivedTask, err := s.clientHandler.GetTask(s.context, request)
	s.Require().NoError(err)
	getTaskCalls := s.targetHandler.GetTaskCalls()
	s.Require().Len(getTaskCalls, 1, "expected one call to GetTask")
	s.Require().Equal(request, getTaskCalls[0].Request)

	expectedTask := tasksForExecutors[executorId]
	s.Equal(expectedTask, receivedTask)
}

func (s *TaskRequestHandlerTestSuite) Test_TaskRequestHandler_UpdateTaskStatus() {
	testCases := []struct {
		name   string
		result types.TaskResult
	}{
		{
			"success result FinalProof",
			types.SuccessProverTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), types.MergeProof, types.TaskResultAddresses{}, types.TaskResultData{}),
		},
		{
			"success result Commitment",
			types.SuccessProverTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), types.MergeProof, types.TaskResultAddresses{}, types.TaskResultData{}),
		},
		{
			"failure result",
			types.FailureProverTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), errors.New("something went wrong")),
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.testSetTaskStatus(testCase.result)
		})
	}
}

func (s *TaskRequestHandlerTestSuite) testSetTaskStatus(resultToSend types.TaskResult) {
	s.T().Helper()

	err := s.clientHandler.SetTaskResult(s.context, &resultToSend)
	s.Require().NoError(err)

	setResultCalls := s.targetHandler.SetTaskResultCalls()
	s.Require().Len(setResultCalls, 1, "expected one call to SetTaskResult")
	s.Require().Equal(&resultToSend, setResultCalls[0].Result)
}
