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
		From: common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20"),
		Data: hexutil.FromHex("6009600c60003960096000f3600054600101600055"),
	}
	m2 := m1
	m2.To = common.CreateAddress(uint32(shardId), m2.From, m2.Seqno)

	err = HandleMessages(ctx, es, []*types.Message{&m1, &m2})
	s.Require().NoError(err)

	r := es.Receipts[0]
	s.Equal(uint64(0), r.MsgIndex)
	s.Equal(m1.Hash(), r.MsgHash)

	r = es.Receipts[1]
	s.Equal(uint64(1), r.MsgIndex)
	s.Equal(m2.Hash(), r.MsgHash)
}

func (s *SuiteShardchainState) TestValidateMessage() {
	tx, err := s.db.CreateRwTx(context.Background())
	s.Require().NoError(err)

	es, err := execution.NewExecutionStateForShard(tx, types.MasterShardId, common.NewTestTimer(0))
	s.Require().NoError(err)
	s.Require().NotNil(es)

	key, err := crypto.GenerateKey()
	s.Require().NoError(err)

	addrFrom := common.HexToAddress("0000832983856CB0CF6CD570F071122F1BEA2F20")
	es.CreateAccount(addrFrom)
	es.Accounts[addrFrom].PublicKey = crypto.CompressPubkey(&key.PublicKey)

	addrTo := common.HexToAddress("1111832983856CB0CF6CD570F071122F1BEA2F20")
	es.CreateAccount(addrTo)

	msg := types.Message{
		From:      common.EmptyAddress,
		To:        addrTo,
		Seqno:     0,
		Signature: common.EmptySignature,
	}

	// "From" doesn't exist
	ok, err := validateMessage(es, &msg, 0)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 1)
	s.False(es.Receipts[0].Success)

	// Invalid signature
	msg.From = addrFrom
	ok, err = validateMessage(es, &msg, 1)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 2)
	s.False(es.Receipts[1].Success)

	// Signed message - OK
	s.Require().NoError(msg.Sign(key))
	ok, err = validateMessage(es, &msg, 2)
	s.Require().NoError(err)
	s.True(ok)
	s.Len(es.Receipts, 2)

	// Gap in seqno
	msg.Seqno = 100
	ok, err = validateMessage(es, &msg, 3)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 3)
	s.False(es.Receipts[2].Success)
}

func TestSuiteShardchainState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteShardchainState))
}
