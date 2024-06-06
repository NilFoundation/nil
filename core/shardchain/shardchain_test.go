package shardchain

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type SuiteShardchainState struct {
	suite.Suite
	db db.DB
}

func (s *SuiteShardchainState) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *SuiteShardchainState) TearDownTest() {
	s.db.Close()
}

func (s *SuiteShardchainState) TestGenerateBlock() {
	ctx := context.Background()
	shardId := types.ShardId(1)
	shard := NewShardChain(shardId, s.db)
	s.Require().NotNil(shard)

	rwTx, err := shard.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer rwTx.Rollback()

	es, err := execution.NewExecutionStateForShard(rwTx, shardId, common.NewTimer())
	s.Require().NoError(err)

	m1 := types.Message{
		From: types.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20"),
		Data: hexutil.FromHex("6009600c60003960096000f3600054600101600055"),
	}
	m2 := m1
	m2.To = types.CreateAddress(shardId, m2.From, m2.Seqno)

	err = HandleMessages(ctx, es, []*types.Message{&m1, &m2})
	s.Require().NoError(err)

	r := es.Receipts[0]
	s.Equal(m1.Hash(), r.MsgHash)

	r = es.Receipts[1]
	s.Equal(m2.Hash(), r.MsgHash)
}

func (s *SuiteShardchainState) TestValidateMessage() {
	tx, err := s.db.CreateRwTx(context.Background())
	s.Require().NoError(err)

	es, err := execution.NewExecutionStateForShard(tx, types.BaseShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(es)

	key, err := crypto.GenerateKey()
	s.Require().NoError(err)

	addrFrom := types.HexToAddress("0000832983856CB0CF6CD570F071122F1BEA2F20")
	es.CreateAccount(addrFrom)
	es.Accounts[addrFrom].PublicKey = [types.PublicKeySize]byte(crypto.CompressPubkey(&key.PublicKey))

	addrTo := types.HexToAddress("1111832983856CB0CF6CD570F071122F1BEA2F20")
	es.CreateAccount(addrTo)

	msg := &types.Message{
		From:      types.EmptyAddress,
		To:        addrTo,
		Seqno:     0,
		Signature: common.EmptySignature,
	}

	// "From" doesn't exist
	ok, err := validateMessage(es, msg)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 1)
	s.False(es.Receipts[0].Success)

	// Invalid signature
	msg.From = addrFrom
	ok, err = validateMessage(es, msg)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 2)
	s.False(es.Receipts[1].Success)

	// Signed message - OK
	s.Require().NoError(msg.Sign(key))
	ok, err = validateMessage(es, msg)
	s.Require().NoError(err)
	s.True(ok)
	s.Len(es.Receipts, 2)

	// Gap in seqno
	msg.Seqno = 100
	ok, err = validateMessage(es, msg)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 3)
	s.False(es.Receipts[2].Success)
}

func (s *SuiteShardchainState) TestValidateDeployMessage() {
	tx, err := s.db.CreateRwTx(context.Background())
	s.Require().NoError(err)

	es, err := execution.NewExecutionStateForShard(tx, types.BaseShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(es)

	dmMaster := &types.DeployMessage{
		ShardId: types.MasterShardId,
		Seqno:   100500,
	}
	dataMaster, err := dmMaster.MarshalSSZ()
	s.Require().NoError(err)

	dmBase := &types.DeployMessage{
		ShardId: types.BaseShardId,
		Seqno:   100501,
	}
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
	dm = validateDeployMessage(es, msg)
	s.Require().Nil(dm)

	// Deploy to base shard
	msg.Data = dataBase
	dm = validateDeployMessage(es, msg)
	s.Require().NotNil(dm)
	s.Equal(dmBase.Seqno, dm.Seqno)
	s.Equal(dmBase.ShardId, dm.ShardId)
}

func TestSuiteShardchainState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteShardchainState))
}
