package execution

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/stretchr/testify/require"
)

func deployContract(contract *compiler.Contract, state *ExecutionState, blockContext *vm.BlockContext, seqno int) (types.Address, error) {
	contractCode := hexutil.FromHex(contract.Code)
	dm := &types.DeployMessage{
		ShardId: state.ShardId,
		Code:    contractCode,
	}
	data, _ := dm.MarshalSSZ()
	message := &types.Message{
		Data:     data,
		Seqno:    uint64(seqno),
		GasLimit: *types.NewUint256(100000),
		To:       types.DeployMsgToAddress(dm.ShardId, data),
	}
	return message.To, state.HandleDeployMessage(message, dm, blockContext)
}

func TestCall(t *testing.T) {
	t.Parallel()
	state := newState(t)

	contracts, err := solc.CompileSource("./testdata/call.sol")
	require.NoError(t, err)

	blockContext := NewEVMBlockContext(state)

	simpleContract := contracts["SimpleContract"]
	addr, err := deployContract(simpleContract, state, &blockContext, 1)
	require.NoError(t, err)

	abi := solc.ExtractABI(simpleContract)
	calldata, err := abi.Pack("getValue")
	require.NoError(t, err)

	callMessage := &types.Message{
		Data:     calldata,
		To:       addr,
		GasLimit: *types.NewUint256(10000),
	}
	ret, err := state.HandleExecutionMessage(callMessage, &blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2A"), 32), ret)

	// deploy and call Caller
	caller := contracts["Caller"]
	callerAddr, err := deployContract(caller, state, &blockContext, 2)
	require.NoError(t, err)
	calldata2, err := solc.ExtractABI(caller).Pack("callSet", addr, big.NewInt(43))
	require.NoError(t, err)

	callMessage2 := &types.Message{
		Data:     calldata2,
		To:       callerAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, err = state.HandleExecutionMessage(callMessage2, &blockContext)
	require.NoError(t, err)

	// check that it changed the state of SimpleContract
	ret, err = state.HandleExecutionMessage(callMessage, &blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), ret)

	// check that callSetAndRevert does not change anything
	calldata2, err = solc.ExtractABI(caller).Pack("callSetAndRevert", addr, big.NewInt(45))
	require.NoError(t, err)

	callMessage2 = &types.Message{
		Data:     calldata2,
		To:       callerAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, err = state.HandleExecutionMessage(callMessage2, &blockContext)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)

	// check that did not change the state of SimpleContract
	ret, err = state.HandleExecutionMessage(callMessage, &blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), ret)
}

