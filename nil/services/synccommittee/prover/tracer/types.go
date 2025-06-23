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
	ZethCache    *mpttracer.FileProviderCache

	ContractsBytecode map[types.Address][]byte
}

func NewExecutionTraces() *ExecutionTraces {
	return &ExecutionTraces{
		ContractsBytecode: make(map[types.Address][]byte),
	}
}

func (tr *ExecutionTraces) AddContractBytecode(addr types.Address, code []byte) {
	tr.ContractsBytecode[addr] = code
}

// Append adds `other` to the end of traces slices, adds kv pairs from `otherTrace` maps
func (tr *ExecutionTraces) Append(other *ExecutionTraces) {
	// TODO: add merging when MPT circuit is designed. Currently, only the last block mpt traces are saved.
	tr.MPTTraces = other.MPTTraces

	tr.MemoryOps = append(tr.MemoryOps, other.MemoryOps...)
	tr.StackOps = append(tr.StackOps, other.StackOps...)
	tr.StorageOps = append(tr.StorageOps, other.StorageOps...)
	tr.ExpOps = append(tr.ExpOps, other.ExpOps...)
	tr.ZKEVMStates = append(tr.ZKEVMStates, other.ZKEVMStates...)
	tr.CopyEvents = append(tr.CopyEvents, other.CopyEvents...)
	tr.KeccakTraces = append(tr.KeccakTraces, other.KeccakTraces...)
	if tr.ZethCache == nil {
		tr.ZethCache = other.ZethCache
	} else {
		tr.ZethCache.Append(other.ZethCache)
	}

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
