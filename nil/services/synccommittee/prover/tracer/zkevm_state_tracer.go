package tracer

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type ZKEVMState struct {
	TxHash          common.Hash
	TxId            int // Index of transaction in block
	PC              uint64
	Gas             uint64
	RwIdx           uint
	BytecodeHash    common.Hash
	OpCode          vm.OpCode
	AdditionalInput types.Uint256
	StackSize       uint64
	MemorySize      uint64
	TxFinish        bool
	Err             error

	StackSlice   []types.Uint256
	MemorySlice  map[uint64]uint8
	StorageSlice map[types.Uint256]types.Uint256
}

type ZKEVMStateTracer struct {
	rwCtr        *RwCounter
	txHash       common.Hash
	bytecodeHash common.Hash
	msgId        uint
	res          []ZKEVMState
}

func (zst *ZKEVMStateTracer) Finalize() []ZKEVMState {
	state_num := len(zst.res)
	// Mark last state of transaction
	if state_num != 0 {
		zst.res[state_num-1].TxFinish = true
	}
	return zst.res
}

func (zst *ZKEVMStateTracer) TraceOp(
	opCode vm.OpCode,
	pc uint64,
	gas uint64,
	stackToSave int,
	additionalInput *types.Uint256,
	memRanges opRanges,
	scope tracing.OpContext,
) bool {
	state := ZKEVMState{
		TxHash:          zst.txHash,
		TxId:            int(zst.msgId),
		PC:              pc,
		Gas:             gas,
		RwIdx:           zst.rwCtr.ctr,
		BytecodeHash:    zst.bytecodeHash,
		OpCode:          opCode,
		AdditionalInput: *additionalInput,
		StackSize:       uint64(len(scope.StackData())),
		MemorySize:      uint64(len(scope.MemoryData())),
		TxFinish:        false,
		Err:             nil,
		StackSlice:      make([]types.Uint256, stackToSave),
		MemorySlice:     make(map[uint64]uint8),
		StorageSlice:    make(map[types.Uint256]types.Uint256),
	}

	// Copy last stackToSave stack values
	stackSize := len(scope.StackData())

	for i := range stackToSave {
		state.StackSlice[i] = types.Uint256(scope.StackData()[stackSize-stackToSave+i])
	}

	// Copy memory from ranges
	for i := memRanges.before.offset; i < memRanges.before.offset+memRanges.before.length; i++ {
		var databyte byte
		if i < uint64(len(scope.MemoryData())) { // see memory tracer for details
			databyte = scope.MemoryData()[i]
		}
		state.MemorySlice[i] = databyte
	}
	for i := memRanges.after.offset; i < memRanges.after.offset+memRanges.after.length; i++ {
		if i >= state.MemorySize {
			// Memory not yet initialized, skipping
			break
		}
		state.MemorySlice[i] = scope.MemoryData()[i]
	}
	zst.res = append(zst.res, state)

	return true
}

func (zst *ZKEVMStateTracer) SetLastStateStorage(key, value types.Uint256) {
	stateNum := len(zst.res)
	if stateNum == 0 {
		panic("State must be already added before storage operation")
	}
	lastRes := &zst.res[stateNum-1]
	lastRes.StorageSlice[key] = value
}

func (zst *ZKEVMStateTracer) FinishPrevOpcodeTracing() {
}
