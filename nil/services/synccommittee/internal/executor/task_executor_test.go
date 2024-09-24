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
	const tasksThreshold = 5

	s.Require().Eventually(
		func() bool {
			getTaskCalls := s.requestHandler.GetTaskCalls()
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
			taskHandleCalls := s.taskHandler.HandleCalls()
			if len(taskHandleCalls) < tasksThreshold {
				return false
			}

			for _, call := range taskHandleCalls {
				s.Require().Equal(s.taskExecutor.Id(), call.ExecutorId, "Task executor should have passed its id in the result")
			}

			return true
		},
		time.Second,
		10*time.Millisecond,
	)
}
