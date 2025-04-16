package execution

import (
	"fmt"
	"math/big"
	"math/rand"
	"slices"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var defaultMaxFeePerGas = types.MaxFeePerGasDefault

func deployContract(t *testing.T, contract *compiler.Contract, state *ExecutionState, seqno types.Seqno) types.Address {
	t.Helper()

	return Deploy(t, state, types.BuildDeployPayload(hexutil.FromHex(contract.Code), common.EmptyHash),
		types.BaseShardId, types.Address{}, seqno)
}

func TestOpcodes(t *testing.T) {
	t.Parallel()

	address := types.BytesToAddress([]byte("contract"))
	address[1] = 1

	codeTemplate := []byte{
		byte(vm.PUSH1), 0, // retSize
		byte(vm.PUSH1), 0, // retOffset
		byte(vm.PUSH1), 0, // argSize
		byte(vm.PUSH1), 0, // argOffset
		byte(vm.PUSH1), 0, // value
		byte(vm.PUSH32), // address
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		byte(vm.GAS),
		byte(vm.CALL),
		byte(vm.STOP),
	}

	// initialize a random generator with a fixed seed
	// to make the test deterministic
	rnd := rand.New(rand.NewSource(1543)) //nolint:gosec

	check := func(i int) {
		state := newState(t)
		defer state.tx.Rollback()

		require.NoError(t, state.CreateAccount(address))
		require.NoError(t, state.SetBalance(address, types.NewValueFromUint64(1_000_000_000)))
		code := slices.Clone(codeTemplate)

		for range 50 {
			position := rnd.Int() % len(code)
			code[position] = byte(rnd.Int() % 256)

			require.NoError(t, state.SetCode(address, code))

			require.NoError(t, state.newVm(true, address))
			_, _, _ = state.evm.Call(vm.AccountRef(address), address, nil, 100000, new(uint256.Int))
		}
		_, err := state.Commit(types.BlockNumber(i), nil)
		require.NoError(t, err)
	}
	for i := range 50 {
		check(i)
	}
}

func TestPrecompiles(t *testing.T) {
	t.Parallel()

	state := newState(t)
	defer state.tx.Rollback()

	// Test checks that precompiles are not crashed
	// if called with an empty input data
	check := func(i int) {
		require.NoError(t, state.newVm(true, types.EmptyAddress))

		callTransaction := types.NewEmptyTransaction()
		callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
		callTransaction.FeeCredit = toGasCredit(100_000)
		callTransaction.MaxFeePerGas = defaultMaxFeePerGas
		callTransaction.Seqno = types.Seqno(i)
		state.AddInTransaction(callTransaction)

		addr := fmt.Sprintf("%x", i)
		_, _, _ = state.evm.Call(
			vm.AccountRef(types.EmptyAddress), types.HexToAddress(addr), nil, 100000, new(uint256.Int))
	}
	for i := range 1000 {
		check(i)
	}
}

func toGasCredit(gas types.Gas) types.Value {
	return gas.ToValue(types.DefaultGasPrice)
}

func TestCall(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	state := newState(t)
	defer state.tx.Rollback()

	contracts, err := solc.CompileSource("./testdata/call.sol")
	require.NoError(t, err)

	simpleContract := contracts["SimpleContract"]
	addr := deployContract(t, simpleContract, state, 1)

	abi := solc.ExtractABI(simpleContract)
	calldata, err := abi.Pack("getValue")
	require.NoError(t, err)

	callTransaction := types.NewEmptyTransaction()
	callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction.FeeCredit = toGasCredit(100_000)
	callTransaction.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction.Data = calldata
	callTransaction.To = addr

	res := state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	require.False(t, res.Failed())
	require.Equal(t, common.LeftPadBytes(hexutil.FromHex("0x2A"), 32), res.ReturnData)

	// deploy and call Caller
	caller := contracts["Caller"]
	callerAddr := deployContract(t, caller, state, 2)
	calldata2, err := solc.ExtractABI(caller).Pack("callSet", addr, big.NewInt(43))
	require.NoError(t, err)

	callTransaction2 := types.NewEmptyTransaction()
	callTransaction2.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction2.FeeCredit = toGasCredit(10000)
	callTransaction2.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction2.Data = calldata2
	callTransaction2.To = callerAddr

	res = state.AddAndHandleTransaction(ctx, callTransaction2, dummyPayer{})
	require.False(t, res.Failed())

	// check that it changed the state of SimpleContract
	res = state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	require.False(t, res.Failed())
	require.Equal(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), res.ReturnData)

	// check that callSetAndRevert does not change anything
	calldata2, err = solc.ExtractABI(caller).Pack("callSetAndRevert", addr, big.NewInt(45))
	require.NoError(t, err)

	callTransaction2.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction2.FeeCredit = toGasCredit(10000)
	callTransaction2.Data = calldata2
	callTransaction2.To = callerAddr
	res = state.AddAndHandleTransaction(ctx, callTransaction2, dummyPayer{})
	require.ErrorIs(t, res.Error, vm.ErrExecutionReverted)

	// check that did not change the state of SimpleContract
	res = state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	require.False(t, res.Failed())
	require.Equal(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), res.ReturnData)
}

