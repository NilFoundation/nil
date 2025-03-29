package tracer

import (
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
)

type ExecutionTraces struct {
	// Stack/Memory/State Ops are handled for entire block, they share the same counter (rw_circuit)
	StackOps     []StackOp
	MemoryOps    []MemoryOp
	StorageOps   []StorageOp
	ExpOps       []ExpOp
	ZKEVMStates  []ZKEVMState
	CopyEvents   []CopyEvent
	KeccakTraces []KeccakBuffer
	MPTTraces    *mpttracer.MPTTraces

	ContractsBytecode map[types.Address][]byte
}

func NewExecutionTraces() *ExecutionTraces {
	return &ExecutionTraces{
		ContractsBytecode: make(map[types.Address][]byte),
	}
}

func (tr *ExecutionTraces) AddMemoryOps(ops []MemoryOp) {
	tr.MemoryOps = append(tr.MemoryOps, ops...)
}

func (tr *ExecutionTraces) AddStackOps(ops []StackOp) {
	tr.StackOps = append(tr.StackOps, ops...)
}

func (tr *ExecutionTraces) AddStorageOps(ops []StorageOp) {
	tr.StorageOps = append(tr.StorageOps, ops...)
}

func (tr *ExecutionTraces) AddZKEVMStates(states []ZKEVMState) {
	tr.ZKEVMStates = append(tr.ZKEVMStates, states...)
}

func (tr *ExecutionTraces) AddCopyEvents(events []CopyEvent) {
	tr.CopyEvents = append(tr.CopyEvents, events...)
}

func (tr *ExecutionTraces) AddContractBytecode(addr types.Address, code []byte) {
	tr.ContractsBytecode[addr] = code
}

func (tr *ExecutionTraces) AddExpOps(ops []ExpOp) {
	tr.ExpOps = append(tr.ExpOps, ops...)
}

func (tr *ExecutionTraces) AddKeccakOps(ops []KeccakBuffer) {
	tr.KeccakTraces = append(tr.KeccakTraces, ops...)
}

func (tr *ExecutionTraces) SetMptTraces(mptTraces *mpttracer.MPTTraces) {
	tr.MPTTraces = mptTraces
}

// Append adds `other` to the end of traces slices, adds kv pairs from `otherTrace` maps
func (tr *ExecutionTraces) Append(other *ExecutionTraces) {
	if tr.MPTTraces != nil {
		panic("you should not merge MPT traces, call `SetMptTraces` once at the end")
	}
	tr.MPTTraces = other.MPTTraces

	tr.MemoryOps = append(tr.MemoryOps, other.MemoryOps...)
	tr.StackOps = append(tr.StackOps, other.StackOps...)
	tr.StorageOps = append(tr.StorageOps, other.StorageOps...)
	tr.ExpOps = append(tr.ExpOps, other.ExpOps...)
	tr.ZKEVMStates = append(tr.ZKEVMStates, other.ZKEVMStates...)
	tr.CopyEvents = append(tr.CopyEvents, other.CopyEvents...)
	tr.KeccakTraces = append(tr.KeccakTraces, other.KeccakTraces...)

	for addr, code := range other.ContractsBytecode {
		tr.ContractsBytecode[addr] = code
	}
}

type Stats struct {
	ProcessedInTxnsN   uint
	OpsN               uint // should be the same as StackOpsN, since every op is a stack op
	StackOpsN          uint
	MemoryOpsN         uint
	StateOpsN          uint
	CopyOpsN           uint
	ExpOpsN            uint
	KeccakOpsN         uint
	AffectedContractsN uint
}
