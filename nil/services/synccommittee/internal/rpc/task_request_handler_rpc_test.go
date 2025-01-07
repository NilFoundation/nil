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
		{"Returns_Task_Without_Deps", firstExecutorId},
		{"Returns_Task_With_Deps", secondExecutorId},
		{"Returns_Nil", testaide.RandomExecutorId()},
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
	getTaskCalls := s.scheduler.GetTaskCalls()
	s.Require().Len(getTaskCalls, 1, "expected one call to GetTask")
	s.Require().Equal(request, getTaskCalls[0].Request)

	expectedTask := tasksForExecutors[executorId]
	s.Equal(expectedTask, receivedTask)
}

func (s *TaskRequestHandlerTestSuite) Test_TaskRequestHandler_UpdateTaskStatus() {
	testCases := []struct {
		name   string
		result *types.TaskResult
	}{
		{
			"Success_Result_Final_Proof",
			types.NewSuccessProverTaskResult(
				types.NewTaskId(),
				testaide.RandomExecutorId(),
				types.TaskResultAddresses{types.FinalProof: "final-proof.1.0xAABC"},
				types.TaskResultData{10, 20, 30, 40},
			),
		},
		{
			"Success_Result_Provider",
			types.NewSuccessProviderTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), nil, nil),
		},
		{
			"Failure_Result_Provider",
			types.NewFailureProverTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), errors.New("something went wrong")),
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.testSetTaskStatus(testCase.result)
		})
	}
}

func (s *TaskRequestHandlerTestSuite) testSetTaskStatus(resultToSend *types.TaskResult) {
	s.T().Helper()

	err := s.clientHandler.SetTaskResult(s.context, resultToSend)
	s.Require().NoError(err)

	setResultCalls := s.scheduler.SetTaskResultCalls()
	s.Require().Len(setResultCalls, 1, "expected one call to SetTaskResult")
	s.Require().Equal(resultToSend, setResultCalls[0].Result)
}
