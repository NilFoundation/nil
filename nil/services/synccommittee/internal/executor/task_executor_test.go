package executor

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/stretchr/testify/suite"
)

func TestTaskExecutorSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) Test_TaskExecutor_Executes_Tasks() {
	go func() {
		err := s.taskExecutor.Run(s.context)
		s.NoError(err)
	}()

	expectedTaskRequest := api.NewTaskRequest(s.taskExecutor.Id())
	const tasksThreshold = 3

	s.Require().Eventually(
		func() bool {
			getTaskCalls := s.targetHandler.GetTaskCalls()
			if len(getTaskCalls) < tasksThreshold {
				return false
			}

			for _, call := range getTaskCalls {
				s.Require().Equal(expectedTaskRequest, call.Request, "Task executor should have passed its id to the target handler")
			}

			return true
		},
		time.Second,
		10*time.Millisecond,
	)

	s.Require().Eventually(
		func() bool {
			setTaskResultCalls := s.targetHandler.SetTaskResultCalls()
			if len(setTaskResultCalls) < tasksThreshold {
				return false
			}

			for _, call := range setTaskResultCalls {
				s.Require().Equal(s.taskExecutor.Id(), call.Result.Sender, "Task executor should have passed its id in the result")
			}

			return true
		},
		time.Second,
		10*time.Millisecond,
	)
}
