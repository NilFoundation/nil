package listener

import (
	"testing"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/stretchr/testify/suite"
)

func TestTaskObserverSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskObserverTestSuite))
}

func (s *TaskObserverTestSuite) Test_TaskObserver_GetTask() {
	proverId := api.ProverNonceId(10)

	addedTask, err := GenerateTask()
	s.Require().NoError(err)
	s.targetObserver.AddTask(addedTask)

	receivedTask, err := s.clientObserver.GetTask(s.context, api.NewTaskRequest(proverId))
	s.Require().NoError(err)
	s.Require().NotNil(receivedTask)
	s.Equal(receivedTask.Id, addedTask.Id)

	entry := s.targetObserver.GetTaskEntry(receivedTask.Id)
	s.Require().NotNil(entry)
	s.Equal(proverId, *entry.ProverId)
}

func (s *TaskObserverTestSuite) Test_TaskObserver_UpdateTaskStatus() {
	proverId := api.ProverNonceId(10)

	addedTask, err := GenerateTask()
	s.Require().NoError(err)
	s.targetObserver.AddTask(addedTask)

	_, err = s.clientObserver.GetTask(s.context, api.NewTaskRequest(proverId))
	s.Require().NoError(err)

	updateStatus := func(newStatus api.ProverTaskStatus) {
		request := api.NewTaskStatusUpdateRequest(proverId, addedTask.Id, newStatus)
		err := s.clientObserver.UpdateTaskStatus(s.context, request)
		s.Require().NoError(err)
		entry := s.targetObserver.GetTaskEntry(addedTask.Id)
		s.Require().NotNil(entry)
		s.Equal(newStatus, entry.Status)
	}

	updateStatus(api.Running)
	updateStatus(api.Done)
}
