package tracer

import (
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type MemoryOp struct {
	IsRead bool // Is write otherwise
	Idx    int  // Index of element in memory
	Value  byte
	PC     uint64
	MsgId  uint
	RwIdx  uint
}

type MemoryOpTracer struct {
	rwCtr *RwCounter
	msgId uint
	res   []MemoryOp

	prevOpFinisher func()
}

func NewMemoryOpTracer(rwCounter *RwCounter, msgId uint) *MemoryOpTracer {
	return &MemoryOpTracer{
		rwCtr: rwCounter,
		msgId: msgId,
	}
}

type memoryRange struct {
	offset uint64
	length uint64
}

type opRanges struct {
	before memoryRange
	after  memoryRange
}

// TODO: refactor, each opcode contains stack init logic
var opsToMemoryRanges = map[vm.OpCode]func(stack *StackAccessor, memoryLen int) opRanges{
	vm.KECCAK256: func(stack *StackAccessor, _ int) opRanges {
		offset := stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), 32},
		}
	},
	vm.CALLDATACOPY: func(stack *StackAccessor, _ int) opRanges {
		var (
			memOffset    = stack.Pop()
			_            = stack.Pop()
			lengthToCopy = stack.Pop()
		)

		return opRanges{
			after: memoryRange{memOffset.Uint64(), lengthToCopy.Uint64()},
		}
	},
	vm.RETURNDATACOPY: func(stack *StackAccessor, _ int) opRanges {
		var (
			memOffset    = stack.Pop()
			_            = stack.Pop()
			lengthToCopy = stack.Pop()
		)

		// Data to copy is read from returndata, not from memory. Not handled here.
		return opRanges{
			after: memoryRange{memOffset.Uint64(), lengthToCopy.Uint64()},
		}
	},
	vm.CODECOPY: func(stack *StackAccessor, _ int) opRanges {
		var (
			memOffset    = stack.Pop()
			_            = stack.Pop()
			lengthToCopy = stack.Pop()
		)
		// Data to copy is read from contract, not from memory. Not handled here.
		return opRanges{
			after: memoryRange{memOffset.Uint64(), lengthToCopy.Uint64()},
		}
	},
	vm.EXTCODECOPY: func(stack *StackAccessor, _ int) opRanges {
		var (
			_            = stack.Pop()
			memOffset    = stack.Pop()
			_            = stack.Pop()
			lengthToCopy = stack.Pop()
		)
		// Data to copy is read from external code, not from memory. Not handled here.
		return opRanges{
			after: memoryRange{memOffset.Uint64(), lengthToCopy.Uint64()},
		}
	},
	vm.MLOAD: func(stack *StackAccessor, _ int) opRanges {
		memOffset := stack.Pop()
		return opRanges{
			before: memoryRange{memOffset.Uint64(), 32},
		}
	},
	vm.MSTORE: func(stack *StackAccessor, _ int) opRanges {
		memOffset := stack.Pop()
		return opRanges{
			after: memoryRange{memOffset.Uint64(), 32},
		}
	},
	vm.MSTORE8: func(stack *StackAccessor, _ int) opRanges {
		memOffset := stack.Pop()
		return opRanges{
			after: memoryRange{memOffset.Uint64(), 1},
		}
	},
	vm.MCOPY: func(stack *StackAccessor, _ int) opRanges {
		var (
			dstOffset = stack.Pop()
			srcOffset = stack.Pop()
			size      = stack.Pop()
		)
		return opRanges{
			before: memoryRange{srcOffset.Uint64(), size.Uint64()},
			after:  memoryRange{dstOffset.Uint64(), size.Uint64()},
		}
	},
	vm.CREATE: func(stack *StackAccessor, _ int) opRanges {
		var (
			_            = stack.Pop()
			memOffset    = stack.Pop()
			lengthToCopy = stack.Pop()
		)
		return opRanges{
			before: memoryRange{memOffset.Uint64(), lengthToCopy.Uint64()},
		}
	},
	vm.CREATE2: func(stack *StackAccessor, _ int) opRanges {
		var (
			_            = stack.Pop()
			memOffset    = stack.Pop()
			lengthToCopy = stack.Pop()
		)
		return opRanges{
			before: memoryRange{memOffset.Uint64(), lengthToCopy.Uint64()},
		}
	},
	vm.CALL: func(stack *StackAccessor, _ int) opRanges {
		stack.Skip(3)
		var (
			inOffset  = stack.Pop()
			inSize    = stack.Pop()
			retOffset = stack.Pop()
			retSize   = stack.Pop()
		)
		return opRanges{
			before: memoryRange{inOffset.Uint64(), inSize.Uint64()},
			after:  memoryRange{retOffset.Uint64(), retSize.Uint64()},
		}
	},
	vm.CALLCODE: func(stack *StackAccessor, _ int) opRanges {
		stack.Skip(3)
		var (
			inOffset  = stack.Pop()
			inSize    = stack.Pop()
			retOffset = stack.Pop()
			retSize   = stack.Pop()
		)
		return opRanges{
			before: memoryRange{inOffset.Uint64(), inSize.Uint64()},
			after:  memoryRange{retOffset.Uint64(), retSize.Uint64()},
		}
	},
	vm.DELEGATECALL: func(stack *StackAccessor, _ int) opRanges {
		stack.Skip(2)
		var (
			inOffset  = stack.Pop()
			inSize    = stack.Pop()
			retOffset = stack.Pop()
			retSize   = stack.Pop()
		)
		return opRanges{
			before: memoryRange{inOffset.Uint64(), inSize.Uint64()},
			after:  memoryRange{retOffset.Uint64(), retSize.Uint64()},
		}
	},
	vm.STATICCALL: func(stack *StackAccessor, _ int) opRanges {
		stack.Skip(2)
		var (
			inOffset  = stack.Pop()
			inSize    = stack.Pop()
			retOffset = stack.Pop()
			retSize   = stack.Pop()
		)
		return opRanges{
			before: memoryRange{inOffset.Uint64(), inSize.Uint64()},
			after:  memoryRange{retOffset.Uint64(), retSize.Uint64()},
		}
	},
	vm.RETURN: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
	vm.REVERT: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
	vm.LOG0: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
	vm.LOG1: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
	vm.LOG2: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
	vm.LOG3: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
	vm.LOG4: func(stack *StackAccessor, _ int) opRanges {
		offset, size := stack.Pop(), stack.Pop()
		return opRanges{
			before: memoryRange{offset.Uint64(), size.Uint64()},
		}
	},
}

