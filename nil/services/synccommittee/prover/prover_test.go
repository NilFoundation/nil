package prover

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/listener"
	"github.com/stretchr/testify/suite"
)

func TestProverSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TestSuite))
}

func (s *TestSuite) Test_Prover_Handles_Tasks() {
	for range 3 {
		task, err := listener.GenerateTask()
		s.Require().NoError(err)
		s.targetHandler.AddTask(task)
	}

	go func() {
		err := s.prover.Run(s.context)
		s.NoError(err)
	}()

	s.Require().Eventually(
		func() bool {
			for _, value := range *s.targetHandler.GetAllEntries() {
				if value.ProverId == nil || *(value.ProverId) != s.prover.nonceId {
					return false
				}
			}
			return true
		},
		time.Second,
		10*time.Millisecond,
	)
}
