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
	gasPrice := types.NewValueFromUint64(10)
	from := types.MainWalletAddress
	to := contracts.CounterAddress(s.T(), shardId)

	pool := &MockMsgPool{}
	c := newCollator(Params{
		BlockGeneratorParams: execution.NewBlockGeneratorParams(shardId, nShards, gasPrice, 0),
	}, new(TrivialShardTopology), pool, sharedLogger)

	generateBlock := func() {
		proposal, err := c.GenerateProposal(s.ctx, s.db)
		s.Require().NoError(err)
		s.Require().NotNil(proposal)

		blockGenerator, err := execution.NewBlockGenerator(s.ctx, execution.NewBlockGeneratorParams(shardId, nShards, gasPrice, 0), s.db)
		s.Require().NoError(err)
		defer blockGenerator.Rollback()

		block, err := blockGenerator.GenerateBlock(proposal, gasPrice)
		s.Require().NoError(err)
		s.Require().NotNil(block)
	}

	s.Run("GenerateZeroState", func() {
		execution.GenerateZeroState(s.T(), s.ctx, shardId, s.db)
	})

	// This values depends on the current implementation (precompiled contract, opcode gas prices).
	actualMsgGas := types.Gas(12_959)

	// These parameters can be adjusted for test purposes. The rest is calculated.
	gasLimit := types.Gas(100_000)
	sentValue := types.NewValueFromUint64(2_000_000)

	balance := s.getBalance(shardId, from)
	reserveForGas := gasLimit.ToValue(gasPrice)
	actualMsgPrice := actualMsgGas.ToValue(gasPrice)

	s.Run("SendTokens", func() {
		msgValue := sentValue.Add(reserveForGas)
		m1 := execution.NewExecutionMessage(from, from, 0, contracts.NewWalletSendCallData(s.T(), types.Code{},
			gasLimit, msgValue, []types.CurrencyBalance{}, to, types.ExecutionMessageKind))
		m2 := common.CopyPtr(m1)
		m2.Seqno = 1
		s.Require().NoError(m1.Sign(execution.MainPrivateKey))
		s.Require().NoError(m2.Sign(execution.MainPrivateKey))
		pool.Msgs = []*types.Message{m1, m2}

		generateBlock()
		s.checkReceipt(shardId, m1)
		s.checkReceipt(shardId, m2)

		// Each message subtracts its value + actual gas used from the balance.
		balance = balance.
			Sub(msgValue).Sub(actualMsgPrice).
			Sub(msgValue).Sub(actualMsgPrice)
		s.Equal(balance, s.getBalance(shardId, from))
		s.Equal(types.Value{}, s.getBalance(shardId, to))
	})

	// Now process messages by one to test queueing.
	c.params.MaxInMessagesInBlock = 1
	// Messages from the pool must not be processed, while we have internal messages.
	// So add a faulty message.
	pool.Msgs = []*types.Message{nil}

	s.Run("ProcessInternalMessage1", func() {
		generateBlock()

		s.Equal(balance, s.getBalance(shardId, from))
		s.Equal(sentValue, s.getBalance(shardId, to))
	})

	s.Run("ProcessInternalMessage2", func() {
		generateBlock()

		s.Equal(balance, s.getBalance(shardId, from))
		s.Equal(sentValue.Add(sentValue), s.getBalance(shardId, to))
	})

	c.params.MaxInMessagesInBlock = 2

	s.Run("ProcessRefundMessages", func() {
		generateBlock()

		balance = balance.Add(reserveForGas).Add(reserveForGas)
		s.Equal(balance, s.getBalance(shardId, from))
	})

	s.Run("Deploy", func() {
		m := execution.NewDeployMessage(contracts.CounterDeployPayload(s.T()), shardId, to, 0)
		m.Flags.ClearBit(types.MessageFlagInternal)
		s.Equal(to, m.To)
		pool.Msgs = []*types.Message{m}

		generateBlock()
		pool.Msgs = nil
		s.checkReceipt(shardId, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionMessage(to, to, 0, contracts.NewCounterAddCallData(s.T(), 3))
		pool.Msgs = []*types.Message{m}

		generateBlock()
		pool.Msgs = nil
		s.checkReceipt(shardId, m)
	})
}

func (s *CollatorTestSuite) getBalance(shardId types.ShardId, addr types.Address) types.Value {
	s.T().Helper()

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	state, err := execution.NewExecutionStateForShard(tx, shardId, common.NewTimer())
	s.Require().NoError(err)
	acc, err := state.GetAccount(addr)
	s.Require().NoError(err)
	if acc == nil {
		return types.Value{}
	}
	return acc.Balance
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
