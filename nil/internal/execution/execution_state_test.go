package execution

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteExecutionState struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (suite *SuiteExecutionState) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *SuiteExecutionState) SetupTest() {
	var err error
	suite.db, err = db.NewBadgerDb(suite.Suite.T().TempDir() + "test.db")
	suite.Require().NoError(err)
}

func (suite *SuiteExecutionState) TearDownTest() {
	suite.db.Close()
}

func (suite *SuiteExecutionState) TestExecState() {
	const shardId types.ShardId = 5
	const numTransactions types.Seqno = 10
	const code = "6004600c60003960046000f301020304"

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	addr := types.GenerateRandomAddress(shardId)
	storageKey := common.BytesToHash([]byte("storage-key"))

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)

	suite.Run("CreateAccount", func() {
		suite.Require().NoError(es.CreateAccount(addr))
		suite.Require().NoError(es.SetState(addr, storageKey, common.IntToHash(123456)))
	})

	suite.Run("DeployTransactions", func() {
		from := types.GenerateRandomAddress(shardId)
		for i := range numTransactions {
			Deploy(suite.T(), suite.ctx, es,
				types.BuildDeployPayload(hexutil.FromHex(code), common.BytesToHash([]byte{byte(i)})),
				shardId, from, i)
		}
	})

	var blockHash common.Hash

	suite.Run("CommitBlock", func() {
		blockHash, _, err = es.Commit(0, nil)
		suite.Require().NoError(err)
	})

	suite.Run("CheckAccount", func() {
		es, err := NewExecutionState(tx, shardId, StateParams{
			BlockHash:      blockHash,
			ConfigAccessor: config.GetStubAccessor(),
		})
		suite.Require().NoError(err)

		storageVal, err := es.GetState(addr, storageKey)
		suite.Require().NoError(err)
		suite.Equal(storageVal, common.IntToHash(123456))
	})

	suite.Run("CheckTransactions", func() {
		data, err := es.shardAccessor.GetBlock().ByHash(blockHash)
		suite.Require().NoError(err)
		suite.Require().NotNil(data)
		suite.Require().NotNil(data.Block())

		transactionsRoot := NewDbTransactionTrieReader(tx, es.ShardId)
		transactionsRoot.SetRootHash(data.Block().InTransactionsRoot)
		receiptsRoot := NewDbReceiptTrieReader(tx, es.ShardId)
		receiptsRoot.SetRootHash(data.Block().ReceiptsRoot)

		var transactionIndex types.TransactionIndex
		for {
			m, err := transactionsRoot.Fetch(transactionIndex)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			suite.Require().NoError(err)

			deploy := types.BuildDeployPayload(hexutil.FromHex(code), common.BytesToHash([]byte{byte(transactionIndex)}))
			suite.Equal(types.Code(deploy.Bytes()), m.Data)

			_, err = receiptsRoot.Fetch(transactionIndex)
			suite.Require().NoError(err)

			transactionIndex++
		}
		suite.Equal(types.TransactionIndex(numTransactions), transactionIndex)
	})

	suite.Run("CommitTx", func() {
		suite.Require().NoError(tx.Commit())
	})
}

func (suite *SuiteExecutionState) TestDeployAndCall() {
	shardId := types.ShardId(5)

	payload := contracts.CounterDeployPayload(suite.T())
	addrSmartAccount := types.CreateAddress(shardId, payload)

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)

	suite.Run("Deploy", func() {
		seqno, err := es.GetSeqno(addrSmartAccount)
		suite.Require().NoError(err)
		suite.EqualValues(0, seqno)

		Deploy(suite.T(), suite.ctx, es, payload, shardId, types.Address{}, 0)

		seqno, err = es.GetSeqno(addrSmartAccount)
		suite.Require().NoError(err)
		suite.EqualValues(1, seqno)
	})

	suite.Run("Execute", func() {
		txn := NewExecutionTransaction(addrSmartAccount, addrSmartAccount, 1,
			contracts.NewCounterAddCallData(suite.T(), 47))
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.Require().False(res.Failed())

		seqno, err := es.GetSeqno(addrSmartAccount)
		suite.Require().NoError(err)
		suite.EqualValues(1, seqno)

		extSeqno, err := es.GetExtSeqno(addrSmartAccount)
		suite.Require().NoError(err)
		suite.EqualValues(1, extSeqno)
	})
}

