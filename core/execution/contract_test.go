package execution

import (
	"context"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func deployContract(t *testing.T, contract *compiler.Contract, state *ExecutionState, blockContext *vm.BlockContext, seqno types.Seqno) types.Address {
	t.Helper()

	contractCode := hexutil.FromHex(contract.Code)
	dm := types.BuildDeployPayload(contractCode, common.EmptyHash)
	message := &types.Message{
		Internal: true,
		Data:     dm.Bytes(),
		Seqno:    seqno,
		GasLimit: *types.NewUint256(100000),
		To:       types.CreateAddress(state.ShardId, contractCode),
	}
	_, err := state.HandleDeployMessage(context.Background(), message, &dm, blockContext)
	require.NoError(t, err)
	return message.To
}

func TestCall(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := newState(t)

	contracts, err := solc.CompileSource("./testdata/call.sol")
	require.NoError(t, err)

	blockContext, err := NewEVMBlockContext(state)
	require.NoError(t, err)

	simpleContract := contracts["SimpleContract"]
	addr := deployContract(t, simpleContract, state, blockContext, 1)

	abi := solc.ExtractABI(simpleContract)
	calldata, err := abi.Pack("getValue")
	require.NoError(t, err)

	callMessage := &types.Message{
		Internal: true,
		Data:     calldata,
		To:       addr,
		GasLimit: *types.NewUint256(10000),
	}
	_, ret, err := state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2A"), 32), ret)

	// deploy and call Caller
	caller := contracts["Caller"]
	callerAddr := deployContract(t, caller, state, blockContext, 2)
	calldata2, err := solc.ExtractABI(caller).Pack("callSet", addr, big.NewInt(43))
	require.NoError(t, err)

	callMessage2 := &types.Message{
		Internal: true,
		Data:     calldata2,
		To:       callerAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, _, err = state.HandleExecutionMessage(ctx, callMessage2, blockContext)
	require.NoError(t, err)

	// check that it changed the state of SimpleContract
	_, ret, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), ret)

	// check that callSetAndRevert does not change anything
	calldata2, err = solc.ExtractABI(caller).Pack("callSetAndRevert", addr, big.NewInt(45))
	require.NoError(t, err)

	callMessage2 = &types.Message{
		Internal: true,
		Data:     calldata2,
		To:       callerAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, _, err = state.HandleExecutionMessage(ctx, callMessage2, blockContext)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)

	// check that did not change the state of SimpleContract
	_, ret, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), ret)
}

