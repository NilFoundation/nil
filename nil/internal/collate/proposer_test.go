package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type ProposerTestSuite struct {
	suite.Suite

	ctx     context.Context
	shardId types.ShardId
	db      db.DB
}

func (s *ProposerTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.shardId = types.BaseShardId
}

func (s *ProposerTestSuite) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *ProposerTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *ProposerTestSuite) newParams() Params {
	return Params{
		BlockGeneratorParams: execution.NewBlockGeneratorParams(s.shardId, 2, types.DefaultGasPrice, 0),
	}
}

func newTestProposer(params Params, pool TxnPool) *proposer {
	return newProposer(params, new(TrivialShardTopology), pool, logging.NewLogger("proposer"))
}

func (s *ProposerTestSuite) TestBlockGas() {
	s.Run("GenerateZeroState", func() {
		execution.GenerateZeroState(s.T(), s.ctx, s.shardId, s.db)
	})

	to := contracts.CounterAddress(s.T(), s.shardId)
	m1 := execution.NewSendMoneyTransaction(s.T(), to, 0)
	m2 := execution.NewSendMoneyTransaction(s.T(), to, 1)
	pool := &MockTxnPool{Txns: []*types.Transaction{m1, m2}}

	params := s.newParams()

	s.Run("DefaultMaxGasInBlock", func() {
		p := newTestProposer(params, pool)

		proposal, err := p.GenerateProposal(s.ctx, s.db)
		s.Require().NoError(err)
		s.Require().NotNil(proposal)

		s.Require().Len(proposal.InTxns, 2)
	})

	s.Run("MaxGasInBlockFor1Txn", func() {
		params.MaxGasInBlock = 5000
		p := newTestProposer(params, pool)

		proposal, err := p.GenerateProposal(s.ctx, s.db)
		s.Require().NoError(err)
		s.Require().NotNil(proposal)

		s.Require().Len(proposal.InTxns, 1)
	})
}

