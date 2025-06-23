package collate

import (
	"slices"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
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
		BlockGeneratorParams: execution.NewTestBlockGeneratorParams(s.shardId, 2),
	}
}

func newTestProposer(params *Params, pool TxnPool) *proposer {
	return newProposer(params, new(TrivialShardTopology), pool, logging.NewLogger("proposer"))
}

func (s *ProposerTestSuite) generateProposal(p *proposer) *execution.Proposal {
	s.T().Helper()

	proposalSerializable, err := p.GenerateProposal(s.T().Context(), s.db)
	s.Require().NoError(err)
	s.Require().NotNil(proposalSerializable)

	proposal, err := execution.ConvertProposal(proposalSerializable)
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
		params.MaxGasInBlock = 2000
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

	generateBlock := func() (*execution.Proposal, *execution.BlockGenerationResult) {
		proposal := s.generateProposal(p)

		tx, err := s.db.CreateRoTx(s.T().Context())
		s.Require().NoError(err)
		defer tx.Rollback()

		prevBlock, err := db.ReadBlock(tx, shardId, proposal.PrevBlockHash)
		s.Require().NoError(err)

		gen, err := execution.NewBlockGenerator(s.T().Context(), params.BlockGeneratorParams, s.db, prevBlock)
		s.Require().NoError(err)
		defer gen.Rollback()

		block, err := gen.GenerateBlock(proposal, &types.ConsensusParams{})
		s.Require().NoError(err)

		return proposal, block
	}

	s.Run("GenerateZeroState", func() {
		execution.GenerateZeroState(s.T(), types.MainShardId, s.db)
		execution.GenerateZeroState(s.T(), shardId, s.db)
	})

	balance := s.getMainBalance()
	txnValue := execution.DefaultSendValue

	m1 := execution.NewSendMoneyTransaction(s.T(), to, 0)
	m2 := execution.NewSendMoneyTransaction(s.T(), to, 1)
	var r1, r2 *types.Receipt

	s.Run("SendTokens", func() {
		pool.Reset()
		pool.Add(m1, m2)

		proposal, res := generateBlock()
		r1 = s.checkReceipt(res, m1)
		r2 = s.checkReceipt(res, m2)
		s.Equal(pool.Txns, proposal.ExternalTxns)

		// Each transaction subtracts its value + actual gas used from the balance.
		balance = balance.
			Sub(txnValue).Sub(r1.GasUsed.ToValue(types.DefaultGasPrice)).Sub(r1.Forwarded).
			Sub(txnValue).Sub(r2.GasUsed.ToValue(types.DefaultGasPrice)).Sub(r2.Forwarded)
		s.Equal(balance, s.getMainBalance())
		s.Equal(types.Value{}, s.getBalance(shardId, to))
	})

	pool.Reset()

	s.Run("ProcessInternalTransaction1", func() {
		generateBlock()

		s.Equal(balance, s.getMainBalance())
		s.Equal(txnValue.Mul(types.NewValueFromUint64(2)), s.getBalance(shardId, to))
	})

	s.Run("ProcessRefundTransactions", func() {
		generateBlock()

		balance = balance.Add(r1.Forwarded).Add(r2.Forwarded)
		s.Equal(balance, s.getMainBalance())
	})

	s.Run("DoNotProcessDuplicates", func() {
		pool.Reset()
		pool.Add(m1, m2)

		proposal, _ := generateBlock()
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

		_, res := generateBlock()
		s.checkReceipt(res, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionTransaction(to, to, 1, contracts.NewCounterAddCallData(s.T(), 3))
		pool.Reset()
		pool.Add(m)

		_, res := generateBlock()
		s.checkReceipt(res, m)
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

	state := execution.NewTestExecutionState(s.T(), tx, shardId, execution.StateParams{
		Block: block,
	})
	acc, err := state.GetAccount(addr)
	s.Require().NoError(err)
	if acc == nil {
		return types.Value{}
	}
	return acc.GetBalance()
}

func (s *ProposerTestSuite) checkReceipt(genRes *execution.BlockGenerationResult, m *types.Transaction) *types.Receipt {
	s.T().Helper()

	hash := m.Hash()
	idx := slices.IndexFunc(genRes.Receipts, func(r *types.Receipt) bool {
		return r.TxnHash == hash
	})
	s.Require().GreaterOrEqual(idx, 0, "receipt not found for transaction %s", hash)

	return genRes.Receipts[idx]
}

func TestProposer(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ProposerTestSuite{})
}
