package execution

import (
	"context"
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

func deployContract(t *testing.T, contract *compiler.Contract, state *ExecutionState, seqno types.Seqno) types.Address {
	t.Helper()

	return Deploy(t, context.Background(), state,
		types.BuildDeployPayload(hexutil.FromHex(contract.Code), common.EmptyHash),
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

			require.NoError(t, state.newVm(true, address, nil))
			_, _, _ = state.evm.Call(vm.AccountRef(address), address, nil, 100000, new(uint256.Int))
		}
		_, _, err := state.Commit(types.BlockNumber(i))
		require.NoError(t, err)
	}
	for i := range 50 {
		check(i)
	}
}

func TestPrecompiles(t *testing.T) {
	t.Parallel()

	// Test checks that precompiles are not crashed
	// if called with an empty input data
	check := func(i int) {
		state := newState(t)
		defer state.tx.Rollback()
		require.NoError(t, state.newVm(true, types.EmptyAddress, nil))

		callMessage := types.NewEmptyMessage()
		callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
		callMessage.FeeCredit = toGasCredit(100_000)
		state.AddInMessage(callMessage)

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

	ctx := context.Background()
	state := newState(t)
	defer state.tx.Rollback()

	contracts, err := solc.CompileSource("./testdata/call.sol")
	require.NoError(t, err)

	simpleContract := contracts["SimpleContract"]
	addr := deployContract(t, simpleContract, state, 1)

	abi := solc.ExtractABI(simpleContract)
	calldata, err := abi.Pack("getValue")
	require.NoError(t, err)

	callMessage := types.NewEmptyMessage()
	callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage.FeeCredit = toGasCredit(100_000)
	callMessage.Data = calldata
	callMessage.To = addr

	res := state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2A"), 32), res.ReturnData)

	// deploy and call Caller
	caller := contracts["Caller"]
	callerAddr := deployContract(t, caller, state, 2)
	calldata2, err := solc.ExtractABI(caller).Pack("callSet", addr, big.NewInt(43))
	require.NoError(t, err)

	callMessage2 := types.NewEmptyMessage()
	callMessage2.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage2.FeeCredit = toGasCredit(10000)
	callMessage2.Data = calldata2
	callMessage2.To = callerAddr

	res = state.HandleExecutionMessage(ctx, callMessage2)
	require.False(t, res.Failed())

	// check that it changed the state of SimpleContract
	res = state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), res.ReturnData)

	// check that callSetAndRevert does not change anything
	calldata2, err = solc.ExtractABI(caller).Pack("callSetAndRevert", addr, big.NewInt(45))
	require.NoError(t, err)

	callMessage2.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage2.FeeCredit = toGasCredit(10000)
	callMessage2.Data = calldata2
	callMessage2.To = callerAddr
	res = state.HandleExecutionMessage(ctx, callMessage2)
	require.ErrorIs(t, res.Error, vm.ErrExecutionReverted)

	// check that did not change the state of SimpleContract
	res = state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), res.ReturnData)
}

func TestDelegate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
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
	callMessage := types.NewEmptyMessage()
	callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage.FeeCredit = toGasCredit(30_000)
	callMessage.Data = calldata
	callMessage.To = proxyAddr
	res := state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())

	// call ProxyContract.getValue()
	calldata, err = solc.ExtractABI(proxyContract).Pack("getValue", delegateAddr)
	require.NoError(t, err)
	callMessage = types.NewEmptyMessage()
	callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage.FeeCredit = toGasCredit(10_000)
	callMessage.Data = calldata
	callMessage.To = proxyAddr
	res = state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())
	// check that it returned 42
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2a"), 32), res.ReturnData)

	// call ProxyContract.setValueStatic(delegateAddr, 42)
	calldata, err = solc.ExtractABI(proxyContract).Pack("setValueStatic", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callMessage = types.NewEmptyMessage()
	callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage.FeeCredit = toGasCredit(10_000)
	callMessage.Data = calldata
	callMessage.To = proxyAddr
	res = state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())
}