func (s *ProposerTestSuite) TestCollator() {
	to := contracts.CounterAddress(s.T(), s.shardId)

	pool := &MockTxnPool{}
	params := s.newParams()
	p := newTestProposer(params, pool)
	gasPrice := p.params.GasBasePrice
	shardId := p.params.ShardId

	generateBlock := func() *execution.Proposal {
		proposal, err := p.GenerateProposal(s.ctx, s.db)
		s.Require().NoError(err)
		s.Require().NotNil(proposal)

		blockGenerator, err := execution.NewBlockGenerator(s.ctx, params.BlockGeneratorParams, s.db)
		s.Require().NoError(err)
		defer blockGenerator.Rollback()

		_, err = blockGenerator.GenerateBlock(proposal, zerolog.Nop(), nil)
		s.Require().NoError(err)

		return proposal
	}

	s.Run("GenerateZeroState", func() {
		execution.GenerateZeroState(s.T(), s.ctx, shardId, s.db)
	})

	balance := s.getMainBalance()
	txnValue := execution.DefaultSendValue
	feeCredit := execution.DefaultGasCredit

	m1 := execution.NewSendMoneyTransaction(s.T(), to, 0)
	m2 := execution.NewSendMoneyTransaction(s.T(), to, 1)

	s.Run("SendTokens", func() {
		pool.Txns = []*types.Transaction{m1, m2}

		proposal := generateBlock()
		r1 := s.checkReceipt(shardId, m1)
		r2 := s.checkReceipt(shardId, m2)
		s.Equal(pool.Txns, proposal.InTxns)
		s.Equal(pool.Txns, proposal.RemoveFromPool)

		pool.Txns = nil

		// Each transaction subtracts its value + actual gas used from the balance.
		balance = balance.
			Sub(txnValue).Sub(r1.GasUsed.ToValue(gasPrice)).Sub(feeCredit).
			Sub(txnValue).Sub(r2.GasUsed.ToValue(gasPrice)).Sub(feeCredit)
		s.Equal(balance, s.getMainBalance())
		s.Equal(types.Value{}, s.getBalance(shardId, to))
	})

	// Now process internal transactions by one to test queueing.
	p.params.MaxInternalTransactionsInBlock = 1

	s.Run("ProcessInternalTransaction1", func() {
		generateBlock()

		s.Equal(balance, s.getMainBalance())
		s.Equal(txnValue, s.getBalance(shardId, to))
	})

	s.Run("ProcessInternalTransaction2", func() {
		generateBlock()

		s.Equal(balance, s.getMainBalance())
		s.Equal(txnValue.Add(txnValue), s.getBalance(shardId, to))
	})

	p.params.MaxInternalTransactionsInBlock = defaultMaxInternalTxns

	s.Run("ProcessRefundTransactions", func() {
		generateBlock()

		balance = balance.Add(feeCredit).Add(feeCredit)
		s.Equal(balance, s.getMainBalance())

		// TODO: Enable when fixed uninitialized refunds
		// s.checkSeqno(shardId)
	})

	s.Run("DoNotProcessDuplicates", func() {
		pool.Txns = []*types.Transaction{m1, m2}

		proposal := generateBlock()
		s.Len(proposal.InTxns, 2)
		s.Empty(proposal.ForwardTxns)
		s.Equal(pool.Txns, proposal.RemoveFromPool)

		pool.Txns = nil
	})

	s.Run("Deploy", func() {
		m := execution.NewDeployTransaction(contracts.CounterDeployPayload(s.T()), shardId, to, 0, types.Value{})
		m.Flags.ClearBit(types.TransactionFlagInternal)
		s.Equal(to, m.To)
		pool.Txns = []*types.Transaction{m}

		generateBlock()
		pool.Txns = nil
		s.checkReceipt(shardId, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionTransaction(to, to, 0, contracts.NewCounterAddCallData(s.T(), 3))
		pool.Txns = []*types.Transaction{m}

		generateBlock()
		pool.Txns = nil
		s.checkReceipt(shardId, m)
	})

	s.Run("CheckRefundsSeqno", func() {
		m01 := execution.NewSendMoneyTransaction(s.T(), to, 2)
		m02 := execution.NewSendMoneyTransaction(s.T(), to, 3)
		pool.Txns = []*types.Transaction{m01, m02}

		// send tokens
		generateBlock()

		// process internal transactions
		generateBlock()

		// process refunds
		generateBlock()

		// check refunds seqnos
		s.checkSeqno(shardId)
	})
}

func (s *ProposerTestSuite) getMainBalance() types.Value {
	s.T().Helper()

	return s.getBalance(s.shardId, types.MainSmartAccountAddress)
}

func (s *ProposerTestSuite) getBalance(shardId types.ShardId, addr types.Address) types.Value {
	s.T().Helper()

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	state, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		GetBlockFromDb: true,
		ConfigAccessor: config.GetStubAccessor(),
	})
	s.Require().NoError(err)
	acc, err := state.GetAccount(addr)
	s.Require().NoError(err)
	if acc == nil {
		return types.Value{}
	}
	return acc.Balance
}

func (s *ProposerTestSuite) checkSeqno(shardId types.ShardId) {
	s.T().Helper()

	tx, err := s.db.CreateRoTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	sa := execution.NewStateAccessor()
	blockHash, err := db.ReadLastBlockHash(tx, shardId)
	s.Require().NoError(err)

	block, err := sa.Access(tx, shardId).GetBlock().WithInTransactions().WithOutTransactions().ByHash(blockHash)
	s.Require().NoError(err)

	check := func(txns []*types.Transaction) {
		if len(txns) == 0 {
			return
		}
		seqno := txns[0].Seqno
		for _, m := range txns {
			s.Require().Equal(seqno, m.Seqno)
			seqno += 1
		}
	}

	check(block.InTransactions())
	check(block.OutTransactions())
}

func (s *ProposerTestSuite) checkReceipt(shardId types.ShardId, m *types.Transaction) *types.Receipt {
	s.T().Helper()

	tx, err := s.db.CreateRoTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	sa := execution.NewStateAccessor()
	txnData, err := sa.Access(tx, m.From.ShardId()).GetInTransaction().ByHash(m.Hash())
	s.Require().NoError(err)

	receiptsTrie := execution.NewDbReceiptTrieReader(tx, shardId)
	receiptsTrie.SetRootHash(txnData.Block().ReceiptsRoot)
	receipt, err := receiptsTrie.Fetch(txnData.Index())
	s.Require().NoError(err)
	s.Equal(m.Hash(), receipt.TxnHash)
	return receipt
}

func TestCollator(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ProposerTestSuite{})
}
