package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
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

	m1 := types.Message{
		From:     types.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20"),
		Data:     hexutil.FromHex("6009600c60003960096000f3600054600101600055"),
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

	addrFrom := types.HexToAddress("0000832983856CB0CF6CD570F071122F1BEA2F20")
	es.CreateAccount(addrFrom)

	addrTo := types.HexToAddress("1111832983856CB0CF6CD570F071122F1BEA2F20")
	es.CreateAccount(addrTo)

	msg := &types.Message{
		From:     types.EmptyAddress,
		To:       addrTo,
		Seqno:    0,
		Internal: false,
	}

	// "From" doesn't exist
	ok, err := validateMessage(tx, es, msg)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 1)
	s.False(es.Receipts[0].Success)

	// Invalid signature
	msg.From = addrFrom
	ok, err = validateMessage(tx, es, msg)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 2)
	s.False(es.Receipts[1].Success)

	// contract that always returns "true",
	// so verifies any message
	es.SetCode(addrTo, hexutil.FromHex("600160005260206000f3"))
	ok, err = validateMessage(tx, es, msg)
	s.Require().NoError(err)
	s.True(ok)
	s.Len(es.Receipts, 2)

	// Gap in seqno
	msg.Seqno = 100
	ok, err = validateMessage(tx, es, msg)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 3)
	s.False(es.Receipts[2].Success)
}

func (s *MessagesSuite) TestValidateDeployMessage() {
	tx, err := s.db.CreateRwTx(context.Background())
	s.Require().NoError(err)
	defer tx.Rollback()

	es, err := execution.NewExecutionStateForShard(tx, types.BaseShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(es)

	dmMaster := &types.DeployMessage{}
	dataMaster, err := dmMaster.MarshalSSZ()
	s.Require().NoError(err)

	dmBase := &types.DeployMessage{}
	dataBase, err := dmBase.MarshalSSZ()
	s.Require().NoError(err)

	msg := &types.Message{
		Data: types.Code("invalid-ssz"),
	}

	// Invalid SSZ
	dm := validateDeployMessage(es, msg)
	s.Require().Nil(dm)

	// Deploy to master shard
	msg.Data = dataMaster
	msg.To = types.CreateAddress(types.MasterShardId, nil)
	dm = validateDeployMessage(es, msg)
	s.Require().Nil(dm)

	// Deploy to base shard
	msg.Data = dataBase
	msg.To = types.CreateAddress(types.BaseShardId, nil)
	dm = validateDeployMessage(es, msg)
	s.Require().NotNil(dm)
}

func TestMessages(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MessagesSuite))
}
