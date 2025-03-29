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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteExecutionState struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (s *SuiteExecutionState) SetupSuite() {
	s.ctx = context.Background()
}

func (s *SuiteExecutionState) SetupTest() {
	var err error
	s.db, err = db.NewBadgerDb(s.Suite.T().TempDir() + "test.db")
	s.Require().NoError(err)
}

func (s *SuiteExecutionState) TearDownTest() {
	s.db.Close()
}

func (s *SuiteExecutionState) TestExecState() {
	const shardId types.ShardId = 5
	const numTransactions types.Seqno = 10
	const code = "6004600c60003960046000f301020304"

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	addr := types.GenerateRandomAddress(shardId)
	storageKey := common.BytesToHash([]byte("storage-key"))

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	s.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	s.Run("CreateAccount", func() {
		s.Require().NoError(es.CreateAccount(addr))
		s.Require().NoError(es.SetState(addr, storageKey, common.IntToHash(123456)))
	})

	s.Run("DeployTransactions", func() {
		from := types.GenerateRandomAddress(shardId)
		for i := range numTransactions {
			Deploy(s.T(), s.ctx, es,
				types.BuildDeployPayload(hexutil.FromHex(code), common.BytesToHash([]byte{byte(i)})),
				shardId, from, i)
		}
	})

	var blockRes *BlockGenerationResult

	s.Run("CommitBlock", func() {
		blockRes, err = es.Commit(0, &types.ConsensusParams{})
		s.Require().NoError(err)
	})

	s.Run("CheckAccount", func() {
		es, err := NewExecutionState(tx, shardId, StateParams{
			Block:          blockRes.Block,
			ConfigAccessor: config.GetStubAccessor(),
		})
		s.Require().NoError(err)

		storageVal, err := es.GetState(addr, storageKey)
		s.Require().NoError(err)
		s.Equal(storageVal, common.IntToHash(123456))
	})

	s.Run("CheckTransactions", func() {
		data, err := es.shardAccessor.GetBlock().ByHash(blockRes.BlockHash)
		s.Require().NoError(err)
		s.Require().NotNil(data)
		s.Require().NotNil(data.Block())

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
			s.Require().NoError(err)

			deploy := types.BuildDeployPayload(
				hexutil.FromHex(code),
				common.BytesToHash([]byte{byte(transactionIndex)}))
			s.Equal(types.Code(deploy.Bytes()), m.Data)

			_, err = receiptsRoot.Fetch(transactionIndex)
			s.Require().NoError(err)

			transactionIndex++
		}
		s.Equal(types.TransactionIndex(numTransactions), transactionIndex)
	})

	s.Run("CommitTx", func() {
		s.Require().NoError(tx.Commit())
	})
}

func (s *SuiteExecutionState) TestDeployAndCall() {
	shardId := types.ShardId(5)

	payload := contracts.CounterDeployPayload(s.T())
	addrSmartAccount := types.CreateAddress(shardId, payload)

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	s.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	s.Run("Deploy", func() {
		seqno, err := es.GetSeqno(addrSmartAccount)
		s.Require().NoError(err)
		s.EqualValues(0, seqno)

		Deploy(s.T(), s.ctx, es, payload, shardId, types.Address{}, 0)

		seqno, err = es.GetSeqno(addrSmartAccount)
		s.Require().NoError(err)
		s.EqualValues(1, seqno)
	})

	s.Run("Execute", func() {
		txn := NewExecutionTransaction(addrSmartAccount, addrSmartAccount, 1,
			contracts.NewCounterAddCallData(s.T(), 47))
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.Require().False(res.Failed())

		seqno, err := es.GetSeqno(addrSmartAccount)
		s.Require().NoError(err)
		s.EqualValues(1, seqno)

		extSeqno, err := es.GetExtSeqno(addrSmartAccount)
		s.Require().NoError(err)
		s.EqualValues(1, extSeqno)
	})
}

