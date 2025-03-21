package l2

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type TransactionSenderTestSuite struct {
	suite.Suite
}

func TestTransactionSender(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TransactionSenderTestSuite))
}

func (s *TransactionSenderTestSuite) TestTrivial() {
	s.True(false, "implement me!")
}

// TODO (oclaw) test case:
//
// 1. Basic
// - add pending events
// - trigger polling
// - check that events are relayed and dropped
//
// 2. Failure
// - add pending events
// - trigger failure in the middle
// - check that events before failrure are dropped and after are not
//
// 3. Batched
// - do basic test with event count > batch size of the sender
