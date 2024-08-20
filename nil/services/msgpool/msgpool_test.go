package msgpool

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteMsgPool struct {
	suite.Suite

	pool *MsgPool
}

func newMessage(to types.Address, seqno types.Seqno, fee uint64) types.Message {
	return types.Message{
		To:    to,
		Value: types.NewValueFromUint64(fee),
		Data:  types.Code(""),
		Seqno: seqno,
	}
}

func (suite *SuiteMsgPool) BeforeTest(suiteName, testName string) {
	suite.pool = New(DefaultConfig)
	suite.Require().NotNil(suite.pool)
	suite.Equal(0, suite.pool.MessageCount())
}

func (suite *SuiteMsgPool) TestAdd() {
	suite.Equal(0, suite.pool.MessageCount())

	ctx := context.Background()

	address := types.HexToAddress("deadbeef")
	suite.Require().NotEqual(types.Address{}, address)

	msg1 := newMessage(address, 0, 123)

	// Send the message for the first time - OK
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Send message once again - Duplicate hash
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg1})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{DuplicateHash}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Send the same message with higher fee - OK
	msg2 := msg1
	msg2.FeeCredit = msg2.FeeCredit.Add64(1)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg2})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Send a different message with the same seqno - NotReplaced
	msg3 := msg1
	// Force the message to be different
	msg3.Data = append(msg3.Data, 0x01)
	suite.Require().NotEqual(msg1.Hash(), msg3.Hash())
	// Add a higher fee (otherwise, no replacement can be expected anyway)
	msg3.FeeCredit = msg3.FeeCredit.Add64(1)

	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg3})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotReplaced}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Add a message with higher seqno to the same receiver
	msg4 := newMessage(address, 1, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg4})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(2, suite.pool.MessageCount())

	// Add a message with lower seqno to the same receiver - SeqnoTooLow
	msg5 := newMessage(address, 0, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg5})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{SeqnoTooLow}, reasons)
	suite.Equal(2, suite.pool.MessageCount())

	// Add a message with higher seqno to new receiver
	msg6 := newMessage(types.HexToAddress("deadbeef02"), 1, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg6})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(3, suite.pool.MessageCount())
}

func (suite *SuiteMsgPool) TestAddOverflow() {
	ctx := context.Background()

	suite.pool.cfg.Size = 1

	address := types.HexToAddress("deadbeef")
	suite.Require().NotEqual(types.Address{}, address)

	msg1 := newMessage(address, 0, 123)
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	msg2 := newMessage(address, 1, 123)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg2})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{PoolOverflow}, reasons)
}

func (suite *SuiteMsgPool) TestStarted() {
	suite.True(suite.pool.Started())
}

func (suite *SuiteMsgPool) TestIdHashKnownGet() {
	ctx := context.Background()

	address := types.HexToAddress("deadbeef01")
	suite.Require().NotEqual(types.Address{}, address)

	msg := newMessage(address, 0, 123)
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	has, err := suite.pool.IdHashKnown(msg.Hash())
	suite.Require().NoError(err)
	suite.Require().True(has)

	poolMsg, err := suite.pool.Get(msg.Hash())
	suite.Require().NoError(err)
	suite.Require().NotNil(poolMsg)
	suite.Equal(msg, *poolMsg)

	has, err = suite.pool.IdHashKnown(common.BytesToHash([]byte("abcd")))
	suite.Require().NoError(err)
	suite.Require().False(has)

	poolMsg, err = suite.pool.Get(common.BytesToHash([]byte("abcd")))
	suite.Require().NoError(err)
	suite.Require().Nil(poolMsg)
}

func (suite *SuiteMsgPool) TestSeqnoFromAddress() {
	ctx := context.Background()

	address := types.HexToAddress("deadbeef02")
	suite.Require().NotEqual(types.Address{}, address)

	_, inPool := suite.pool.SeqnoToAddress(address)
	suite.Require().False(inPool)

	msg := newMessage(address, 0, 123)
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	seqno, inPool := suite.pool.SeqnoToAddress(address)
	suite.Require().True(inPool)
	suite.Require().EqualValues(0, seqno)

	nextMsg := msg
	nextMsg.Seqno++
	reasons, err = suite.pool.Add(ctx, []*types.Message{&nextMsg})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	seqno, inPool = suite.pool.SeqnoToAddress(address)
	suite.Require().True(inPool)
	suite.Require().EqualValues(1, seqno)

	_, inPool = suite.pool.SeqnoToAddress(types.BytesToAddress([]byte("abcd")))
	suite.Require().False(inPool)
}

func (suite *SuiteMsgPool) TestPeek() {
	ctx := context.Background()

	address1 := types.HexToAddress("deadbeef01")
	address2 := types.HexToAddress("deadbeef02")

	msg1_1 := newMessage(address1, 0, 123)
	msg1_2 := newMessage(address1, 1, 123)

	msg2_1 := newMessage(address2, 0, 123)
	msg2_2 := newMessage(address2, 1, 123)

	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1_1, &msg1_2, &msg2_1, &msg2_2})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet, NotSet, NotSet, NotSet}, reasons)
	suite.Equal(4, suite.pool.MessageCount())

	msgs, err := suite.pool.Peek(ctx, 1)
	suite.Require().NoError(err)
	suite.Require().Len(msgs, 1)

	msgs, err = suite.pool.Peek(ctx, 4)
	suite.Require().NoError(err)
	suite.Require().Len(msgs, 4)

	msgs, err = suite.pool.Peek(ctx, 10)
	suite.Require().NoError(err)
	suite.Require().Len(msgs, 4)

	suite.Equal(4, suite.pool.MessageCount())
}

func (suite *SuiteMsgPool) TestOnNewBlock() {
	ctx := context.Background()

	address1 := types.HexToAddress("deadbeef01")
	address2 := types.HexToAddress("deadbeef02")

	msg1_1 := newMessage(address1, 0, 123)
	msg1_2 := newMessage(address1, 1, 123)

	msg2_1 := newMessage(address2, 0, 123)
	msg2_2 := newMessage(address2, 1, 123)

	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1_1, &msg1_2, &msg2_1, &msg2_2})
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet, NotSet, NotSet, NotSet}, reasons)
	suite.Equal(4, suite.pool.MessageCount())

	// TODO: Ideally we need to do that via execution state
	err = suite.pool.OnCommitted(ctx, []*types.Message{&msg1_1, &msg1_2, &msg2_1})
	suite.Require().NoError(err)

	time.Sleep(100 * time.Millisecond)

	// After commit Peek should return only one message
	messages, err := suite.pool.Peek(ctx, 10)
	suite.Require().NoError(err)
	suite.Require().Len(messages, 1)
	suite.Require().Equal([]*types.Message{&msg2_2}, messages)
	suite.Equal(1, suite.pool.MessageCount())
}

func TestSuiteMsgpool(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMsgPool))
}