func (s *SuiteExecutionState) TestExecStateMultipleBlocks() {
	createTx := func(index int) *types.Transaction {
		txn := types.NewEmptyTransaction()
		txn.Data = []byte{byte(index)}
		txn.Seqno = types.Seqno(index)
		return txn
	}

	txn1 := createTx(1)
	txn2 := createTx(2)
	blockHash1 := GenerateBlockFromTransactionsWithoutExecution(s.T(), context.Background(),
		types.BaseShardId, 0, common.EmptyHash, s.db, txn1, txn2)

	txn3 := createTx(3)
	blockHash2 := GenerateBlockFromTransactionsWithoutExecution(s.T(), context.Background(),
		types.BaseShardId, 1, blockHash1, s.db, txn3)

	tx, err := s.db.CreateRoTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	check := func(blockHash common.Hash, idx types.TransactionIndex, txn *types.Transaction) {
		block, err := db.ReadBlock(tx, types.BaseShardId, blockHash)
		s.Require().NoError(err)
		s.Require().NotNil(block)

		transactionsRoot := NewDbTransactionTrieReader(tx, types.BaseShardId)
		transactionsRoot.SetRootHash(block.InTransactionsRoot)
		txnRead, err := transactionsRoot.Fetch(idx)
		s.Require().NoError(err)

		s.EqualValues(txn, txnRead)
	}

	check(blockHash1, 0, txn1)
	check(blockHash1, 1, txn2)
	check(blockHash2, 0, txn3)
}

func TestSuiteExecutionState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteExecutionState))
}

func newState(t *testing.T) *ExecutionState {
	t.Helper()

	ctx := t.Context()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)

	cfgAccessor, err := config.NewConfigAccessorTx(tx, nil)
	require.NoError(t, err)

	require.NoError(t, config.SetParamGasPrice(cfgAccessor, &config.ParamGasPrice{
		Shards: []types.Uint256{*types.NewUint256(10), *types.NewUint256(10), *types.NewUint256(10)},
	}))

	state, err := NewExecutionState(tx, types.BaseShardId, StateParams{
		ConfigAccessor: cfgAccessor,
	})
	state.BaseFee = types.DefaultGasPrice
	require.NoError(t, err)

	defaultZeroStateConfig, err := CreateDefaultZeroStateConfig(MainPublicKey)
	require.NoError(t, err)
	err = state.GenerateZeroState(defaultZeroStateConfig)
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

	require.Zero(t, s.GetCommittedState(stateobjaddr, storageaddr))

	// revert up to the genesis state and ensure correct content
	s.RevertToSnapshot(genesis)
	v, err = s.GetState(stateobjaddr, storageaddr)
	require.NoError(t, err)
	assert.Empty(t, v)
	require.Zero(t, s.GetCommittedState(stateobjaddr, storageaddr))
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

	_, err = state.Commit(0, &types.ConsensusParams{})
	require.NoError(t, err)

	// Drop local state account cache
	delete(state.Accounts, addr)

	acc, err = state.GetAccount(addr)
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, balance, acc.Balance)
}

func (s *SuiteExecutionState) TestTransactionStatus() {
	shardId := types.ShardId(5)
	var vmErrStub *types.VmError

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	s.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	var counterAddr, faucetAddr types.Address

	s.Run("Deploy", func() {
		counterAddr = Deploy(s.T(), s.ctx, es,
			contracts.CounterDeployPayload(s.T()), shardId, types.Address{}, 0)

		faucetAddr = Deploy(s.T(), s.ctx, es,
			contracts.FaucetDeployPayload(s.T()), shardId, types.Address{}, 0)
		s.Require().NoError(es.SetBalance(faucetAddr, types.NewValueFromUint64(100_000_000)))
	})

	s.Run("ExecuteOutOfGas", func() {
		txn := types.NewEmptyTransaction()
		txn.To = counterAddr
		txn.Data = contracts.NewCounterAddCallData(s.T(), 47)
		txn.Seqno = 1
		txn.FeeCredit = toGasCredit(0)
		txn.MaxFeePerGas = types.MaxFeePerGasDefault
		txn.From = counterAddr
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.Equal(types.ErrorOutOfGas, res.Error.Code())
		s.Require().ErrorAs(res.Error, &vmErrStub)
	})

	s.Run("ExecuteReverted", func() {
		txn := types.NewEmptyTransaction()
		txn.To = counterAddr
		txn.Data = []byte("wrong calldata")
		txn.Seqno = 1
		txn.FeeCredit = toGasCredit(1_000_000)
		txn.MaxFeePerGas = types.MaxFeePerGasDefault
		txn.From = counterAddr
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		fmt.Println(res.Error.Error())
		s.Equal(types.ErrorExecutionReverted, res.Error.Code())
		s.Require().ErrorAs(res.Error, &vmErrStub)
	})

	s.Run("CallToMainShard", func() {
		txn := types.NewEmptyTransaction()
		txn.To = faucetAddr
		txn.Data = contracts.NewFaucetWithdrawToCallData(s.T(),
			types.GenerateRandomAddress(types.MainShardId), types.NewValueFromUint64(1_000))
		txn.Seqno = 1
		txn.FeeCredit = toGasCredit(100_000)
		txn.MaxFeePerGas = types.MaxFeePerGasDefault
		txn.From = faucetAddr
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.Equal(types.ErrorTransactionToMainShard, res.Error.Code())
		s.Require().ErrorAs(res.Error, &vmErrStub)
	})

	s.Run("Errors with transactions", func() {
		err = vm.StackUnderflowError(0, 1, 2)
		s.Require().ErrorAs(err, &vmErrStub)
		s.Equal(types.ErrorStackUnderflow, types.GetErrorCode(err))
		s.Equal("StackUnderflow: stack:0 < required:1, opcode: MUL", err.Error())

		err = vm.StackOverflowError(1, 0, 2)
		s.Require().ErrorAs(err, &vmErrStub)
		s.Equal(types.ErrorStackOverflow, types.GetErrorCode(err))
		s.Equal("StackOverflow: stack: 1, limit: 0, opcode: MUL", err.Error())

		err = vm.InvalidOpCodeError(4)
		s.Require().ErrorAs(err, &vmErrStub)
		s.Equal(types.ErrorInvalidOpcode, types.GetErrorCode(err))
		s.Equal("InvalidOpcode: invalid opcode: DIV", err.Error())
	})
}

