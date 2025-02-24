package collate

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
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

func (s *ProposerTestSuite) newParams() Params {
	return Params{
		BlockGeneratorParams: execution.NewBlockGeneratorParams(s.shardId, 2),
	}
}

func newTestProposer(params Params, pool TxnPool) *proposer {
	return newProposer(params, new(TrivialShardTopology), pool, logging.NewLogger("proposer"))
}

func (s *ProposerTestSuite) generateProposal(p *proposer) *execution.Proposal {
	s.T().Helper()

	proposal, err := p.GenerateProposal(s.T().Context(), s.db)
	s.Require().NoError(err)
	s.Require().NotNil(proposal)

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
	pool := &MockTxnPool{Txns: []*types.Transaction{m1, m2}}

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

func (s *ProposerTestSuite) generateBlock(p *proposer) *execution.Proposal {
	s.T().Helper()

	proposal := s.generateProposal(p)

	tx, err := s.db.CreateRoTx(s.T().Context())
	s.Require().NoError(err)
	defer tx.Rollback()

	block, err := db.ReadBlock(tx, s.shardId, proposal.PrevBlockHash)
	s.Require().NoError(err)

	params := s.newParams()

	blockGenerator, err := execution.NewBlockGenerator(s.T().Context(), params.BlockGeneratorParams, s.db, block)
	s.Require().NoError(err)
	defer blockGenerator.Rollback()

	_, err = blockGenerator.GenerateBlock(proposal, &types.ConsensusParams{})
	s.Require().NoError(err)

	return proposal
}

func (s *ProposerTestSuite) TestTransactionsOrdering() {
	pool := &MockTxnPool{}
	params := s.newParams()
	p := newTestProposer(params, pool)
	shardId := p.params.ShardId

	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)
	calldata, err := abi.Pack("noReturn")
	s.Require().NoError(err)

	const accountsNum = 5

	accounts := make([]types.Address, 0, accountsNum)
	seqnos := make(map[types.Address]types.Seqno, accountsNum)

	updateSeqnos := func() {
		for addr := range seqnos {
			seqnos[addr] = s.getSeqno(addr)
		}
	}

	for i := range accountsNum {
		addr, err := contracts.CalculateAddress(contracts.NameTest, shardId, []byte{byte(i)})
		s.Require().NoError(err)
		accounts = append(accounts, addr)
		seqnos[addr] = 0
	}

	s.Run("GenerateZeroState", func() {
		zerostate := &execution.ZeroStateConfig{}
		params.ShardId = types.MainShardId
		gen, err := execution.NewBlockGenerator(s.T().Context(), params.BlockGeneratorParams, s.db, nil)
		s.Require().NoError(err)
		_, err = gen.GenerateZeroState(zerostate)
		s.Require().NoError(err)

		for i := range accounts {
			zerostate.Contracts = append(zerostate.Contracts, &execution.ContractDescr{
				Contract: contracts.NameTest,
				Shard:    types.MainShardId,
				Address:  &accounts[i],
				Value:    types.NewValueFromUint64(1_000_000_000_000_000),
			})
		}
		params.ShardId = types.BaseShardId
		gen, err = execution.NewBlockGenerator(s.T().Context(), params.BlockGeneratorParams, s.db, nil)
		s.Require().NoError(err)
		_, err = gen.GenerateZeroState(zerostate)
		s.Require().NoError(err)
	})

	feeCalc := &execution.ConstFeeCalculator{Value: types.NewValueFromUint64(100)}
	params.FeeCalculator = feeCalc

	var txGas types.Gas
	s.Run("Calculate transaction gas cost", func() {
		roTx, err := s.db.CreateRoTx(s.T().Context())
		s.Require().NoError(err)
		defer roTx.Rollback()

		block, _, err := db.ReadLastBlock(roTx, shardId)
		s.Require().NoError(err)

		es, err := execution.NewExecutionState(roTx, s.shardId, execution.StateParams{
			Block:          block,
			ConfigAccessor: config.GetStubAccessor(),
			FeeCalculator:  feeCalc,
		})
		s.Require().NoError(err)

		txn := execution.NewExecutionTransaction(accounts[0], accounts[0], 0, calldata)
		verifyRes := es.CallVerifyExternal(txn)
		s.Require().False(verifyRes.Failed())
		res := es.HandleTransaction(s.T().Context(), txn, execution.NewDummyPayer())
		s.Require().False(res.Failed())
		txGas = res.GasUsed + verifyRes.GasUsed
	})

	p.params.MaxGasInBlock = txGas * 10000
	p.params.FeeCalculator = feeCalc
	createTxn := func(accountIdx, priorityFee, maxFee, value uint64) {
		addr := accounts[accountIdx]
		txn := execution.NewExecutionTransaction(addr, addr, seqnos[addr], calldata)
		seqnos[addr]++
		txn.MaxPriorityFeePerGas = types.NewValueFromUint64(priorityFee)
		txn.MaxFeePerGas = types.NewValueFromUint64(maxFee)
		txn.Value = types.NewValueFromUint64(value)
		pool.Txns = append(pool.Txns, txn)
	}

	checkOrder := func(proposal *execution.Proposal, vals ...int) {
		s.T().Helper()

		s.Require().Equal(len(vals), len(proposal.ExternalTxns))

		for i, txn := range proposal.ExternalTxns {
			s.Require().Equal(vals[i], int(txn.Value.Uint64()))
		}
	}

	updateSeqnos()
	// accIdx, priorityFee, maxFee, value, effective priorityFee
	createTxn(0, 5, 110, 0)  // 5
	createTxn(1, 5, 99, 1)   // -1
	createTxn(2, 50, 102, 2) // 2
	createTxn(3, 10, 120, 3) // 10
	createTxn(4, 15, 111, 4) // 11

	s.Run("Check order 1", func() {
		proposal := s.generateBlock(p)
		checkOrder(proposal, 4, 3, 0, 2)
		pool.Txns = nil
	})

	updateSeqnos()
	// accIdx, priorityFee, maxFee, value, effective priorityFee
	createTxn(0, 50, 102, 0) // 2
	createTxn(0, 5, 110, 2)  // 5

	createTxn(1, 8, 110, 3)  // 8
	createTxn(1, 20, 112, 4) // 12

	createTxn(2, 10, 100, 5) // 0

	createTxn(3, 20, 150, 7) // 20

	createTxn(4, 20, 116, 8)  // 16
	createTxn(4, 6, 110, 9)   // 6
	createTxn(4, 50, 130, 10) // 30

	s.Run("Check order 2", func() {
		proposal := s.generateBlock(p)
		checkOrder(proposal, 7, 8, 3, 4, 9, 10, 0, 2, 5)
		pool.Txns = nil
	})

	updateSeqnos()
	// accIdx, priorityFee, maxFee, value, effective priorityFee
	createTxn(0, 50, 98, 0) // -2
	createTxn(0, 50, 95, 2) // -5

	createTxn(1, 8, 90, 3) // -10

	createTxn(2, 30, 99, 5) // -1

	s.Run("No transactions with positive Effective fee", func() {
		proposal := s.generateBlock(p)
		checkOrder(proposal)
		pool.Txns = nil
	})

	updateSeqnos()
	// accIdx, priorityFee, maxFee, value, effective priorityFee
	createTxn(0, 50, 102, 0) // 2
	createTxn(0, 5, 110, 1)  // 5

	createTxn(1, 8, 110, 2)  // 8
	createTxn(1, 20, 112, 3) // 12

	p.params.MaxGasInBlock = txGas * 2
	p.executionState.GasUsed = 0

	s.Run("Check block gas limit", func() {
		// Only two transactions fit into the block
		proposal := s.generateBlock(p)
		checkOrder(proposal, 2, 3)
		pool.Txns = nil
	})
}

