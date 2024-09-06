package listener

import (
	"testing"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

func TestTaskRequestHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskRequestHandlerTestSuite))
}

func (s *TaskRequestHandlerTestSuite) Test_TaskRequestHandler_GetTask() {
	proverId := types.ProverId(10)

	addedTask, err := GenerateTask()
	s.Require().NoError(err)
	s.targetHandler.AddTask(addedTask)

	receivedTask, err := s.clientHandler.GetTask(s.context, api.NewTaskRequest(proverId))
	s.Require().NoError(err)
	s.Require().NotNil(receivedTask)
	s.Equal(receivedTask, addedTask)

	entry := s.targetHandler.GetTaskEntry(receivedTask.Id)
	s.Require().NotNil(entry)
	s.Equal(proverId, *entry.ProverId)
}

func (s *TaskRequestHandlerTestSuite) Test_TaskRequestHandler_UpdateTaskStatus() {
	proverId := types.ProverId(10)

	addedTask, err := GenerateTask()
	s.Require().NoError(err)
	s.targetHandler.AddTask(addedTask)

	_, err = s.clientHandler.GetTask(s.context, api.NewTaskRequest(proverId))
	s.Require().NoError(err)

	sentResult := types.SuccessTaskResult(addedTask.Id, proverId, types.FinalProof, "")

	err = s.clientHandler.SetTaskResult(s.context, &sentResult)
	s.Require().NoError(err)

	receivedResult := s.targetHandler.GetTaskEntry(addedTask.Id).Result
	s.Require().Equal(sentResult, *receivedResult)
}