func (suite *SuiteExecutionState) TestExecStateMultipleBlocks() {
	txn1 := types.NewEmptyTransaction()
	txn1.Data = []byte{1}
	txn1.Seqno = 1
	txn2 := types.NewEmptyTransaction()
	txn2.Data = []byte{2}
	txn2.Seqno = 2
	blockHash1 := GenerateBlockFromTransactionsWithoutExecution(suite.T(), context.Background(),
		types.BaseShardId, 0, common.EmptyHash, suite.db, txn1, txn2)
	blockHash2 := GenerateBlockFromTransactionsWithoutExecution(suite.T(), context.Background(),
		types.BaseShardId, 1, blockHash1, suite.db, txn2)

	tx, err := suite.db.CreateRoTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	check := func(blockHash common.Hash, idx types.TransactionIndex, txn *types.Transaction) {
		block, err := db.ReadBlock(tx, types.BaseShardId, blockHash)
		suite.Require().NoError(err)
		suite.Require().NotNil(block)

		transactionsRoot := NewDbTransactionTrieReader(tx, types.BaseShardId)
		transactionsRoot.SetRootHash(block.InTransactionsRoot)
		txnRead, err := transactionsRoot.Fetch(idx)
		suite.Require().NoError(err)

		suite.EqualValues(txn, txnRead)
	}

	check(blockHash1, 0, txn1)
	check(blockHash1, 1, txn2)
	check(blockHash2, 0, txn2)
}

func TestSuiteExecutionState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteExecutionState))
}

func newState(t *testing.T) *ExecutionState {
	t.Helper()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(context.Background())
	require.NoError(t, err)
	state, err := NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	require.NoError(t, err)

	err = state.GenerateZeroStateYaml(DefaultZeroStateConfig)
	require.NoError(t, err)
	return state
}

func TestStorage(t *testing.T) {
	t.Parallel()

	state := newState(t)
	defer state.tx.Rollback()

	account := types.GenerateRandomAddress(types.BaseShardId)
	key := common.EmptyHash
	value := common.IntToHash(42)

	num, err := state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, common.EmptyHash)

	exists, err := state.Exists(account)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, state.CreateAccount(account))

	exists, err = state.Exists(account)
	require.NoError(t, err)
	require.True(t, exists)

	require.NoError(t, state.SetState(account, key, value))

	num, err = state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	t.Parallel()

	state := newState(t)
	defer state.tx.Rollback()
	account := types.GenerateRandomAddress(types.BaseShardId)

	require.NoError(t, state.SetBalance(account, types.NewValueFromUint64(100500)))

	balance, err := state.GetBalance(account)
	require.NoError(t, err)
	require.Equal(t, types.NewValueFromUint64(100500), balance)
}

func TestSnapshot(t *testing.T) {
	t.Parallel()
	stateobjaddr := types.GenerateRandomAddress(types.BaseShardId)
	var storageaddr common.Hash
	data1 := common.BytesToHash([]byte{42})
	data2 := common.BytesToHash([]byte{43})
	s := newState(t)
	defer s.tx.Rollback()

	// snapshot the genesis state
	genesis := s.Snapshot()

	// set initial state object value
	require.NoError(t, s.SetState(stateobjaddr, storageaddr, data1))
	snapshot := s.Snapshot()

	// set a new state object value, revert it and ensure correct content
	require.NoError(t, s.SetState(stateobjaddr, storageaddr, data2))
	s.RevertToSnapshot(snapshot)

	v, err := s.GetState(stateobjaddr, storageaddr)
	require.NoError(t, err)
	assert.Equal(t, data1, v)

	if v := s.GetCommittedState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong committed storage value %v, want %v", v, common.Hash{})
	}

	// revert up to the genesis state and ensure correct content
	s.RevertToSnapshot(genesis)
	v, err = s.GetState(stateobjaddr, storageaddr)
	require.NoError(t, err)
	assert.Empty(t, v)
	if v := s.GetCommittedState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong committed storage value %v, want %v", v, common.Hash{})
	}
}