func (s *ProposerTestSuite) getSeqno(addr types.Address) types.Seqno {
	s.T().Helper()

	roTx, err := s.db.CreateRoTx(s.T().Context())
	s.Require().NoError(err)
	defer roTx.Rollback()

	block, _, err := db.ReadLastBlock(roTx, addr.ShardId())
	s.Require().NoError(err)

	root := mpt.NewDbReader(roTx, addr.ShardId(), db.ContractTrieTable)
	root.SetRootHash(block.SmartContractsRoot)

	addressBytes := addr.Hash().Bytes()
	contractRaw, err := root.Get(addressBytes)
	s.Require().NoError(err)

	contract := new(types.SmartContract)
	err = contract.UnmarshalSSZ(contractRaw)
	s.Require().NoError(err)

	return contract.ExtSeqno
}

func (s *ProposerTestSuite) TestCollator() {
	to := contracts.CounterAddress(s.T(), s.shardId)

	pool := &MockTxnPool{}
	params := s.newParams()
	p := newTestProposer(params, pool)
	shardId := p.params.ShardId

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
		pool.Txns = []*types.Transaction{m1, m2}

		proposal := s.generateBlock(p)
		r1 := s.checkReceipt(shardId, m1)
		r2 := s.checkReceipt(shardId, m2)
		s.Equal(pool.Txns, proposal.ExternalTxns)

		pool.Txns = nil

		// Each transaction subtracts its value + actual gas used from the balance.
		balance = balance.
			Sub(txnValue).Sub(r1.GasUsed.ToValue(types.DefaultGasPrice)).Sub(feeCredit).
			Sub(txnValue).Sub(r2.GasUsed.ToValue(types.DefaultGasPrice)).Sub(feeCredit)
		s.Equal(balance, s.getMainBalance())
		s.Equal(types.Value{}, s.getBalance(shardId, to))
	})

	// Now process internal transactions by one to test queueing.
	p.params.MaxInternalTransactionsInBlock = 1

	s.Run("ProcessInternalTransaction1", func() {
		s.generateBlock(p)

		s.Equal(balance, s.getMainBalance())
		s.Equal(txnValue, s.getBalance(shardId, to))
	})

	s.Run("ProcessInternalTransaction2", func() {
		s.generateBlock(p)

		s.Equal(balance, s.getMainBalance())
		s.Equal(txnValue.Add(txnValue), s.getBalance(shardId, to))
	})

	p.params.MaxInternalTransactionsInBlock = defaultMaxInternalTxns

	s.Run("ProcessRefundTransactions", func() {
		s.generateBlock(p)

		balance = balance.Add(feeCredit).Add(feeCredit)
		s.Equal(balance, s.getMainBalance())

		// TODO: Enable when fixed uninitialized refunds
		// s.checkSeqno(shardId)
	})

	s.Run("DoNotProcessDuplicates", func() {
		pool.Txns = []*types.Transaction{m1, m2}

		proposal := s.generateBlock(p)
		s.Empty(proposal.ExternalTxns)
		s.Empty(proposal.InternalTxns)
		s.Empty(proposal.ForwardTxns)
		s.Equal(pool.Txns, pool.LastDiscarded)
		s.Equal(txnpool.DuplicateHash, pool.LastReason)
	})

	s.Run("Deploy", func() {
		m := execution.NewDeployTransaction(contracts.CounterDeployPayload(s.T()), shardId, to, 0, types.Value{})
		m.Flags.ClearBit(types.TransactionFlagInternal)
		s.Equal(to, m.To)
		pool.Txns = []*types.Transaction{m}

		s.generateBlock(p)
		pool.Txns = nil
		s.checkReceipt(shardId, m)
	})

	s.Run("Execute", func() {
		m := execution.NewExecutionTransaction(to, to, 0, contracts.NewCounterAddCallData(s.T(), 3))
		pool.Txns = []*types.Transaction{m}

		s.generateBlock(p)
		pool.Txns = nil
		s.checkReceipt(shardId, m)
	})

	s.Run("CheckRefundsSeqno", func() {
		m01 := execution.NewSendMoneyTransaction(s.T(), to, 2)
		m02 := execution.NewSendMoneyTransaction(s.T(), to, 3)
		pool.Txns = []*types.Transaction{m01, m02}

		// send tokens
		s.generateBlock(p)

		// process internal transactions
		s.generateBlock(p)

		// process refunds
		s.generateBlock(p)

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
