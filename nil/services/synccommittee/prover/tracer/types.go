package tracer

import (
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
)

type ExecutionTraces interface {
	AddMemoryOps(ops []MemoryOp)
	AddStackOps(ops []StackOp)
	AddStorageOps(ops []StorageOp)
	AddExpOps(ops []ExpOp)
	AddZKEVMStates(states []ZKEVMState)
	AddCopyEvents(events []CopyEvent)
	AddContractBytecode(addr types.Address, code []byte)
	SetMptTraces(mptTraces *mpttracer.MPTTraces)
}

type executionTracesImpl struct {
	// Stack/Memory/State Ops are handled for entire block, they share the same counter (rw_circuit)
	StackOps    []StackOp
	MemoryOps   []MemoryOp
	StorageOps  []StorageOp
	ExpOps      []ExpOp
	ZKEVMStates []ZKEVMState
	CopyEvents  []CopyEvent
	MPTTraces   *mpttracer.MPTTraces

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

func (tr *executionTracesImpl) SetMptTraces(mptTraces *mpttracer.MPTTraces) {
	tr.MPTTraces = mptTraces
}

type Stats struct {
	ProcessedInMsgsN   uint
	OpsN               uint // should be the same as StackOpsN, since every op is a stack op
	StackOpsN          uint
	MemoryOpsN         uint
	StateOpsN          uint
	CopyOpsN           uint
	ExpOpsN            uint
	AffectedContractsN uint
}
