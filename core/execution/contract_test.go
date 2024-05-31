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
		Data:  data,
		Seqno: uint64(seqno),
	}
	addr := types.CreateAddress(state.ShardId, message.From, message.Seqno)

	return addr, state.HandleDeployMessage(message, 0, blockContext)
}

func TestCall(t *testing.T) {
	t.Parallel()
	state := newState(t)

	contracts, _ := solc.CompileSource("./testdata/contracts.sol")

	blockContext := NewEVMBlockContext(state)

	simpleContract := contracts["SimpleContract"]
	addr, err := deployContract(simpleContract, state, &blockContext, 1)
	require.NoError(t, err)

	abi := solc.ExtractABI(simpleContract)
	calldata, err := abi.Pack("getValue")
	require.NoError(t, err)

	callMessage := &types.Message{
		Data: calldata,
		To:   addr,
	}
	ret, err := state.HandleExecutionMessage(callMessage, 2, &blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2A"), 32), ret)

	// deploy and call Caller
	caller := contracts["Caller"]
	callerAddr, err := deployContract(caller, state, &blockContext, 2)
	require.NoError(t, err)
	calldata2, err := solc.ExtractABI(caller).Pack("callSet", addr, big.NewInt(43))
	require.NoError(t, err)

	callMessage2 := &types.Message{
		Data: calldata2,
		To:   callerAddr,
	}
	_, err = state.HandleExecutionMessage(callMessage2, 3, &blockContext)
	require.NoError(t, err)

	// check that it changed the state of SimpleContract
	ret, err = state.HandleExecutionMessage(callMessage, 2, &blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), ret)

	// check that callSetAndRevert does not change anything
	calldata2, err = solc.ExtractABI(caller).Pack("callSetAndRevert", addr, big.NewInt(45))
	require.NoError(t, err)

	callMessage2 = &types.Message{
		Data: calldata2,
		To:   callerAddr,
	}
	_, err = state.HandleExecutionMessage(callMessage2, 3, &blockContext)
	require.NoError(t, err)

	// check that did not change the state of SimpleContract
	ret, err = state.HandleExecutionMessage(callMessage, 2, &blockContext)
	require.NoError(t, err)
	require.EqualValues(t, common.LeftPadBytes(hexutil.FromHex("0x2b"), 32), ret)
}
