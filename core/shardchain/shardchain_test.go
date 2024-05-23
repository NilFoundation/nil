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
	shardId := types.ShardId(1)
	shard := NewShardChain(shardId, s.db, 2)
	s.Require().NotNil(shard)

	var m types.Message
	m.From = common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	m.Data = hexutil.FromHex("6009600c60003960096000f3600054600101600055")

	_, err := shard.GenerateBlock(context.Background(), []*types.Message{&m})
	s.Require().NoError(err)

	m.To = execution.CreateAddress(m.From, m.Seqno)

	_, err = shard.GenerateBlock(context.Background(), []*types.Message{&m})
	s.Require().NoError(err)

	tx, err := s.db.CreateRoTx(context.Background())
	s.Require().NoError(err)

	es, err := execution.NewExecutionStateForShard(tx, shardId, common.NewTestTimer(0))
	s.Require().NoError(err)

	r, err := es.GetReceipt(0)
	s.Require().NoError(err)
	s.Equal(uint64(0), r.MsgIndex)
	s.Equal(m.Hash(), r.MsgHash)
	s.Require().NoError(err)
}

func (s *SuiteShardchainState) TestValidateMessage() {
	shard := NewShardChain(types.MasterShardId, s.db, 2)
	s.Require().NotNil(shard)

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
	ok, err := shard.validateMessage(es, &msg, 0)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 1)
	s.False(es.Receipts[0].Success)

	// Invalid signature
	msg.From = addrFrom
	ok, err = shard.validateMessage(es, &msg, 1)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 2)
	s.False(es.Receipts[1].Success)

	// Signed message - OK
	s.Require().NoError(msg.Sign(key))
	ok, err = shard.validateMessage(es, &msg, 2)
	s.Require().NoError(err)
	s.True(ok)
	s.Len(es.Receipts, 2)

	// Gap in seqno
	msg.Seqno = 100
	ok, err = shard.validateMessage(es, &msg, 3)
	s.Require().NoError(err)
	s.False(ok)
	s.Require().Len(es.Receipts, 3)
	s.False(es.Receipts[2].Success)
}

func TestSuiteShardchainState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteShardchainState))
}
