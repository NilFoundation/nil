package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
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

func (m *MockMsgPool) OnNewBlock(context.Context, *types.Block, []*types.Message) error {
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

	code, err := contracts.GetCode("tests/Counter")
	s.Require().NoError(err)

	m := &types.Message{
		From:     types.CreateAddress(shardId, []byte("1234")),
		Data:     code,
		Internal: true,
	}
	pool := &MockMsgPool{Msgs: []*types.Message{m}}

	c := newCollator(shardId, 2, new(TrivialShardTopology), false, pool, logging.NewLogger("collator"))

	s.Run("zero-state", func() {
		s.Require().NoError(c.GenerateZeroState(ctx, s.db, ""))
	})

	s.Run("deploy-message", func() {
		s.Require().NoError(c.GenerateBlock(ctx, s.db))

		s.checkReceipt(ctx, shardId, m)
	})

	s.Run("call-message", func() {
		m.To = types.CreateAddress(shardId, []byte("call-message"))
		pool.Msgs = append(pool.Msgs, m)

		s.Require().NoError(c.GenerateBlock(ctx, s.db))

		s.checkReceipt(ctx, shardId, m)
	})
}

func (s *CollatorTestSuite) checkReceipt(ctx context.Context, shardId types.ShardId, m *types.Message) {
	s.T().Helper()

	tx, err := s.db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	sa, err := execution.NewStateAccessor()
	s.Require().NoError(err)

	msgData, err := sa.Access(tx, m.From.ShardId()).GetInMessage().ByHash(m.Hash())
	s.Require().NoError(err)

	receiptsTrie := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.ReceiptTrieTable, msgData.Block().ReceiptsRoot)
	data, err := receiptsTrie.Get(msgData.Index().Bytes())
	s.Require().NoError(err)

	var receipt types.Receipt
	s.Require().NoError(receipt.UnmarshalSSZ(data))
	s.Equal(m.Hash(), receipt.MsgHash)
}

func TestCollator(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CollatorTestSuite{})
}