func TestAsyncCall(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
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

	require.NoError(t, state.SetBalance(addrCaller, types.NewValueFromUint64(20_000_000)))

	callMessage := types.NewEmptyMessage()
	callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage.FeeCredit = toGasCredit(100_000)
	callMessage.Data = calldata
	callMessage.To = addrCaller
	msgHash := callMessage.Hash()
	state.AddInMessage(callMessage)
	res := state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())

	require.Len(t, state.OutMessages, 1)
	require.Len(t, state.OutMessages[msgHash], 1)

	outMsg := state.OutMessages[msgHash][0]
	require.Equal(t, addrCaller, outMsg.From)
	require.Equal(t, addrCallee, outMsg.To)

	// Process outbound message, i.e. "Callee::add"
	state.AddInMessage(outMsg.Message)
	res = state.HandleExecutionMessage(ctx, outMsg.Message)
	require.False(t, res.Failed())
	require.Len(t, res.ReturnData, 32)
	require.Equal(t, types.NewUint256FromBytes(res.ReturnData), types.NewUint256(11))

	// Call Callee::add that should decrease value by 7
	calldata, err = abi.Pack("call", addrCallee, int32(-7))
	require.NoError(t, err)

	callMessage.Data = calldata
	msgHash = callMessage.Hash()
	state.AddInMessage(callMessage)
	res = state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())

	require.Len(t, state.OutMessages, 2)
	require.Len(t, state.OutMessages[msgHash], 1)

	outMsg = state.OutMessages[msgHash][0]
	require.Equal(t, outMsg.From, addrCaller)
	require.Equal(t, outMsg.To, addrCallee)

	// Process outbound message, i.e. "Callee::add"
	res = state.HandleExecutionMessage(ctx, outMsg.Message)
	require.False(t, res.Failed())
	require.Len(t, res.ReturnData, 32)
	require.Equal(t, types.NewUint256FromBytes(res.ReturnData), types.NewUint256(4))
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := newState(t)
	defer state.tx.Rollback()

	state.TraceVm = false

	contracts, err := solc.CompileSource(common.GetAbsolutePath("../../tests/contracts/async_call.sol"))
	require.NoError(t, err)

	smcCallee := contracts["Callee"]
	addrCallee := deployContract(t, smcCallee, state, 0)

	smcCaller := contracts["Caller"]
	addrCaller := deployContract(t, smcCaller, state, 1)
	require.NoError(t, state.SetBalance(addrCaller, types.NewValueFromUint64(20_000_000)))

	// Send a message that calls `Callee::add`, which should increase the value by 11
	abiCalee := solc.ExtractABI(smcCallee)
	calldata, err := abiCalee.Pack("add", int32(11))
	require.NoError(t, err)
	messageToSend := &types.InternalMessagePayload{
		Data:      calldata,
		To:        addrCallee,
		FeeCredit: toGasCredit(100000),
	}
	calldata, err = messageToSend.MarshalSSZ()
	require.NoError(t, err)

	abi := solc.ExtractABI(smcCaller)
	calldata, err = abi.Pack("sendMessage", calldata)
	require.NoError(t, err)

	callMessage := types.NewEmptyMessage()
	callMessage.Flags = types.NewMessageFlags(types.MessageFlagInternal)
	callMessage.FeeCredit = toGasCredit(100_000)
	callMessage.Data = calldata
	callMessage.To = addrCaller
	res := state.HandleExecutionMessage(ctx, callMessage)
	require.False(t, res.Failed())
	require.NotEmpty(t, state.Receipts)
	require.True(t, state.Receipts[len(state.Receipts)-1].Success)
	tx := state.Receipts[len(state.Receipts)-1].MsgHash

	require.Len(t, state.OutMessages, 1)
	require.Len(t, state.OutMessages[tx], 1)

	outMsg := state.OutMessages[tx][0]
	require.Equal(t, addrCaller, outMsg.From)
	require.Equal(t, addrCallee, outMsg.To)
	require.Less(t, uint64(99999), outMsg.FeeCredit.Uint64())

	// Process outbound message, i.e. "Callee::add"
	res = state.HandleExecutionMessage(ctx, outMsg.Message)
	require.False(t, res.Failed())
	lastReceipt := state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, res.ReturnData, 32)
	require.Equal(t, types.NewUint256FromBytes(res.ReturnData), types.NewUint256(11))
}