func (s *SuiteExecutionState) TestPrecompiles() {
	shardId := types.ShardId(1)

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()
	var testAddr types.Address

	es := newState(s.T())
	es.BaseFee = types.DefaultGasPrice
	s.Require().NoError(err)

	s.Run("Deploy", func() {
		code, err := contracts.GetCode(contracts.NamePrecompilesTest)
		s.Require().NoError(err)
		testAddr = Deploy(
			s.T(), s.ctx, es, types.BuildDeployPayload(code, common.EmptyHash), shardId, types.Address{}, 0)
	})

	abi, err := contracts.GetAbi(contracts.NamePrecompilesTest)
	s.Require().NoError(err)

	txn := types.NewEmptyTransaction()
	txn.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	txn.To = testAddr
	txn.Data = []byte("wrong calldata")
	txn.Seqno = 1
	txn.FeeCredit = toGasCredit(1_000_000)
	txn.MaxFeePerGas = types.MaxFeePerGasDefault
	txn.From = testAddr

	s.Run("testAsyncCall: success", func() {
		txn.Data, err = abi.Pack(
			"testAsyncCall",
			testAddr,
			types.EmptyAddress,
			types.EmptyAddress,
			big.NewInt(0),
			uint8(types.ForwardKindNone),
			big.NewInt(0),
			[]byte{})
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.False(res.Failed())
	})

	s.Run("testAsyncCall: Send to main shard", func() {
		txn.Data, err = abi.Pack(
			"testAsyncCall",
			types.EmptyAddress,
			types.EmptyAddress,
			types.EmptyAddress,
			big.NewInt(0),
			uint8(types.ForwardKindNone),
			big.NewInt(0),
			[]byte{1, 2, 3, 4})
		s.Require().NoError(err)

		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorTransactionToMainShard, res.Error.Code())
	})

	s.Run("testAsyncCall: withdrawFunds failed", func() {
		txn.Data, err = abi.Pack("testAsyncCall", testAddr, types.EmptyAddress, types.EmptyAddress, big.NewInt(0),
			uint8(types.ForwardKindNone), big.NewInt(1_000_000_000_000_000), []byte{1, 2, 3, 4})
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorInsufficientBalance, res.Error.Code())
	})

	payload := &types.InternalTransactionPayload{
		To: testAddr,
	}

	s.Run("testSendRawTxn: invalid transaction", func() {
		txn.Data, err = abi.Pack("testSendRawTxn", []byte{1, 2})
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorInvalidTransactionInputUnmarshalFailed, res.Error.Code())
	})

	s.Run("testSendRawTxn: send to main shard", func() {
		payload.To = types.GenerateRandomAddress(0)
		data, err := payload.MarshalSSZ()
		s.Require().NoError(err)
		txn.Data, err = abi.Pack("testSendRawTxn", data)
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorTransactionToMainShard, res.Error.Code())
		payload.To = testAddr
	})

	s.Run("testSendRawTxn: withdraw value failed", func() {
		payload.Value = types.NewValueFromUint64(1_000_000_000_000_000)
		data, err := payload.MarshalSSZ()
		s.Require().NoError(err)
		txn.Data, err = abi.Pack("testSendRawTxn", data)
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorInsufficientBalance, res.Error.Code())
	})

	s.Run("testSendRawTxn: withdraw feeCredit failed", func() {
		payload.Value = types.NewZeroValue()
		payload.FeeCredit = types.NewValueFromUint64(1_000_000_000_000_000)
		payload.ForwardKind = types.ForwardKindNone
		data, err := payload.MarshalSSZ()
		s.Require().NoError(err)
		txn.Data, err = abi.Pack("testSendRawTxn", data)
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorInsufficientBalance, res.Error.Code())
	})

	s.Run("testTokenBalance: cross shard", func() {
		txn.Data, err = abi.Pack("testTokenBalance", types.GenerateRandomAddress(0),
			types.TokenId(types.HexToAddress("0x0a")))
		s.Require().NoError(err)
		res := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
		s.True(res.Failed())
		s.Equal(types.ErrorCrossShardTransaction, res.Error.Code())
	})

	s.Run("Test required gas for outbound transactions", func() {
		gasPrice := types.DefaultGasPrice
		gasScale := types.DefaultGasPrice.Div(types.Value100)

		state := &vm.StateDBReadOnlyMock{
			GetGasPriceFunc: func(shardId types.ShardId) (types.Value, error) {
				return gasPrice, nil
			},
		}
		gas := vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		s.Zero(gas)

		gasPrice = types.DefaultGasPrice.Sub(gasScale.Mul(types.Value10))
		gas = vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		s.Zero(gas)

		gasPrice = types.DefaultGasPrice.Add(gasScale.Mul(types.Value10))
		gas = vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		s.EqualValues(vm.ExtraForwardFeeStep*10, gas)

		gasPrice = types.DefaultGasPrice.Add(gasScale.Mul(types.NewValueFromUint64(101)))
		gas = vm.GetExtraGasForOutboundTransaction(state, types.ShardId(2))
		s.EqualValues(vm.ExtraForwardFeeStep*101, gas)
	})
}

