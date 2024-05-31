package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type MockMsgPool struct {
	Msgs []*types.Message
}

var _ MsgPool = (*MockMsgPool)(nil)

func (m *MockMsgPool) Peek(context.Context, int, uint64) ([]*types.Message, error) {
	return m.Msgs, nil
}

func (m *MockMsgPool) OnNewBlock(context.Context, *types.Block, []*types.Message, db.Tx) error {
	return nil
}

type CollatorTestSuite struct {
	suite.Suite
	db db.DB
}

func (s *CollatorTestSuite) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *CollatorTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *CollatorTestSuite) TestCollator() {
	ctx := context.Background()
	shardId := types.ShardId(1)
	shard := shardchain.NewShardChain(shardId, s.db)

	m := &types.Message{
		From: types.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20"),
		Data: hexutil.FromHex("6009600c60003960096000f3600054600101600055"),
	}
	pool := &MockMsgPool{Msgs: []*types.Message{m}}

	c := newCollator(shard, pool, shardId, 2, common.NewLogger("collator"), new(TrivialShardTopology))

	s.Run("zero-state", func() {
		err := c.GenerateBlock(ctx)
		s.Require().NoError(err)
	})

	s.Run("deploy-message", func() {
		err := c.GenerateBlock(ctx)
		s.Require().NoError(err)

		s.checkReceipt(ctx, shardId, m)
	})

	s.Run("call-message", func() {
		m.To = types.CreateAddress(shardId, m.From, m.Seqno)

		err := c.GenerateBlock(ctx)
		s.Require().NoError(err)

		s.checkReceipt(ctx, shardId, m)
	})
}

func (s *CollatorTestSuite) checkReceipt(ctx context.Context, shardId types.ShardId, m *types.Message) {
	s.T().Helper()

	tx, err := s.db.CreateRoTx(ctx)
	s.Require().NoError(err)

	es, err := execution.NewExecutionStateForShard(tx, shardId, common.NewTestTimer(0))
	s.Require().NoError(err)

	r, err := es.GetReceipt(0)
	s.Require().NoError(err)

	s.Equal(uint64(0), r.MsgIndex)
	s.Equal(m.Hash(), r.MsgHash)
}

func TestCollator(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CollatorTestSuite{})
}
