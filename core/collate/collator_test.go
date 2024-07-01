package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type CollatorTestSuite struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (s *CollatorTestSuite) SetupSuite() {
	s.ctx = context.Background()
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
	shardId := types.BaseShardId
	nShards := 2
	from := types.MainWalletAddress
	to := contracts.CounterAddress(s.T(), shardId)

	pool := &MockMsgPool{}
	c := newCollator(Params{
		BlockGeneratorParams: execution.NewBlockGeneratorParams(shardId, nShards, uint256.NewInt(10), 0),
	}, new(TrivialShardTopology), pool, sharedLogger)

	s.Run("GenerateZeroState", func() {
		s.Require().NoError(c.GenerateZeroState(s.ctx, s.db, execution.DefaultZeroStateConfig))
	})

	balance := s.getBalance(shardId, from)
	gasLimit := types.NewUint256(100_000)
	sentValue := types.NewUint256(2_000_000)
	reserveForGas := uint256.NewInt(0).Mul(&gasLimit.Int, uint256.NewInt(10))
	msgFinalPrice := uint256.NewInt(2_129_650)

	s.Run("SendTokens", func() {
		var msgValue types.Uint256
		msgValue.Add(&sentValue.Int, reserveForGas)
		m1 := execution.NewExecutionMessage(from, from, 0, contracts.NewWalletSendCallData(s.T(), types.Code{},
			gasLimit, &msgValue, []types.CurrencyBalance{}, to, types.ExecutionMessageKind))
		m2 := common.CopyPtr(m1)
		m2.Seqno = 1
		s.Require().NoError(m1.Sign(execution.MainPrivateKey))
		s.Require().NoError(m2.Sign(execution.MainPrivateKey))
		pool.Msgs = []*types.Message{m1, m2}

		s.Require().NoError(c.GenerateBlock(s.ctx, s.db))
		s.checkReceipt(shardId, m1)
		s.checkReceipt(shardId, m2)

		balance.Sub(balance, msgFinalPrice)
		balance.Sub(balance, reserveForGas)
		balance.Sub(balance, msgFinalPrice)
		balance.Sub(balance, reserveForGas)
		s.Equal(balance, s.getBalance(shardId, from))
		s.Equal(uint256.NewInt(0), s.getBalance(shardId, to))
	})

	// Now process messages by one to test queueing.
	c.params.MaxInMessagesInBlock = 1
	// Messages from the pool must not be processed, while we have internal messages.
	// So add a faulty message.
	pool.Msgs = []*types.Message{nil}

	s.Run("ProcessFirstInternalMessage", func() {
		s.Require().NoError(c.GenerateBlock(s.ctx, s.db))

		s.Equal(balance, s.getBalance(shardId, from))
		s.Equal(&sentValue.Int, s.getBalance(shardId, to))
	})

	s.Run("ProcessSecondInternalMessage", func() {
		s.Require().NoError(c.GenerateBlock(s.ctx, s.db))

		s.Equal(balance, s.getBalance(shardId, from))
		s.Equal(uint256.NewInt(0).Add(&sentValue.Int, &sentValue.Int), s.getBalance(shardId, to))
	})

	c.params.MaxInMessagesInBlock = 2

	s.Run("ProcessRefundMessages", func() {
		s.Require().NoError(c.GenerateBlock(s.ctx, s.db))

		balance.Add(balance, reserveForGas)
		balance.Add(balance, reserveForGas)
		s.Equal(balance, s.getBalance(shardId, from))
	})

	s.Run("Deploy", func() {
		m := execution.NewDeployMessage(contracts.CounterDeployPayload(s.T()), shardId, to, 0)
		m.Flags.ClearBit(types.MessageFlagInternal)
		s.Equal(to, m.To)
		pool.Msgs = []*types.Message{m}

		s.Require().NoError(c.GenerateBlock(s.ctx, s.db))
		pool.Msgs = nil
		s.checkReceipt(shardId, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionMessage(to, to, 0, contracts.NewCounterAddCallData(s.T(), 3))
		pool.Msgs = []*types.Message{m}

		s.Require().NoError(c.GenerateBlock(s.ctx, s.db))
		pool.Msgs = nil
		s.checkReceipt(shardId, m)
	})
}

func (s *CollatorTestSuite) getBalance(shardId types.ShardId, addr types.Address) *uint256.Int {
	s.T().Helper()

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	state, err := execution.NewExecutionStateForShard(tx, shardId, common.NewTimer())
	s.Require().NoError(err)
	acc, err := state.GetAccount(addr)
	s.Require().NoError(err)
	if acc == nil {
		return uint256.NewInt(0)
	}
	return &acc.Balance
}

func (s *CollatorTestSuite) checkReceipt(shardId types.ShardId, m *types.Message) {
	s.T().Helper()

	tx, err := s.db.CreateRoTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	sa, err := execution.NewStateAccessor()
	s.Require().NoError(err)

	msgData, err := sa.Access(tx, m.From.ShardId()).GetInMessage().ByHash(m.Hash())
	s.Require().NoError(err)

	receiptsTrie := execution.NewReceiptTrieReader(mpt.NewReaderWithRoot(tx, shardId, db.ReceiptTrieTable, msgData.Block().ReceiptsRoot))
	receipt, err := receiptsTrie.Fetch(msgData.Index())
	s.Require().NoError(err)
	s.Equal(m.Hash(), receipt.MsgHash)
}

func TestCollator(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CollatorTestSuite{})
}