func TestDelegate(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	state := newState(t)
	defer state.tx.Rollback()

	contracts, err := solc.CompileSource("./testdata/delegate.sol")
	require.NoError(t, err)

	delegateContract := contracts["DelegateContract"]
	delegateAddr := deployContract(t, delegateContract, state, 1)

	proxyContract := contracts["ProxyContract"]
	proxyAddr := deployContract(t, proxyContract, state, 2)

	// call ProxyContract.setValue(delegateAddr, 42)
	calldata, err := solc.ExtractABI(proxyContract).Pack("setValue", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callTransaction := types.NewEmptyTransaction()
	callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction.FeeCredit = toGasCredit(30_000)
	callTransaction.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction.Data = calldata
	callTransaction.To = proxyAddr
	res := state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	require.False(t, res.Failed())

	// call ProxyContract.getValue()
	calldata, err = solc.ExtractABI(proxyContract).Pack("getValue", delegateAddr)
	require.NoError(t, err)
	callTransaction = types.NewEmptyTransaction()
	callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction.FeeCredit = toGasCredit(10_000)
	callTransaction.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction.Data = calldata
	callTransaction.To = proxyAddr
	res = state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	require.False(t, res.Failed())
	// check that it returned 42
	require.Equal(t, common.LeftPadBytes(hexutil.FromHex("0x2a"), 32), res.ReturnData)

	// call ProxyContract.setValueStatic(delegateAddr, 42)
	calldata, err = solc.ExtractABI(proxyContract).Pack("setValueStatic", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callTransaction = types.NewEmptyTransaction()
	callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction.FeeCredit = toGasCredit(10_000)
	callTransaction.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction.Data = calldata
	callTransaction.To = proxyAddr
	res = state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	require.False(t, res.Failed())
}

func TestAsyncCall(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	state := newState(t)
	defer state.tx.Rollback()

	contracts, err := solc.CompileSource(common.GetAbsolutePath("../../tests/contracts/async_call.sol"))
	require.NoError(t, err)

	smcCallee := contracts["Callee"]
	addrCallee := deployContract(t, smcCallee, state, 0)

	smcCaller := contracts["Caller"]
	addrCaller := deployContract(t, smcCaller, state, 1)

	// Call Callee::add that should increase value by 11
	abi := solc.ExtractABI(smcCaller)
	calldata, err := abi.Pack("call", addrCallee, int32(11))
	require.NoError(t, err)

	require.NoError(t, state.SetBalance(addrCaller, types.NewValueFromUint64(2_000_000_000_000_000)))

	callTransaction := types.NewEmptyTransaction()
	callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction.FeeCredit = toGasCredit(100_000)
	callTransaction.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction.Data = calldata
	callTransaction.To = addrCaller
	res := state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	txnHash := callTransaction.Hash()
	require.False(t, res.Failed())

	require.Len(t, state.OutTransactions, 1)
	require.Len(t, state.OutTransactions[txnHash], 1)

	outTxn := state.OutTransactions[txnHash][0]
	require.Equal(t, addrCaller, outTxn.From)
	require.Equal(t, addrCallee, outTxn.To)

	// Process outbound transaction, i.e. "Callee::add"
	res = state.AddAndHandleTransaction(ctx, outTxn.Transaction, dummyPayer{})
	require.False(t, res.Failed())
	require.Len(t, res.ReturnData, 32)
	require.Equal(t, types.NewUint256FromBytes(res.ReturnData), types.NewUint256(11))

	// Call Callee::add that should decrease value by 7
	calldata, err = abi.Pack("call", addrCallee, int32(-7))
	require.NoError(t, err)

	callTransaction.Data = calldata
	res = state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	txnHash = callTransaction.Hash()
	require.False(t, res.Failed())

	require.Len(t, state.OutTransactions, 2)
	require.Len(t, state.OutTransactions[txnHash], 1)

	outTxn = state.OutTransactions[txnHash][0]
	require.Equal(t, outTxn.From, addrCaller)
	require.Equal(t, outTxn.To, addrCallee)

	// Process outbound transaction, i.e. "Callee::add"
	res = state.AddAndHandleTransaction(ctx, outTxn.Transaction, dummyPayer{})
	require.False(t, res.Failed())
	require.Len(t, res.ReturnData, 32)
	require.Equal(t, types.NewUint256FromBytes(res.ReturnData), types.NewUint256(4))
}

func TestSendTransaction(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	state := newState(t)
	defer state.tx.Rollback()

	compiled, err := solc.CompileSource(common.GetAbsolutePath("../../tests/contracts/async_call.sol"))
	require.NoError(t, err)

	smcCallee := compiled["Callee"]
	addrCallee := deployContract(t, smcCallee, state, 0)

	smcCaller := compiled["Caller"]
	addrCaller := deployContract(t, smcCaller, state, 1)
	require.NoError(t, state.SetBalance(addrCaller, types.NewValueFromUint64(20_000_000)))

	// Send a transaction that calls `Callee::add`, which should increase the value by 11
	abiCalee := solc.ExtractABI(smcCallee)
	calldata, err := abiCalee.Pack("add", int32(11))
	require.NoError(t, err)

	abi := solc.ExtractABI(smcCaller)
	calldata, err = abi.Pack("asyncCall", addrCallee, types.EmptyAddress, types.EmptyAddress,
		toGasCredit(100_000), uint8(types.ForwardKindRemaining), types.Value0, calldata)
	require.NoError(t, err)

	callTransaction := types.NewEmptyTransaction()
	callTransaction.Flags = types.NewTransactionFlags(types.TransactionFlagInternal)
	callTransaction.FeeCredit = toGasCredit(100_000)
	callTransaction.MaxFeePerGas = defaultMaxFeePerGas
	callTransaction.Data = calldata
	callTransaction.To = addrCaller
	callTransaction.Seqno = 1
	res := state.AddAndHandleTransaction(ctx, callTransaction, dummyPayer{})
	tx := callTransaction.Hash()
	require.False(t, res.Failed())
	require.NotEmpty(t, state.Receipts)
	require.True(t, state.Receipts[len(state.Receipts)-1].Success)

	require.Len(t, state.OutTransactions, 1)
	require.Len(t, state.OutTransactions[tx], 1)

	outTxn := state.OutTransactions[tx][0]
	require.Equal(t, addrCaller, outTxn.From)
	require.Equal(t, addrCallee, outTxn.To)
	require.Less(t, uint64(99999), outTxn.FeeCredit.Uint64())

	// Process outbound transaction, i.e. "Callee::add"
	res = state.AddAndHandleTransaction(ctx, outTxn.Transaction, dummyPayer{})
	require.False(t, res.Failed())
	lastReceipt := state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, res.ReturnData, 32)
	require.Equal(t, types.NewUint256FromBytes(res.ReturnData), types.NewUint256(11))
}
