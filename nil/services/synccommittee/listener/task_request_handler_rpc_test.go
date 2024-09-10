package listener

import (
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
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
		name     string
		proverId types.ProverId
	}{
		{"returns task without deps", firstProverId},
		{"returns task with deps", secondProverId},
		{"returns nil", randomProverId()},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.testGetTask(testCase.proverId)
		})
	}
}

func (s *TaskRequestHandlerTestSuite) testGetTask(proverId types.ProverId) {
	s.T().Helper()

	request := api.NewTaskRequest(proverId)
	receivedTask, err := s.clientHandler.GetTask(s.context, request)
	s.Require().NoError(err)
	getTaskCalls := s.targetHandler.GetTaskCalls()
	s.Require().Len(getTaskCalls, 1, "expected one call to GetTask")
	s.Require().Equal(request, getTaskCalls[0].Request)

	expectedTask := tasksForProvers[proverId]
	s.Equal(expectedTask, receivedTask)
}

func (s *TaskRequestHandlerTestSuite) Test_TaskRequestHandler_UpdateTaskStatus() {
	testCases := []struct {
		name   string
		result types.ProverTaskResult
	}{
		{
			"success result FinalProof",
			types.SuccessTaskResult(randomTaskId(), randomProverId(), types.FinalProof, "1A2B3C4D"),
		},
		{
			"success result Commitment",
			types.SuccessTaskResult(randomTaskId(), randomProverId(), types.Commitment, "1A2B3C4D"),
		},
		{
			"failure result",
			types.FailureTaskResult(randomTaskId(), randomProverId(), errors.New("something went wrong")),
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.testSetTaskStatus(testCase.result)
		})
	}
}

func (s *TaskRequestHandlerTestSuite) testSetTaskStatus(resultToSend types.ProverTaskResult) {
	s.T().Helper()

	err := s.clientHandler.SetTaskResult(s.context, &resultToSend)
	s.Require().NoError(err)

	setResultCalls := s.targetHandler.SetTaskResultCalls()
	s.Require().Len(setResultCalls, 1, "expected one call to SetTaskResult")
	s.Require().Equal(&resultToSend, setResultCalls[0].Result)
}