func TestDelegate(t *testing.T) {
	t.Parallel()
	state := newState(t)

	contracts, err := solc.CompileSource("./testdata/delegate.sol")
	require.NoError(t, err)

	blockContext := NewEVMBlockContext(state)

	delegateContract := contracts["DelegateContract"]
	delegateAddr, err := deployContract(delegateContract, state, &blockContext, 1)
	require.NoError(t, err)

	proxyContract := contracts["ProxyContract"]
	proxyAddr, err := deployContract(proxyContract, state, &blockContext, 2)
	require.NoError(t, err)

	// call ProxyContract.setValue(delegateAddr, 42)
	calldata, err := solc.ExtractABI(proxyContract).Pack("setValue", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callMessage := &types.Message{
		Data:     calldata,
		To:       proxyAddr,
		GasLimit: *types.NewUint256(100000),
	}
	_, err = state.HandleExecutionMessage(callMessage, &blockContext)
	require.NoError(t, err)

	// call ProxyContract.getValue()
	calldata, err = solc.ExtractABI(proxyContract).Pack("getValue", delegateAddr)
	require.NoError(t, err)
	callMessage = &types.Message{
		Data:     calldata,
		To:       proxyAddr,
		GasLimit: *types.NewUint256(10000),
	}
	ret, err := state.HandleExecutionMessage(callMessage, &blockContext)
	require.NoError(t, err)
	// check that it returned 42
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2a"), 32), ret)

	// call ProxyContract.setValueStatic(delegateAddr, 42)
	calldata, err = solc.ExtractABI(proxyContract).Pack("setValueStatic", delegateAddr, big.NewInt(42))
	require.NoError(t, err)
	callMessage = &types.Message{
		Data:     calldata,
		To:       proxyAddr,
		GasLimit: *types.NewUint256(10000),
	}
	_, err = state.HandleExecutionMessage(callMessage, &blockContext)
	require.ErrorAs(t, err, new(vm.VMError))
}

func TestAsyncCall(t *testing.T) {
	t.Parallel()
	state := newState(t)

	state.TraceVm = false

	contracts, err := solc.CompileSource(common.GetAbsolutePath("../../tests/rpc_server/contracts/async_call.sol"))
	require.NoError(t, err)

	blockContext := NewEVMBlockContext(state)

	smcCallee := contracts["Callee"]
	addrCallee, err := deployContract(smcCallee, state, &blockContext, 0)
	require.NoError(t, err)

	smcCaller := contracts["Caller"]
	addrCaller, err := deployContract(smcCaller, state, &blockContext, 1)
	require.NoError(t, err)

	// Call Callee::add that should increase value by 11
	abi := solc.ExtractABI(smcCaller)
	calldata, err := abi.Pack("call", addrCallee, int32(11))
	require.NoError(t, err)

	callMessage := &types.Message{
		Data:     calldata,
		To:       addrCaller,
		GasLimit: *types.NewUint256(100000),
	}
	_, err = state.HandleExecutionMessage(callMessage, &blockContext)
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
	ret, err := state.HandleExecutionMessage(outMsg, &blockContext)
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
		Data:     calldata,
		To:       addrCaller,
		GasLimit: *types.NewUint256(10000),
	}
	_, err = state.HandleExecutionMessage(callMessage, &blockContext)
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
	ret, err = state.HandleExecutionMessage(outMsg, &blockContext)
	require.NoError(t, err)
	lastReceipt = state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, ret, 32)
	res.SetBytes(ret)
	require.Equal(t, res, *types.NewUint256(4))
}

func TestSendMessage(t *testing.T) {
	t.Parallel()
	state := newState(t)

	state.TraceVm = false

	contracts, err := solc.CompileSource(common.GetAbsolutePath("../../tests/rpc_server/contracts/async_call.sol"))
	require.NoError(t, err)

	blockContext := NewEVMBlockContext(state)

	smcCallee := contracts["Callee"]
	addrCallee, err := deployContract(smcCallee, state, &blockContext, 0)
	require.NoError(t, err)

	smcCaller := contracts["Caller"]
	addrCaller, err := deployContract(smcCaller, state, &blockContext, 1)
	require.NoError(t, err)

	// Send message that calls `Callee::add`, which should increase value by 11
	abiCalee := solc.ExtractABI(smcCallee)
	calldata, err := abiCalee.Pack("add", int32(11))
	require.NoError(t, err)
	messageToSend := &types.Message{
		Data:     calldata,
		From:     addrCaller,
		To:       addrCallee,
		Value:    *types.NewUint256(0),
		Internal: true,
		GasLimit: *types.NewUint256(100000),
	}
	calldata, err = messageToSend.MarshalSSZ()
	require.NoError(t, err)

	abi := solc.ExtractABI(smcCaller)
	calldata, err = abi.Pack("send_msg", calldata)
	require.NoError(t, err)

	callMessage := &types.Message{
		Data:     calldata,
		To:       addrCaller,
		GasLimit: *types.NewUint256(100000),
	}
	_, err = state.HandleExecutionMessage(callMessage, &blockContext)
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
	ret, err := state.HandleExecutionMessage(outMsg, &blockContext)
	require.NoError(t, err)
	lastReceipt := state.Receipts[len(state.Receipts)-1]
	require.True(t, lastReceipt.Success)
	require.Len(t, ret, 32)
	var res types.Uint256
	res.SetBytes(ret)
	require.Equal(t, res, *types.NewUint256(11))
}
