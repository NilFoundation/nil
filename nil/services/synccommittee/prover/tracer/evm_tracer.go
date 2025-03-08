package tracer

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type transactionTraceContext struct {
	evm *vm.EVM // EVM instance re-running current transaction
	// code      []byte      // currently executed code
	// codeHash  common.Hash // hash of this.code
	rwCounter *RwCounter // inherited from TracerStateDB, sequential RW operations counter

	// tracers recording different events
	stackTracer   *StackOpTracer
	memoryTracer  *MemoryOpTracer
	storageTracer *StorageOpTracer
	zkevmTracer   *ZKEVMStateTracer
	copyTracer    *CopyTracer
	expTracer     *ExpOpTracer
	keccakTracer  *KeccakTracer
}

func (mtc *transactionTraceContext) processOpcode(
	stats *Stats,
	pc uint64,
	op byte,
	gas uint64,
	scope tracing.OpContext,
	returnData []byte,
) error {
	opCode := vm.OpCode(op)
	stats.OpsN++

	// Finish in reverse order to keep rw_counter sequential.
	// Each operation consists of read stack -> read data -> write data -> write stack (we
	// ignore specific memory parts like returndata, etc for now). Intermediate stages could be omitted, but
	// to keep RW ctr correct, stack tracer should be run the first on new opcode, and be finalized the last on previous opcode.
	// TODO: add check that only one of first 3 is run
	mtc.memoryTracer.FinishPrevOpcodeTracing()
	mtc.expTracer.FinishPrevOpcodeTracing()
	mtc.storageTracer.FinishPrevOpcodeTracing()
	mtc.stackTracer.FinishPrevOpcodeTracing()
	mtc.keccakTracer.FinishPrevOpcodeTracing()
	if err := mtc.copyTracer.FinishPrevOpcodeTracing(); err != nil {
		return err
	}

	ranges, hasMemOps := mtc.memoryTracer.GetUsedMemoryRanges(opCode, scope)

	// Store zkevmState before counting rw operations
	if err := mtc.zkevmTracer.TraceOp(opCode, pc, gas, ranges, scope); err != nil {
		return err
	}

	if err := mtc.stackTracer.TraceOp(opCode, pc, scope); err != nil {
		return err
	}
	stats.StackOpsN++

	if hasMemOps {
		copyOccurred, err := mtc.copyTracer.TraceOp(opCode, mtc.rwCounter.ctr, scope, returnData)
		if err != nil {
			return err
		}
		if copyOccurred {
			stats.CopyOpsN++
		}

		if err := mtc.memoryTracer.TraceOp(opCode, pc, ranges, scope); err != nil {
			return err
		}
		stats.MemoryOpsN++
	}

	expTraced, err := mtc.expTracer.TraceOp(opCode, pc, scope)
	if err != nil {
		return err
	}
	if expTraced {
		stats.ExpOpsN++
	}

	keccakTraced, err := mtc.keccakTracer.TraceOp(opCode, scope)
	if err != nil {
		return err
	}
	if keccakTraced {
		stats.KeccakOpsN++
	}

	storageTraced, err := mtc.storageTracer.TraceOp(opCode, pc, scope)
	if err != nil {
		return err
	}
	if storageTraced {
		stats.StateOpsN++
	}

	return nil
}

func (mtc *transactionTraceContext) saveTransactionTraces(dst ExecutionTraces) error {
	dst.AddMemoryOps(mtc.memoryTracer.Finalize())
	dst.AddStackOps(mtc.stackTracer.Finalize())
	dst.AddZKEVMStates(mtc.zkevmTracer.Finalize())
	dst.AddExpOps(mtc.expTracer.Finalize())
	dst.AddKeccakOps(mtc.keccakTracer.Finalize())
	dst.AddStorageOps(mtc.storageTracer.GetStorageOps())

	copies, err := mtc.copyTracer.Finalize()
	if err != nil {
		return err
	}
	dst.AddCopyEvents(copies)

	return nil
}

type Tracer struct {
	stateDb   vm.StateDB
	Traces    ExecutionTraces
	Stats     *Stats
	rwCounter *RwCounter // sequential RW operations counter
	// Reinited for each transaction
	txnTraceCtx *transactionTraceContext
	txnCtr      uint

	evm *vm.EVM
}

func NewTracer(stateDb vm.StateDB) *Tracer {
	return &Tracer{
		stateDb:   stateDb,
		Stats:     &Stats{},
		rwCounter: &RwCounter{},
		Traces:    NewExecutionTraces(),
	}
}

func (t *Tracer) initTransactionTraceContext(
	txHash common.Hash,
) {
	t.txnTraceCtx = &transactionTraceContext{
		evm: t.evm,

		rwCounter: t.rwCounter,

		stackTracer:   NewStackOpTracer(t.rwCounter, t.txnCtr),
		memoryTracer:  NewMemoryOpTracer(t.rwCounter, t.txnCtr),
		expTracer:     NewExpOpTracer(t.txnCtr),
		keccakTracer:  NewKeccakTracer(),
		storageTracer: NewStorageOpTracer(t.rwCounter, t.txnCtr, t.stateDb),

		zkevmTracer: NewZkEVMStateTracer(
			t.rwCounter,
			txHash,
			t.txnCtr,
		),

		copyTracer: NewCopyTracer(t.stateDb, t.txnCtr),
	}
}

func (t *Tracer) getTracingHook() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart: func(tx *types.Transaction) {
			t.initTransactionTraceContext(tx.Hash())
		},
		OnOpcode: func(pc uint64, op byte, gas uint64, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
			if err != nil {
				return // this error will be forwarded to the caller as is, no need to trace anything
			}

			// debug-only: ensure that tracer impl did not change any data from the EVM context
			verifyIntegrity := assertEVMStateConsistent(pc, scope, returnData)
			defer verifyIntegrity()

			if err := t.txnTraceCtx.processOpcode(t.Stats, pc, op, gas, scope, returnData); err != nil {
				err = fmt.Errorf("pc: %d opcode: %X, gas: %d, cost: %d, mem_size: %d bytes, stack: %d items, ret_data_size: %d bytes, depth: %d cause: %w",
					pc, op, gas, cost, len(scope.MemoryData()), len(scope.StackData()), len(returnData), depth, err,
				)

				// tracer by default should not affect the code execution but since we only run code to collect the traces - we should know
				// about any failure as soon as possible instead of continue running
				panic(managedTracerFailureError{underlying: err})
			}

			t.Traces.AddContractBytecode(scope.Address(), scope.Code())
		},
		OnTxEnd: func(tx *types.Transaction, err types.ExecError) {
			if err := t.saveTransactionTraces(); err != nil {
				panic(err)
			}
			t.resetTxnTrace()
		},
	}
}

func (t *Tracer) resetTxnTrace() {
	t.txnTraceCtx = nil
}

func (t *Tracer) saveTransactionTraces() error {
	return t.txnTraceCtx.saveTransactionTraces(t.Traces)
}