func (s *SuiteExecutionState) TestPanic() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	txMock := db.NewTxMock(tx)

	es, err := NewExecutionState(txMock, types.ShardId(1), StateParams{
		ConfigAccessor: config.GetStubAccessor(),
	})
	s.Require().NoError(err)
	es.BaseFee = types.DefaultGasPrice

	// Check normal execution is ok
	txn := NewExecutionTransaction(types.MainSmartAccountAddress, types.MainSmartAccountAddress, 1,
		contracts.NewSmartAccountSendCallData(s.T(), []byte(""), types.Gas(500_000), types.Value0, nil,
			types.MainSmartAccountAddress, types.ExecutionTransactionKind))
	execResult := es.AddAndHandleTransaction(s.ctx, txn, dummyPayer{})
	s.False(execResult.Failed())

	// Check panic is handled correctly
	txMock.GetFromShardFunc = func(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
		panic("test panic")
	}
	execResult = es.HandleTransaction(s.ctx, txn, dummyPayer{})
	s.True(execResult.Failed())
	s.False(execResult.IsFatal())
	s.Equal("PanicDuringExecution: panic transaction: test panic", execResult.Error.Error())
}

func BenchmarkBlockGeneration(b *testing.B) {
	ctx := b.Context()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(b, err)
	logging.SetupGlobalLogger("error")

	address, err := contracts.CalculateAddress(contracts.NameCounter, 1, nil)
	require.NoError(b, err)

	zeroState := &ZeroStateConfig{
		Contracts: []*ContractDescr{
			{Name: "Counter", Contract: "tests/Counter", Address: address, Value: types.NewValueFromUint64(10000000)},
		},
	}

	params := NewBlockGeneratorParams(1, 2)

	gen, err := NewBlockGenerator(ctx, params, database, nil)
	require.NoError(b, err)
	_, err = gen.GenerateZeroState(zeroState)
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

	proposal := &Proposal{}
	for range 1000 {
		proposal.InternalTxns = append(proposal.InternalTxns, txn)
	}

	b.ResetTimer()

	for range b.N {
		tx, _ := database.CreateRwTx(ctx)
		proposal.PrevBlockHash, _ = db.ReadLastBlockHash(tx, 1)

		gen, err = NewBlockGenerator(ctx, params, database, nil)
		require.NoError(b, err)
		_, err = gen.GenerateBlock(proposal, &types.ConsensusParams{})
		require.NoError(b, err)

		tx.Rollback()
	}
}