func TestSnapshotEmpty(t *testing.T) {
	t.Parallel()
	s := newState(t)
	defer s.tx.Rollback()
	s.RevertToSnapshot(s.Snapshot())
}

func TestCreateObjectRevert(t *testing.T) {
	t.Parallel()
	state := newState(t)
	defer state.tx.Rollback()
	addr := types.GenerateRandomAddress(types.BaseShardId)
	snap := state.Snapshot()

	require.NoError(t, state.CreateAccount(addr))

	so0, err := state.GetAccount(addr)
	require.NoError(t, err)
	so0.SetBalance(types.NewValueFromUint64(42))
	so0.SetSeqno(43)
	code := types.Code([]byte{'c', 'a', 'f', 'e'})
	so0.SetCode(code.Hash(), code)
	state.setAccountObject(so0)

	state.RevertToSnapshot(snap)
	exists, err := state.Exists(addr)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAccountState(t *testing.T) {
	t.Parallel()
	state := newState(t)
	defer state.tx.Rollback()
	addr := types.GenerateRandomAddress(types.BaseShardId)

	require.NoError(t, state.CreateAccount(addr))

	balance := types.NewValueFromUint64(42)
	acc, err := state.GetAccount(addr)
	require.NoError(t, err)
	acc.SetBalance(balance)
	acc.SetSeqno(43)
	code := types.Code([]byte{'c', 'a', 'f', 'e'})
	acc.SetCode(code.Hash(), code)

	_, _, err = state.Commit(0, nil)
	require.NoError(t, err)

	// Drop local state account cache
	delete(state.Accounts, addr)

	acc, err = state.GetAccount(addr)
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, balance, acc.Balance)
}

func (suite *SuiteExecutionState) TestTransactionStatus() {
	shardId := types.ShardId(5)
	var vmErrStub *types.VmError

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)

	var counterAddr, faucetAddr types.Address

	suite.Run("Deploy", func() {
		counterAddr = Deploy(suite.T(), suite.ctx, es,
			contracts.CounterDeployPayload(suite.T()), shardId, types.Address{}, 0)

		faucetAddr = Deploy(suite.T(), suite.ctx, es,
			contracts.FaucetDeployPayload(suite.T()), shardId, types.Address{}, 0)
		suite.Require().NoError(es.SetBalance(faucetAddr, types.NewValueFromUint64(100_000_000)))
	})

	suite.Run("ExecuteOutOfGas", func() {
		txn := types.NewEmptyTransaction()
		txn.To = counterAddr
		txn.Data = contracts.NewCounterAddCallData(suite.T(), 47)
		txn.Seqno = 1
		txn.FeeCredit = toGasCredit(0)
		txn.From = counterAddr
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.Equal(types.ErrorOutOfGas, res.Error.Code())
		suite.Require().ErrorAs(res.Error, &vmErrStub)
	})

	suite.Run("ExecuteReverted", func() {
		txn := types.NewEmptyTransaction()
		txn.To = counterAddr
		txn.Data = []byte("wrong calldata")
		txn.Seqno = 1
		txn.FeeCredit = toGasCredit(1_000_000)
		txn.From = counterAddr
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		fmt.Println(res.Error.Error())
		suite.Equal(types.ErrorExecutionReverted, res.Error.Code())
		suite.Require().ErrorAs(res.Error, &vmErrStub)
	})

	suite.Run("CallToMainShard", func() {
		txn := types.NewEmptyTransaction()
		txn.To = faucetAddr
		txn.Data = contracts.NewFaucetWithdrawToCallData(suite.T(),
			types.GenerateRandomAddress(types.MainShardId), types.NewValueFromUint64(1_000))
		txn.Seqno = 1
		txn.FeeCredit = toGasCredit(100_000)
		txn.From = faucetAddr
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.Equal(types.ErrorMessageToMainShard, res.Error.Code())
		suite.Require().ErrorAs(res.Error, &vmErrStub)
	})

	suite.Run("Errors with transactions", func() {
		err = vm.StackUnderflowError(0, 1, 2)
		suite.Require().ErrorAs(err, &vmErrStub)
		suite.Equal(types.ErrorStackUnderflow, types.GetErrorCode(err))
		suite.Equal("StackUnderflow: stack:0 < required:1, opcode: MUL", err.Error())

		err = vm.StackOverflowError(1, 0, 2)
		suite.Require().ErrorAs(err, &vmErrStub)
		suite.Equal(types.ErrorStackOverflow, types.GetErrorCode(err))
		suite.Equal("StackOverflow: stack: 1, limit: 0, opcode: MUL", err.Error())

		err = vm.InvalidOpCodeError(4)
		suite.Require().ErrorAs(err, &vmErrStub)
		suite.Equal(types.ErrorInvalidOpcode, types.GetErrorCode(err))
		suite.Equal("InvalidOpcode: invalid opcode: DIV", err.Error())
	})
}