func TestDelegate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := newState(t)

	contracts, err := solc.CompileSource("./testdata/delegate.sol")
	require.NoError(t, err)

	blockContext, err := NewEVMBlockContext(state)
	require.NoError(t, err)

	delegateContract := contracts["DelegateContract"]
	delegateAddr := deployContract(t, delegateContract, state, blockContext, 1)

	proxyContract := contracts["ProxyContract"]
	proxyAddr := deployContract(t, proxyContract, state, blockContext, 2)

	// call ProxyContract.setValue(delegateAddr, 42)
	calldata, err := solc.ExtractABI(proxyContract).Pack("setValue", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callMessage := &types.Message{
		Internal: true,
		Data:     calldata,
		To:       proxyAddr,
		GasLimit: *types.NewUint256(100000),
	}
	_, _, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)

	// call ProxyContract.getValue()
	calldata, err = solc.ExtractABI(proxyContract).Pack("getValue", delegateAddr)
	require.NoError(t, err)
	callMessage = &types.Message{
		Internal: true,
		Data:     calldata,
		To:       proxyAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, ret, err := state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	// check that it returned 42
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2a"), 32), ret)

	// call ProxyContract.setValueStatic(delegateAddr, 42)
	calldata, err = solc.ExtractABI(proxyContract).Pack("setValueStatic", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callMessage = &types.Message{
		Internal: true,
		Data:     calldata,
		To:       proxyAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, _, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.ErrorAs(t, err, new(vm.VMError))
}

func TestAsyncCall(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := newState(t)

	state.TraceVm = false

	contracts, err := solc.CompileSource(common.GetAbsolutePath("../../tests/rpc_server/contracts/async_call.sol"))
	require.NoError(t, err)

	blockContext, err := NewEVMBlockContext(state)
	require.NoError(t, err)

	smcCallee := contracts["Callee"]
	addrCallee := deployContract(t, smcCallee, state, blockContext, 0)

	smcCaller := contracts["Caller"]
	addrCaller := deployContract(t, smcCaller, state, blockContext, 1)

	// Call Callee::add that should increase value by 11
	abi := solc.ExtractABI(smcCaller)
	calldata, err := abi.Pack("call", addrCallee, int32(11))
	require.NoError(t, err)

	state.SetBalance(addrCaller, *uint256.NewInt(1_000_000))

	callMessage := &types.Message{
		Internal: true,
		Data:     calldata,
		To:       addrCaller,
		GasLimit: *types.NewUint256(100_000),
	}
	state.AddInMessage(callMessage)
	_, _, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	require.NotEmpty(t, state.Receipts)
	require.True(t, state.Receipts[len(state.Receipts)-1].Success)
	tx := state.Receipts[len(state.Receipts)-1].MsgHash

	require.Len(t, state.OutMessages, 1)
	require.Len(t, state.OutMessages[tx], 1)

	outMsg := state.OutMessages[tx][0]
	require.Equal(t, addrCaller, outMsg.From)
	require.Equal(t, addrCallee, outMsg.To)

	// Process outbound message, i.e. "Callee::add"
	_, ret, err := state.HandleExecutionMessage(ctx, outMsg, blockContext)
	require.NoError(t, err)
	lastReceipt := state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, ret, 32)
	var res types.Uint256
	res.SetBytes(ret)
	require.Equal(t, res, *types.NewUint256(11))

	// Call Callee::add that should decrease value by 7
	calldata, err = abi.Pack("call", addrCallee, int32(-7))
	require.NoError(t, err)
	callMessage = &types.Message{
		Internal: true,
		Data:     calldata,
		To:       addrCaller,
		GasLimit: *types.NewUint256(10000),
	}
	_, _, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	require.NotEmpty(t, state.Receipts)
	require.True(t, state.Receipts[len(state.Receipts)-1].Success)
	tx = state.Receipts[len(state.Receipts)-1].MsgHash

	require.Len(t, state.OutMessages, 1)
	require.Len(t, state.OutMessages[tx], 2)

	outMsg = state.OutMessages[tx][1]
	require.Equal(t, outMsg.From, addrCaller)
	require.Equal(t, outMsg.To, addrCallee)

	require.Len(t, state.OutMessages, 1)
	require.Len(t, state.OutMessages[tx], 2)

	outMsg = state.OutMessages[tx][1]
	require.Equal(t, outMsg.From, addrCaller)
	require.Equal(t, outMsg.To, addrCallee)

	// Process outbound message, i.e. "Callee::add"
	_, ret, err = state.HandleExecutionMessage(ctx, outMsg, blockContext)
	require.NoError(t, err)
	lastReceipt = state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, ret, 32)
	res.SetBytes(ret)
	require.Equal(t, res, *types.NewUint256(4))
}

func TestSendMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := newState(t)

	state.TraceVm = false

	contracts, err := solc.CompileSource(common.GetAbsolutePath("../../tests/rpc_server/contracts/async_call.sol"))
	require.NoError(t, err)

	blockContext, err := NewEVMBlockContext(state)
	require.NoError(t, err)

	smcCallee := contracts["Callee"]
	addrCallee := deployContract(t, smcCallee, state, blockContext, 0)

	smcCaller := contracts["Caller"]
	addrCaller := deployContract(t, smcCaller, state, blockContext, 1)

	// Send a message that calls `Callee::add`, which should increase the value by 11
	abiCalee := solc.ExtractABI(smcCallee)
	calldata, err := abiCalee.Pack("add", int32(11))
	require.NoError(t, err)
	messageToSend := &types.InternalMessagePayload{
		Data:     calldata,
		To:       addrCallee,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
	}
	calldata, err = messageToSend.MarshalSSZ()
	require.NoError(t, err)

	abi := solc.ExtractABI(smcCaller)
	calldata, err = abi.Pack("sendMessage", calldata)
	require.NoError(t, err)

	callMessage := &types.Message{
		Internal: true,
		Data:     calldata,
		To:       addrCaller,
		GasLimit: *types.NewUint256(100000),
	}
	_, _, err = state.HandleExecutionMessage(ctx, callMessage, blockContext)
	require.NoError(t, err)
	require.NotEmpty(t, state.Receipts)
	require.True(t, state.Receipts[len(state.Receipts)-1].Success)
	tx := state.Receipts[len(state.Receipts)-1].MsgHash

	require.Len(t, state.OutMessages, 1)
	require.Len(t, state.OutMessages[tx], 1)

	outMsg := state.OutMessages[tx][0]
	require.Equal(t, addrCaller, outMsg.From)
	require.Equal(t, addrCallee, outMsg.To)
	require.True(t, outMsg.GasLimit.GtUint64(99999))

	// Process outbound message, i.e. "Callee::add"
	_, ret, err := state.HandleExecutionMessage(ctx, outMsg, blockContext)
	require.NoError(t, err)
	lastReceipt := state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, ret, 32)
	var res types.Uint256
	res.SetBytes(ret)
	require.Equal(t, res, *types.NewUint256(11))
}
