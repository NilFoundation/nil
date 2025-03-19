package tracer

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type transactionTraceContext struct {
	rwCounter *RwCounter // sequential RW operations counter

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
	// to keep RW ctr correct, stack tracer should be run the first on new opcode, and be finalized
	// the last on previous opcode.
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

func (mtc *transactionTraceContext) saveTransactionTraces(dst *ExecutionTraces) error {
	copyEvents, err := mtc.copyTracer.Finalize()
	if err != nil {
		return err
	}

	dst.Append(&ExecutionTraces{
		StackOps:     mtc.stackTracer.Finalize(),
		MemoryOps:    mtc.memoryTracer.Finalize(),
		StorageOps:   mtc.storageTracer.GetStorageOps(),
		ExpOps:       mtc.expTracer.Finalize(),
		ZKEVMStates:  mtc.zkevmTracer.Finalize(),
		CopyEvents:   copyEvents,
		KeccakTraces: mtc.keccakTracer.Finalize(),
	})

	return nil
}

type Tracer struct {
	stateDb      vm.StateDB
	Traces       *ExecutionTraces
	Stats        *Stats
	rwCounter    *RwCounter // sequential RW operations counter
	curTxIdx     uint
	TracingError error
	// Reinited for each transaction
	txnTraceCtx *transactionTraceContext
}

func NewEVMTracer(stateDb vm.StateDB) *Tracer {
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
		rwCounter: t.rwCounter,

		stackTracer:   NewStackOpTracer(t.rwCounter, t.curTxIdx),
		memoryTracer:  NewMemoryOpTracer(t.rwCounter, t.curTxIdx),
		expTracer:     NewExpOpTracer(t.curTxIdx),
		keccakTracer:  NewKeccakTracer(),
		storageTracer: NewStorageOpTracer(t.rwCounter, t.curTxIdx, t.stateDb),

		zkevmTracer: NewZkEVMStateTracer(
			t.rwCounter,
			txHash,
			t.curTxIdx,
		),

		copyTracer: NewCopyTracer(t.stateDb, t.curTxIdx),
	}
	t.curTxIdx++
}

func (t *Tracer) getTracingHooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart: func(evmCtx *tracing.VMContext, tx *types.Transaction) {
			t.initTransactionTraceContext(tx.Hash())
		},
		OnOpcode: func(
			pc uint64,
			op byte,
			gas uint64,
			cost uint64,
			scope tracing.OpContext,
			returnData []byte,
			depth int,
			err error,
		) {
			if err != nil || t.TracingError != nil {
				// if opcode failed, no need to trace anything
				// if we encountered tracing error before, skip further processing
				return
			}

			// debug-only: ensure that tracer impl did not change any data from the EVM context
			verifyIntegrity := assertEVMStateConsistent(pc, scope, returnData)
			defer verifyIntegrity()

			if err := t.txnTraceCtx.processOpcode(t.Stats, pc, op, gas, scope, returnData); err != nil {
				t.TracingError = fmt.Errorf(
					"pc: %d opcode: %X, gas: %d, cost: %d, mem_size: %d bytes, "+
						"stack: %d items, ret_data_size: %d bytes, depth: %d cause: %w",
					pc, op, gas, cost,
					len(scope.MemoryData()), len(scope.StackData()),
					len(returnData), depth, err,
				)
				return
			}

			t.Traces.AddContractBytecode(scope.Address(), scope.Code())
		},
		OnTxEnd: func(evmCtx *tracing.VMContext, tx *types.Transaction, err types.ExecError) {
			defer t.resetTxnTrace()
			if err != nil {
				// if error occurred, no need to trace anything
				return
			}
			if err := t.saveTransactionTraces(); err != nil {
				panic(err)
			}
		},
	}
}

func (t *Tracer) resetTxnTrace() {
	t.txnTraceCtx = nil
}

func (t *Tracer) saveTransactionTraces() error {
	return t.txnTraceCtx.saveTransactionTraces(t.Traces)
}
