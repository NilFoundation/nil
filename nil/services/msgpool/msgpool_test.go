package msgpool

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteMsgPool struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	pool *MsgPool
}

func newMessage(to types.Address, seqno types.Seqno, fee uint64) *types.Message {
	return &types.Message{
		To:    to,
		Value: types.NewValueFromUint64(fee),
		Seqno: seqno,
	}
}

func (s *SuiteMsgPool) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.pool = New(NewConfig(0))
}

func (s *SuiteMsgPool) TearDownTest() {
	s.cancel()
}

func (s *SuiteMsgPool) addMessagesSuccessfully(msg ...*types.Message) {
	s.T().Helper()

	count := s.pool.MessageCount()

	reasons, err := s.pool.Add(s.ctx, msg...)
	s.Require().NoError(err)
	s.Require().Len(reasons, len(msg))
	for _, reason := range reasons {
		s.Equal(NotSet, reason)
	}

	s.Equal(count+len(msg), s.pool.MessageCount())
}

func (s *SuiteMsgPool) addMessageWithDiscardReason(msg *types.Message, reason DiscardReason) {
	s.T().Helper()

	count := s.pool.MessageCount()

	reasons, err := s.pool.Add(s.ctx, msg)
	s.Require().NoError(err)
	s.Equal([]DiscardReason{reason}, reasons)

	s.Equal(count, s.pool.MessageCount())
}

func (s *SuiteMsgPool) TestAdd() {
	address := types.HexToAddress("deadbeef")

	msg1 := newMessage(address, 0, 123)

	// Send the message for the first time - OK
	s.addMessagesSuccessfully(msg1)

	// Send message once again - Duplicate hash
	s.addMessageWithDiscardReason(msg1, DuplicateHash)

	// Send the same message with higher fee - OK
	// Doesn't use the same helper because here message count doesn't change
	msg2 := common.CopyPtr(msg1)
	msg2.FeeCredit = msg2.FeeCredit.Add64(1)
	reasons, err := s.pool.Add(s.ctx, msg2)
	s.Require().NoError(err)
	s.Require().Equal([]DiscardReason{NotSet}, reasons)
	s.Equal(1, s.pool.MessageCount())

	// Send a different message with the same seqno - NotReplaced
	msg3 := common.CopyPtr(msg1)
	// Force the message to be different
	msg3.Data = append(msg3.Data, 0x01)
	s.Require().NotEqual(msg1.Hash(), msg3.Hash())
	// Add a higher fee (otherwise, no replacement can be expected anyway)
	msg3.FeeCredit = msg3.FeeCredit.Add64(1)
	s.addMessageWithDiscardReason(msg3, NotReplaced)

	// Add a message with higher seqno to the same receiver
	s.addMessagesSuccessfully(
		newMessage(address, 1, 124))

	// Add a message with lower seqno to the same receiver - SeqnoTooLow
	s.addMessageWithDiscardReason(
		newMessage(address, 0, 124), SeqnoTooLow)

	// Add a message with higher seqno to new receiver
	s.addMessagesSuccessfully(
		newMessage(types.HexToAddress("deadbeef02"), 1, 124))
}

func (s *SuiteMsgPool) TestAddOverflow() {
	s.pool.cfg.Size = 1

	address := types.HexToAddress("deadbeef")

	s.addMessagesSuccessfully(
		newMessage(address, 0, 123))

	s.addMessageWithDiscardReason(
		newMessage(address, 1, 123), PoolOverflow)
}

func (s *SuiteMsgPool) TestStarted() {
	s.True(s.pool.Started())
}

func (s *SuiteMsgPool) TestIdHashKnownGet() {
	address := types.HexToAddress("deadbeef01")

	msg := newMessage(address, 0, 123)
	s.addMessagesSuccessfully(msg)

	has, err := s.pool.IdHashKnown(msg.Hash())
	s.Require().NoError(err)
	s.True(has)

	poolMsg, err := s.pool.Get(msg.Hash())
	s.Require().NoError(err)
	s.Equal(poolMsg, msg)

	has, err = s.pool.IdHashKnown(common.BytesToHash([]byte("abcd")))
	s.Require().NoError(err)
	s.False(has)

	poolMsg, err = s.pool.Get(common.BytesToHash([]byte("abcd")))
	s.Require().NoError(err)
	s.Nil(poolMsg)
}

func (s *SuiteMsgPool) TestSeqnoFromAddress() {
	address := types.HexToAddress("deadbeef02")

	_, inPool := s.pool.SeqnoToAddress(address)
	s.Require().False(inPool)

	msg := newMessage(address, 0, 123)
	s.addMessagesSuccessfully(msg)

	seqno, inPool := s.pool.SeqnoToAddress(address)
	s.Require().True(inPool)
	s.Require().EqualValues(0, seqno)

	nextMsg := common.CopyPtr(msg)
	nextMsg.Seqno++
	s.addMessagesSuccessfully(nextMsg)

	seqno, inPool = s.pool.SeqnoToAddress(address)
	s.Require().True(inPool)
	s.Require().EqualValues(1, seqno)

	_, inPool = s.pool.SeqnoToAddress(types.BytesToAddress([]byte("abcd")))
	s.Require().False(inPool)
}

func (s *SuiteMsgPool) TestPeek() {
	address1 := types.HexToAddress("deadbeef01")
	address2 := types.HexToAddress("deadbeef02")

	s.addMessagesSuccessfully(
		newMessage(address1, 0, 123),
		newMessage(address1, 1, 123),
		newMessage(address2, 0, 123),
		newMessage(address2, 1, 123))

	msgs, err := s.pool.Peek(s.ctx, 1)
	s.Require().NoError(err)
	s.Len(msgs, 1)

	msgs, err = s.pool.Peek(s.ctx, 4)
	s.Require().NoError(err)
	s.Len(msgs, 4)

	msgs, err = s.pool.Peek(s.ctx, 10)
	s.Require().NoError(err)
	s.Len(msgs, 4)
}

func (s *SuiteMsgPool) TestOnNewBlock() {
	address1 := types.HexToAddress("deadbeef01")
	address2 := types.HexToAddress("deadbeef02")

	msg1_1 := newMessage(address1, 0, 123)
	msg1_2 := newMessage(address1, 1, 123)

	msg2_1 := newMessage(address2, 0, 123)
	msg2_2 := newMessage(address2, 1, 123)

	s.addMessagesSuccessfully(msg1_1, msg1_2, msg2_1, msg2_2)

	// TODO: Ideally we need to do that via execution state
	err := s.pool.OnCommitted(s.ctx, []*types.Message{msg1_1, msg1_2, msg2_1})
	s.Require().NoError(err)

	// After commit Peek should return only one message
	messages, err := s.pool.Peek(s.ctx, 10)
	s.Require().NoError(err)
	s.Require().Len(messages, 1)
	s.Equal(msg2_2, messages[0])
	s.Equal(1, s.pool.MessageCount())
}

func TestSuiteMsgpool(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMsgPool))
}
