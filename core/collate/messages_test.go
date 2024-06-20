package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type MessagesSuite struct {
	suite.Suite
	db db.DB
}

func (s *MessagesSuite) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *MessagesSuite) TearDownTest() {
	s.db.Close()
}

func (s *MessagesSuite) TestGenerateBlock() {
	ctx := context.Background()
	shardId := types.ShardId(1)

	rwTx, err := s.db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer rwTx.Rollback()

	es, err := execution.NewExecutionStateForShard(rwTx, shardId, common.NewTimer())
	s.Require().NoError(err)

	code, err := contracts.GetCode("tests/Counter")
	s.Require().NoError(err)

	m1 := types.Message{
		From:     types.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20"),
		Data:     code,
		Internal: true,
	}
	m2 := m1
	m2.To = types.CreateAddress(shardId, []byte("code"))

	err = HandleMessages(ctx, rwTx, es, []*types.Message{&m1, &m2})
	s.Require().NoError(err)

	r := es.Receipts[0]
	s.Equal(m1.Hash(), r.MsgHash)

	r = es.Receipts[1]
	s.Equal(m2.Hash(), r.MsgHash)
}

func (s *MessagesSuite) TestValidateMessage() {
	tx, err := s.db.CreateRwTx(context.Background())
	s.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionStateForShard(tx, types.BaseShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(es)

	addrFrom := types.GenerateRandomAddress(types.BaseShardId)
	es.CreateAccount(addrFrom)

	addrTo := types.GenerateRandomAddress(types.BaseShardId)
	es.CreateAccount(addrTo)

	msg := &types.Message{
		To:       addrTo,
		Seqno:    0,
		Data:     []byte("hello"),
		Internal: false,
	}

	// Invalid signature
	es.AddInMessage(msg)
	ok, payer := validateMessage(tx, es, msg)
	s.Require().Nil(payer)
	s.False(ok)
	s.Require().Len(es.Receipts, 1)
	s.False(es.Receipts[0].Success)

	// contract that always returns "true",
	// so verifies any message
	es.SetCode(addrTo, hexutil.FromHex("600160005260206000f3"))
	es.AddInMessage(msg)
	ok, payer = validateMessage(tx, es, msg)
	s.Require().NotNil(payer)
	s.True(ok)
	s.Len(es.Receipts, 1)

	// Invalid ChainId
	msg.ChainId = 100500
	es.AddInMessage(msg)
	ok, payer = validateMessage(tx, es, msg)
	s.Require().Nil(payer)
	s.False(ok)
	s.Require().Len(es.Receipts, 2)
	s.False(es.Receipts[1].Success)

	// Gap in seqno
	msg.ChainId = types.DefaultChainId
	msg.Seqno = 100
	es.AddInMessage(msg)
	ok, payer = validateMessage(tx, es, msg)
	s.Require().Nil(payer)
	s.False(ok)
	s.Require().Len(es.Receipts, 3)
	s.False(es.Receipts[2].Success)
}

func (s *MessagesSuite) TestValidateDeployMessage() {
	tx, err := s.db.CreateRwTx(context.Background())
	s.Require().NoError(err)
	defer tx.Rollback()

	esMain, err := execution.NewExecutionStateForShard(tx, types.MasterShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(esMain)

	es, err := execution.NewExecutionStateForShard(tx, types.BaseShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(es)

	msg := &types.Message{
		Data: types.Code("no-salt"),
	}

	// data too short
	s.Require().ErrorIs(validateDeployMessage(msg), ErrInvalidPayload)

	// Deploy to the main shard from base shard - FAIL
	data := types.BuildDeployPayload([]byte("some-code"), common.EmptyHash)
	msg.To = types.CreateAddress(types.MasterShardId, data.Bytes())
	msg.Data = data.Bytes()
	s.Require().ErrorIs(validateDeployMessage(msg), ErrDeployToMainShard)

	// Deploy to the main shard from main wallet - OK
	data = types.BuildDeployPayload([]byte("some-code"), common.EmptyHash)
	msg.To = types.CreateAddress(types.MasterShardId, data.Bytes())
	msg.From = types.MainWalletAddress
	msg.Data = data.Bytes()
	s.Require().NoError(validateDeployMessage(msg))

	// Deploy to base shard
	msg.To = types.CreateAddress(types.BaseShardId, data.Bytes())
	s.Require().NoError(validateDeployMessage(msg))
}

func TestMessages(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MessagesSuite))
}
