package tracer

import (
	"errors"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
)

type ExecutionTraces interface {
	AddMemoryOps(ops []MemoryOp)
	AddStackOps(ops []StackOp)
	AddStorageOps(ops []StorageOp)
	AddExpOps(ops []ExpOp)
	AddKeccakOps(ops []KeccakBuffer)
	AddZKEVMStates(states []ZKEVMState)
	AddCopyEvents(events []CopyEvent)
	AddContractBytecode(addr types.Address, code []byte)
	SetMptTraces(mptTraces *mpttracer.MPTTraces)
	Append(otherTrace ExecutionTraces) error
}

type executionTracesImpl struct {
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

var _ ExecutionTraces = new(executionTracesImpl)

func NewExecutionTraces() ExecutionTraces {
	return &executionTracesImpl{
		ContractsBytecode: make(map[types.Address][]byte),
	}
}

func (tr *executionTracesImpl) AddMemoryOps(ops []MemoryOp) {
	tr.MemoryOps = append(tr.MemoryOps, ops...)
}

func (tr *executionTracesImpl) AddStackOps(ops []StackOp) {
	tr.StackOps = append(tr.StackOps, ops...)
}

func (tr *executionTracesImpl) AddStorageOps(ops []StorageOp) {
	tr.StorageOps = append(tr.StorageOps, ops...)
}

func (tr *executionTracesImpl) AddZKEVMStates(states []ZKEVMState) {
	tr.ZKEVMStates = append(tr.ZKEVMStates, states...)
}

func (tr *executionTracesImpl) AddCopyEvents(events []CopyEvent) {
	tr.CopyEvents = append(tr.CopyEvents, events...)
}

func (tr *executionTracesImpl) AddContractBytecode(addr types.Address, code []byte) {
	tr.ContractsBytecode[addr] = code
}

func (tr *executionTracesImpl) AddExpOps(ops []ExpOp) {
	tr.ExpOps = append(tr.ExpOps, ops...)
}

func (tr *executionTracesImpl) AddKeccakOps(ops []KeccakBuffer) {
	tr.KeccakTraces = append(tr.KeccakTraces, ops...)
}

func (tr *executionTracesImpl) SetMptTraces(mptTraces *mpttracer.MPTTraces) {
	tr.MPTTraces = mptTraces
}

// Append adds `otherTrace` to the end of traces slices, adds kv pairs from `otherTrace` maps
func (tr *executionTracesImpl) Append(otherTrace ExecutionTraces) error {
	other, ok := otherTrace.(*executionTracesImpl)
	if !ok {
		return errors.New("invalid trace type")
	}

	// TODO: implement mpt traces merging
	if tr.MPTTraces != nil {
		panic("MPT traces merging is not implemented")
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

	return nil
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