func (suite *SuiteExecutionState) TestPrecompiles() {
	shardId := types.ShardId(1)

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()
	var testAddr types.Address

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)

	suite.Run("Deploy", func() {
		code, err := contracts.GetCode(contracts.NamePrecompilesTest)
		suite.Require().NoError(err)
		testAddr = Deploy(suite.T(), suite.ctx, es, types.BuildDeployPayload(code, common.EmptyHash), shardId, types.Address{}, 0)
	})

	abi, err := contracts.GetAbi(contracts.NamePrecompilesTest)
	suite.Require().NoError(err)

	txn := types.NewEmptyTransaction()
	txn.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	txn.To = testAddr
	txn.Data = []byte("wrong calldata")
	txn.Seqno = 1
	txn.FeeCredit = toGasCredit(1_000_000)
	txn.From = testAddr

	suite.Run("testAsyncCall: success", func() {
		txn.Data, err = abi.Pack("testAsyncCall", testAddr, types.EmptyAddress, types.EmptyAddress, big.NewInt(0),
			uint8(types.ForwardKindNone), big.NewInt(0), []byte{})
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.False(res.Failed())
	})

	suite.Run("testAsyncCall: Send to main shard", func() {
		txn.Data, err = abi.Pack("testAsyncCall", types.EmptyAddress, types.EmptyAddress, types.EmptyAddress, big.NewInt(0),
			uint8(types.ForwardKindNone), big.NewInt(0), []byte{1, 2, 3, 4})
		suite.Require().NoError(err)

		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorMessageToMainShard, res.Error.Code())
	})

	suite.Run("testAsyncCall: withdrawFunds failed", func() {
		txn.Data, err = abi.Pack("testAsyncCall", testAddr, types.EmptyAddress, types.EmptyAddress, big.NewInt(0),
			uint8(types.ForwardKindNone), big.NewInt(1_000_000_000_000_000), []byte{1, 2, 3, 4})
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorInsufficientBalance, res.Error.Code())
	})

	payload := &types.InternalTransactionPayload{
		To: testAddr,
	}

	suite.Run("testSendRawTxn: invalid transaction", func() {
		txn.Data, err = abi.Pack("testSendRawTxn", []byte{1, 2})
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorInvalidTransactionInputUnmarshalFailed, res.Error.Code())
	})

	suite.Run("testSendRawTxn: send to main shard", func() {
		payload.To = types.GenerateRandomAddress(0)
		data, err := payload.MarshalSSZ()
		suite.Require().NoError(err)
		txn.Data, err = abi.Pack("testSendRawTxn", data)
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorMessageToMainShard, res.Error.Code())
		payload.To = testAddr
	})

	suite.Run("testSendRawTxn: withdraw value failed", func() {
		payload.Value = types.NewValueFromUint64(1_000_000_000_000_000)
		data, err := payload.MarshalSSZ()
		suite.Require().NoError(err)
		txn.Data, err = abi.Pack("testSendRawTxn", data)
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorInsufficientBalance, res.Error.Code())
	})

	suite.Run("testSendRawTxn: withdraw feeCredit failed", func() {
		payload.Value = types.NewZeroValue()
		payload.FeeCredit = types.NewValueFromUint64(1_000_000_000_000_000)
		payload.ForwardKind = types.ForwardKindNone
		data, err := payload.MarshalSSZ()
		suite.Require().NoError(err)
		txn.Data, err = abi.Pack("testSendRawTxn", data)
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorInsufficientBalance, res.Error.Code())
	})

	suite.Run("testTokenBalance: cross shard", func() {
		txn.Data, err = abi.Pack("testTokenBalance", types.GenerateRandomAddress(0),
			types.TokenId(types.HexToAddress("0x0a")))
		suite.Require().NoError(err)
		res := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
		suite.True(res.Failed())
		suite.Equal(types.ErrorCrossShardTransaction, res.Error.Code())
	})

	suite.Run("Test required gas for outbound transactions", func() {
		gasPrice := types.DefaultGasPrice
		gasScale := types.DefaultGasPrice.Div(types.Value100)

		state := &vm.StateDBReadOnlyMock{
			GetGasPriceFunc: func(shardId types.ShardId) (types.Value, error) {
				return gasPrice, nil
			},
		}
		gas := vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		suite.Zero(gas)

		gasPrice = types.DefaultGasPrice.Sub(gasScale.Mul(types.Value10))
		gas = vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		suite.Zero(gas)

		gasPrice = types.DefaultGasPrice.Add(gasScale.Mul(types.Value10))
		gas = vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		suite.EqualValues(vm.ExtraForwardFeeStep*10, gas)

		gasPrice = types.DefaultGasPrice.Add(gasScale.Mul(types.NewValueFromUint64(101)))
		gas = vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		suite.EqualValues(vm.ExtraForwardFeeStep*101, gas)
	})
}

