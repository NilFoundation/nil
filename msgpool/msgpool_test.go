package msgpool

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteMsgPool struct {
	suite.Suite
	db   db.DB
	pool *MsgPool
}

func newMessage(from types.Address, seqno uint64, fee uint64) types.Message {
	return types.Message{
		From:      from,
		To:        types.Address{},
		Value:     types.Uint256{Int: *uint256.NewInt(fee)},
		Data:      types.Code(""),
		Seqno:     seqno,
		Signature: common.EmptySignature,
	}
}

func (suite *SuiteMsgPool) BeforeTest(suiteName, testName string) {
	var err error
	suite.db, err = db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.pool = New(DefaultConfig)
	suite.Require().NotNil(suite.pool)
	suite.Equal(0, suite.pool.MessageCount())
}

func (suite *SuiteMsgPool) AfterTest(suiteName, testName string) {
	suite.db.Close()
}

func (suite *SuiteMsgPool) TestAdd() {
	suite.Equal(0, suite.pool.MessageCount())

	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	address := types.HexToAddress("deadbeef")
	suite.Require().NotEqual(types.Address{}, address)

	msg1 := newMessage(address, 0, 123)

	// Send message for the first time - OK
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Send message once again - Duplicate hash
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg1}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{DuplicateHash}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Try to replace message but with lower fee - NotReplaced
	msg2 := newMessage(address, 0, 122)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg2}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotReplaced}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Try to replace message but with higher fee - OK
	msg3 := newMessage(address, 0, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg3}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(1, suite.pool.MessageCount())

	// Add a message with higher seqno from the same sender
	msg4 := newMessage(address, 1, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg4}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(2, suite.pool.MessageCount())

	// Add a message with lower seqno from the same sender - SeqnoTooLow
	msg5 := newMessage(address, 0, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg5}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{SeqnoTooLow}, reasons)
	suite.Equal(2, suite.pool.MessageCount())

	// Add a message with higher seqno from new sender
	msg6 := newMessage(types.HexToAddress("deadbeef2"), 1, 124)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg6}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)
	suite.Equal(3, suite.pool.MessageCount())

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteMsgPool) TestAddOverflow() {
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	suite.pool.cfg.Size = 1

	address := types.HexToAddress("deadbeef")
	suite.Require().NotEqual(types.Address{}, address)

	msg1 := newMessage(address, 0, 123)
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	msg2 := newMessage(address, 1, 123)
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg2}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{PoolOverflow}, reasons)

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteMsgPool) TestStarted() {
	suite.True(suite.pool.Started())
}

func (suite *SuiteMsgPool) TestIdHashKnownGet() {
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	address := types.HexToAddress("deadbeef01")
	suite.Require().NotEqual(types.Address{}, address)

	msg := newMessage(address, 0, 123)
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	has, err := suite.pool.IdHashKnown(tx, msg.Hash())
	suite.Require().NoError(err)
	suite.Require().True(has)

	poolMsg, err := suite.pool.Get(tx, msg.Hash())
	suite.Require().NoError(err)
	suite.Require().NotNil(poolMsg)
	suite.Equal(msg, *poolMsg)

	has, err = suite.pool.IdHashKnown(tx, common.BytesToHash([]byte("abcd")))
	suite.Require().NoError(err)
	suite.Require().False(has)

	poolMsg, err = suite.pool.Get(tx, common.BytesToHash([]byte("abcd")))
	suite.Require().NoError(err)
	suite.Require().Nil(poolMsg)

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteMsgPool) TestSeqnoFromAddress() {
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	address := types.HexToAddress("deadbeef02")
	suite.Require().NotEqual(types.Address{}, address)

	_, inPool := suite.pool.SeqnoFromAddress(address)
	suite.Require().False(inPool)

	msg := newMessage(address, 0, 123)
	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	seqno, inPool := suite.pool.SeqnoFromAddress(address)
	suite.Require().True(inPool)
	suite.Require().Equal(uint64(0), seqno)

	msg.Seqno++
	reasons, err = suite.pool.Add(ctx, []*types.Message{&msg}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet}, reasons)

	seqno, inPool = suite.pool.SeqnoFromAddress(address)
	suite.Require().True(inPool)
	suite.Require().Equal(uint64(1), seqno)

	_, inPool = suite.pool.SeqnoFromAddress(types.BytesToAddress([]byte("abcd")))
	suite.Require().False(inPool)

	err = tx.Commit()
	suite.Require().NoError(err)
}

func (suite *SuiteMsgPool) TestPeek() {
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	address1 := types.HexToAddress("deadbeef01")
	address2 := types.HexToAddress("deadbeef02")

	msg1_1 := newMessage(address1, 0, 123)
	msg1_2 := newMessage(address1, 1, 123)

	msg2_1 := newMessage(address2, 0, 123)
	msg2_2 := newMessage(address2, 1, 123)

	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1_1, &msg1_2, &msg2_1, &msg2_2}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet, NotSet, NotSet, NotSet}, reasons)
	suite.Equal(4, suite.pool.MessageCount())

	err = tx.Commit()
	suite.Require().NoError(err)

	msgs, err := suite.pool.Peek(ctx, 1, 0)
	suite.Require().NoError(err)
	suite.Require().Len(msgs, 1)

	msgs, err = suite.pool.Peek(ctx, 4, 0)
	suite.Require().NoError(err)
	suite.Require().Len(msgs, 4)

	msgs, err = suite.pool.Peek(ctx, 10, 0)
	suite.Require().NoError(err)
	suite.Require().Len(msgs, 4)

	suite.Equal(4, suite.pool.MessageCount())
}

func (suite *SuiteMsgPool) TestOnNewBlock() {
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	address1 := types.HexToAddress("deadbeef01")
	address2 := types.HexToAddress("deadbeef02")

	msg1_1 := newMessage(address1, 0, 123)
	msg1_2 := newMessage(address1, 1, 123)

	msg2_1 := newMessage(address2, 0, 123)
	msg2_2 := newMessage(address2, 1, 123)

	reasons, err := suite.pool.Add(ctx, []*types.Message{&msg1_1, &msg1_2, &msg2_1, &msg2_2}, tx)
	suite.Require().NoError(err)
	suite.Require().Equal([]DiscardReason{NotSet, NotSet, NotSet, NotSet}, reasons)
	suite.Equal(4, suite.pool.MessageCount())

	// Attempt to peek messages for new block - wait until it's handled
	ch := make(chan []*types.Message)
	go func(ch chan []*types.Message) {
		ctx := context.Background()
		msgs, err := suite.pool.Peek(ctx, 4, 1)
		if err == nil {
			ch <- msgs
		}
	}(ch)

	time.Sleep(100 * time.Millisecond)
	select {
	case <-ch:
		suite.Fail("Channel expected to be empty")
	default:
	}

	// TODO: Ideally we need to do that via execution state
	block := types.Block{Id: 1}
	err = suite.pool.OnNewBlock(ctx, &block, []*types.Message{&msg1_1, &msg1_2, &msg2_1}, tx)
	suite.Require().NoError(err)

	time.Sleep(100 * time.Millisecond)

	// After commit Peek should return only one message
	var messages []*types.Message
	select {
	case messages = <-ch:
	default:
		suite.Fail("Channel expected to have messages for block 1")
	}

	suite.Require().Len(messages, 1)
	suite.Require().Equal([]*types.Message{&msg2_2}, messages)
	suite.Equal(1, suite.pool.MessageCount())

	err = tx.Commit()
	suite.Require().NoError(err)
}

func TestSuiteMsgpool(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMsgPool))
}
