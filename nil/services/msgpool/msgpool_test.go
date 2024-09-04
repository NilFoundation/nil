package msgpool

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteMsgPool struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	pool *MsgPool
}

func newMessage(seqno types.Seqno, fee uint64) *types.Message {
	address := types.ShardAndHexToAddress(0, "deadbeef")
	return &types.Message{
		To:    address,
		Value: types.NewValueFromUint64(fee),
		Seqno: seqno,
	}
}

func (s *SuiteMsgPool) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	var err error
	s.pool, err = New(s.ctx, NewConfig(0), nil)
	s.Require().NoError(err)
}

func (s *SuiteMsgPool) TearDownTest() {
	s.cancel()
}

func (s *SuiteMsgPool) addMessagesToPoolSuccessfully(pool Pool, msg ...*types.Message) {
	s.T().Helper()

	count := pool.MessageCount()

	reasons, err := pool.Add(s.ctx, msg...)
	s.Require().NoError(err)
	s.Require().Len(reasons, len(msg))
	for _, reason := range reasons {
		s.Equal(NotSet, reason)
	}

	s.Equal(count+len(msg), pool.MessageCount())
}

func (s *SuiteMsgPool) addMessagesSuccessfully(msg ...*types.Message) {
	s.T().Helper()

	s.addMessagesToPoolSuccessfully(s.pool, msg...)
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
	wrongShardMsg := newMessage(0, 123)
	wrongShardMsg.To = types.ShardAndHexToAddress(1, "deadbeef")
	_, err := s.pool.Add(s.ctx, wrongShardMsg)
	s.Require().Error(err)

	msg1 := newMessage(0, 123)

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
		newMessage(1, 124))

	// Add a message with lower seqno to the same receiver - SeqnoTooLow
	s.addMessageWithDiscardReason(
		newMessage(0, 124), SeqnoTooLow)

	// Add a message with higher seqno to a new receiver
	otherAddressMsg := newMessage(1, 124)
	otherAddressMsg.To = types.ShardAndHexToAddress(0, "deadbeef01")
	s.addMessagesSuccessfully(otherAddressMsg)
}

func (s *SuiteMsgPool) TestAddOverflow() {
	s.pool.cfg.Size = 1

	s.addMessagesSuccessfully(
		newMessage(0, 123))

	s.addMessageWithDiscardReason(
		newMessage(1, 123), PoolOverflow)
}

func (s *SuiteMsgPool) TestStarted() {
	s.True(s.pool.Started())
}

func (s *SuiteMsgPool) TestIdHashKnownGet() {
	msg := newMessage(0, 123)
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
	msg := newMessage(0, 123)

	_, inPool := s.pool.SeqnoToAddress(msg.To)
	s.Require().False(inPool)

	s.addMessagesSuccessfully(msg)

	seqno, inPool := s.pool.SeqnoToAddress(msg.To)
	s.Require().True(inPool)
	s.Require().EqualValues(0, seqno)

	nextMsg := common.CopyPtr(msg)
	nextMsg.Seqno++
	s.addMessagesSuccessfully(nextMsg)

	seqno, inPool = s.pool.SeqnoToAddress(msg.To)
	s.Require().True(inPool)
	s.Require().EqualValues(1, seqno)

	_, inPool = s.pool.SeqnoToAddress(types.BytesToAddress([]byte("abcd")))
	s.Require().False(inPool)
}

func (s *SuiteMsgPool) TestPeek() {
	address2 := types.ShardAndHexToAddress(0, "deadbeef02")

	msg21 := newMessage(0, 123)
	msg21.To = address2
	msg22 := newMessage(1, 123)
	msg22.To = address2

	s.addMessagesSuccessfully(
		newMessage(0, 123),
		newMessage(1, 123),
		msg21,
		msg22)

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
	address2 := types.ShardAndHexToAddress(0, "deadbeef02")

	msg11 := newMessage(0, 123)
	msg12 := newMessage(1, 123)

	msg21 := newMessage(0, 123)
	msg21.To = address2
	msg22 := newMessage(1, 123)
	msg22.To = address2

	s.addMessagesSuccessfully(msg11, msg12, msg21, msg22)

	// TODO: Ideally we need to do that via execution state
	err := s.pool.OnCommitted(s.ctx, []*types.Message{msg11, msg12, msg21})
	s.Require().NoError(err)

	// After commit Peek should return only one message
	messages, err := s.pool.Peek(s.ctx, 10)
	s.Require().NoError(err)
	s.Require().Len(messages, 1)
	s.Equal(msg22, messages[0])
	s.Equal(1, s.pool.MessageCount())
}

func (s *SuiteMsgPool) TestNetwork() {
	nms := network.NewTestManagers(s.T(), s.ctx, 2)

	pool1, err := New(s.ctx, NewConfig(0), nms[0])
	s.Require().NoError(err)
	pool2, err := New(s.ctx, NewConfig(0), nms[1])
	s.Require().NoError(err)

	// Ensure that both nodes have subscribed, so that they will exchange this info on the following connect.
	s.Require().Eventually(func() bool {
		return slices.Contains(nms[0].PubSub().Topics(), topicPendingMessages(0)) &&
			slices.Contains(nms[1].PubSub().Topics(), topicPendingMessages(0))
	}, 1*time.Second, 50*time.Millisecond)

	network.ConnectManagers(s.T(), nms[0], nms[1])

	msg := newMessage(0, 123)
	s.addMessagesToPoolSuccessfully(pool1, msg)

	s.Eventually(func() bool {
		has, err := pool2.IdHashKnown(msg.Hash())
		s.Require().NoError(err)
		return has
	}, 10*time.Second, 100*time.Millisecond)
}

func TestSuiteMsgpool(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMsgPool))
}