func (suite *SuiteExecutionState) TestPanic() {
	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	txMock := db.NewTxMock(tx)

	es, err := NewExecutionState(txMock, types.ShardId(1), StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	suite.Require().NoError(err)

	// Check normal execution is ok
	txn := NewExecutionTransaction(types.MainSmartAccountAddress, types.MainSmartAccountAddress, 1,
		contracts.NewSmartAccountSendCallData(suite.T(), []byte(""), types.Gas(500_000), types.Value0, nil,
			types.MainSmartAccountAddress, types.ExecutionTransactionKind))
	execResult := es.HandleTransaction(suite.ctx, txn, dummyPayer{})
	suite.False(execResult.Failed())

	// Check panic is handled correctly
	txMock.GetFromShardFunc = func(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
		panic("test panic")
	}
	execResult = es.HandleTransaction(suite.ctx, txn, dummyPayer{})
	suite.True(execResult.Failed())
	suite.False(execResult.IsFatal())
	suite.Equal("PanicDuringExecution: panic transaction: test panic", execResult.Error.Error())
}

func BenchmarkBlockGeneration(b *testing.B) {
	ctx := context.Background()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(b, err)
	logging.SetupGlobalLogger("error")
	logger := zerolog.Nop()

	address, err := contracts.CalculateAddress(contracts.NameCounter, 1, nil)
	require.NoError(b, err)

	zerostateCfg := fmt.Sprintf(`
contracts:
- name: Counter
  address: %s
  value: 10000000
  contract: tests/Counter
`, address.Hex())

	params := NewBlockGeneratorParams(1, 2, types.DefaultGasPrice, 0)

	gen, err := NewBlockGenerator(ctx, params, database)
	require.NoError(b, err)
	_, err = gen.GenerateZeroState(zerostateCfg, nil)
	require.NoError(b, err)

	txn := types.NewEmptyTransaction()
	txn.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	txn.To = address
	txn.From = address
	txn.RefundTo = address
	txn.FeeCredit = types.NewValueFromUint64(10_000_000)

	abi, err := contracts.GetAbi(contracts.NameCounter)
	require.NoError(b, err)
	txn.Data, err = abi.Pack("add", int32(1))
	require.NoError(b, err)

	proposal := NewEmptyProposal()
	for range 1000 {
		proposal.InTxns = append(proposal.InTxns, txn)
	}

	b.ResetTimer()

	for range b.N {
		tx, _ := database.CreateRwTx(ctx)
		proposal.PrevBlockHash, _ = db.ReadLastBlockHash(tx, 1)

		gen, err = NewBlockGenerator(ctx, params, database)
		require.NoError(b, err)
		_, err = gen.GenerateBlock(proposal, logger, nil)
		require.NoError(b, err)

		tx.Rollback()
	}
}