func (mot *MemoryOpTracer) FinishPrevOpcodeTracing() {
	if mot.prevOpFinisher == nil {
		return
	}

	mot.prevOpFinisher()
	mot.prevOpFinisher = nil
}

func (mot *MemoryOpTracer) GetUsedMemoryRanges(opCode vm.OpCode, scope tracing.OpContext) (opRanges, bool) {
	memRangesFunc, ok := opsToMemoryRanges[opCode]
	if !ok {
		return opRanges{}, false
	}
	stackAccessor := NewStackAccessor(scope.StackData())
	memRanges := memRangesFunc(stackAccessor, len(scope.MemoryData()))
	return memRanges, true
}

func (mot *MemoryOpTracer) TraceOp(opCode vm.OpCode, pc uint64, memRanges opRanges, scope tracing.OpContext) error {
	if mot.prevOpFinisher != nil {
		return ErrTraceNotFinalized
	}

	for i := memRanges.before.offset; i < memRanges.before.offset+memRanges.before.length; i++ {
		var databyte byte
		// EVM does not break on attempt to "read" from memory region which are out of currently allocated range
		// so we should not fail too. Instead of it every read from missing memory cell is replaced with 0 byte
		if i < uint64(len(scope.MemoryData())) {
			databyte = scope.MemoryData()[i]
		}
		mot.res = append(mot.res, MemoryOp{
			IsRead: true,
			Idx:    int(i),
			Value:  databyte,
			PC:     pc,
			MsgId:  mot.msgId,
			RwIdx:  mot.rwCtr.NextIdx(),
		})
	}
	mot.prevOpFinisher = func() {
		for i := memRanges.after.offset; i < memRanges.after.offset+memRanges.after.length; i++ {
			mot.res = append(mot.res, MemoryOp{
				IsRead: false,
				Idx:    int(i),
				Value:  scope.MemoryData()[i],
				PC:     pc,
				MsgId:  mot.msgId,
				RwIdx:  mot.rwCtr.NextIdx(),
			})
		}
	}
	return nil
}

func (mot *MemoryOpTracer) Finalize() []MemoryOp {
	mot.FinishPrevOpcodeTracing()
	return mot.res
}
