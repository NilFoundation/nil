package prover

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/stretchr/testify/suite"
)

func TestProverSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) Test_Prover_Handles_Tasks() {
	go func() {
		err := s.prover.Run(s.context)
		s.NoError(err)
	}()

	expectedTaskRequest := api.NewTaskRequest(s.prover.nonceId)
	const tasksThreshold = 3

	s.Require().Eventually(
		func() bool {
			getTaskCalls := s.targetHandler.GetTaskCalls()
			if len(getTaskCalls) < tasksThreshold {
				return false
			}

			for _, call := range getTaskCalls {
				s.Require().Equal(expectedTaskRequest, call.Request, "prover should have passed its id to the target handler")
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
				s.Require().Equal(s.prover.nonceId, call.Result.Sender, "prover should have passed its id in the result")
			}

			return true
		},
		time.Second,
		10*time.Millisecond,
	)
}
