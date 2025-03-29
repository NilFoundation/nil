package collate

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	"github.com/stretchr/testify/suite"
)

type ProposerTestSuite struct {
	suite.Suite

	shardId types.ShardId
	db      db.DB
}

func (s *ProposerTestSuite) SetupSuite() {
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

func (s *ProposerTestSuite) newParams() *Params {
	return &Params{
		BlockGeneratorParams: execution.NewBlockGeneratorParams(s.shardId, 2),
	}
}

func newTestProposer(params *Params, pool TxnPool) *proposer {
	return newProposer(params, new(TrivialShardTopology), pool, logging.NewLogger("proposer"))
}

func (s *ProposerTestSuite) generateProposal(p *proposer) *execution.Proposal {
	s.T().Helper()

	proposalSSZ, err := p.GenerateProposal(s.T().Context(), s.db)
	s.Require().NoError(err)
	s.Require().NotNil(proposalSSZ)

	proposal, err := execution.ConvertProposal(proposalSSZ)
	s.Require().NoError(err)

	return proposal
}

func (s *ProposerTestSuite) TestBlockGas() {
	s.Run("GenerateZeroState", func() {
		execution.GenerateZeroState(s.T(), types.MainShardId, s.db)
		execution.GenerateZeroState(s.T(), s.shardId, s.db)
	})

	to := contracts.CounterAddress(s.T(), s.shardId)
	m1 := execution.NewSendMoneyTransaction(s.T(), to, 0)
	m2 := execution.NewSendMoneyTransaction(s.T(), to, 1)
	pool := &MockTxnPool{}
	pool.Add(m1, m2)

	params := s.newParams()

	s.Run("DefaultMaxGasInBlock", func() {
		p := newTestProposer(params, pool)

		proposal := s.generateProposal(p)
		s.Equal(pool.Txns, proposal.ExternalTxns)
	})

	s.Run("MaxGasInBlockFor1Txn", func() {
		params.MaxGasInBlock = 12000
		p := newTestProposer(params, pool)

		proposal := s.generateProposal(p)

		s.Equal(pool.Txns[:1], proposal.ExternalTxns)
	})
}

func (s *ProposerTestSuite) TestCollator() {
	to := contracts.CounterAddress(s.T(), s.shardId)

	pool := &MockTxnPool{}
	params := s.newParams()
	p := newTestProposer(params, pool)
	shardId := p.params.ShardId

	generateBlock := func() *execution.Proposal {
		proposal := s.generateProposal(p)

		tx, err := s.db.CreateRoTx(s.T().Context())
		s.Require().NoError(err)
		defer tx.Rollback()

		block, err := db.ReadBlock(tx, shardId, proposal.PrevBlockHash)
		s.Require().NoError(err)

		blockGenerator, err := execution.NewBlockGenerator(s.T().Context(), params.BlockGeneratorParams, s.db, block)
		s.Require().NoError(err)
		defer blockGenerator.Rollback()

		_, err = blockGenerator.GenerateBlock(proposal, &types.ConsensusParams{})
		s.Require().NoError(err)

		return proposal
	}

	s.Run("GenerateZeroState", func() {
		execution.GenerateZeroState(s.T(), types.MainShardId, s.db)
		execution.GenerateZeroState(s.T(), shardId, s.db)
	})

	balance := s.getMainBalance()
	txnValue := execution.DefaultSendValue
	feeCredit := execution.DefaultGasCredit

	m1 := execution.NewSendMoneyTransaction(s.T(), to, 0)
	m2 := execution.NewSendMoneyTransaction(s.T(), to, 1)

	s.Run("SendTokens", func() {
		pool.Reset()
		pool.Add(m1, m2)

		proposal := generateBlock()
		r1 := s.checkReceipt(shardId, m1)
		r2 := s.checkReceipt(shardId, m2)
		s.Equal(pool.Txns, proposal.ExternalTxns)

		// Each transaction subtracts its value + actual gas used from the balance.
		balance = balance.
			Sub(txnValue).Sub(r1.GasUsed.ToValue(types.DefaultGasPrice)).Sub(feeCredit).
			Sub(txnValue).Sub(r2.GasUsed.ToValue(types.DefaultGasPrice)).Sub(feeCredit)
		s.Equal(balance, s.getMainBalance())
		s.Equal(types.Value{}, s.getBalance(shardId, to))
	})

	// Now process internal transactions by one to test queueing.
	p.params.MaxInternalTransactionsInBlock = 1
	pool.Reset()

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

		s.checkSeqno(shardId)
	})

	s.Run("DoNotProcessDuplicates", func() {
		pool.Reset()
		pool.Add(m1, m2)

		proposal := generateBlock()
		s.Empty(proposal.ExternalTxns)
		s.Empty(proposal.InternalTxns)
		s.Empty(proposal.ForwardTxns)
		s.Equal([]common.Hash{m1.Hash(), m2.Hash()}, pool.LastDiscarded)
		s.Equal(txnpool.Unverified, pool.LastReason)
	})

	s.Run("Deploy", func() {
		m := execution.NewDeployTransaction(contracts.CounterDeployPayload(s.T()), shardId, to, 0, types.Value{})
		m.Flags.ClearBit(types.TransactionFlagInternal)
		s.Equal(to, m.To)
		pool.Reset()
		pool.Add(m)

		generateBlock()
		s.checkReceipt(shardId, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionTransaction(to, to, 1, contracts.NewCounterAddCallData(s.T(), 3))
		pool.Reset()
		pool.Add(m)

		generateBlock()
		s.checkReceipt(shardId, m)
	})

	s.Run("CheckRefundsSeqno", func() {
		m01 := execution.NewSendMoneyTransaction(s.T(), to, 2)
		m02 := execution.NewSendMoneyTransaction(s.T(), to, 3)
		pool.Reset()
		pool.Add(m01, m02)

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

	tx, err := s.db.CreateRoTx(s.T().Context())
	s.Require().NoError(err)
	defer tx.Rollback()

	block, _, err := db.ReadLastBlock(tx, shardId)
	s.Require().NoError(err)

	state, err := execution.NewExecutionState(tx, shardId, execution.StateParams{
		Block:          block,
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

	tx, err := s.db.CreateRoTx(s.T().Context())
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

	tx, err := s.db.CreateRoTx(s.T().Context())
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

func TestProposer(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ProposerTestSuite{})
}
