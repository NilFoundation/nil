package tracer

import (
	"errors"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type CopyLocation int

const (
	CopyLocationMemory     CopyLocation = iota // current context memory
	CopyLocationBytecode                       // current (or some another) contract bytecode
	CopyLocationCalldata                       // context or subcontext calldata
	CopyLocationLog                            // write-only: log storage
	CopyLocationKeccak                         // write-only: keccak calculator
	CopyLocationReturnData                     // returndata section of current context or subcontext
)

type CopyParticipant struct {
	Location CopyLocation

	// one of
	TxId         *uint // Index of transaction in block
	BytecodeHash *common.Hash
	KeccakHash   *common.Hash

	// optional if the location is not a memory
	MemAddress uint64
}

type CopyEvent struct {
	From, To CopyParticipant
	RwIdx    uint // global rw counter at the beginning of memory ops execution
	Data     []byte
}

// aux interface to fetch contract codes
type CodeProvider interface {
	GetCurrentCode() ([]byte, common.Hash, error)
	GetCode(types.Address) ([]byte, common.Hash, error)
}

type CopyTracer struct {
	codeProvider CodeProvider
	msgId        uint // transaction id in block

	// array of recorded events
	events []CopyEvent

	// initialized during TraceOp if the event requires to be enriched with some data from stack or memory after actual op execution
	finalizer func()
}

func (ct *CopyTracer) TraceOp(
	opCode vm.OpCode,
	rwCounter uint, // current global RW counter
	opCtx tracing.OpContext,
	returnData []byte,
) bool {
	extractEvent, ok := copyEventExtractors[opCode]
	if !ok {
		return false
	}
	if ct.finalizer != nil {
		panic(errors.New("copy event trace corrupted: previous opcode is not finalized"))
	}

	tCtx := copyEventTraceContext{
		txId:         ct.msgId,
		vmCtx:        opCtx,
		returnData:   returnData,
		codeProvider: ct.codeProvider,
		stack:        NewStackAccessor(opCtx.StackData()),
	}

	eventData := extractEvent(tCtx)
	if eventData.event == nil {
		return false // opcode was found but copy event is not traced
	}
	if len(eventData.event.Data) == 0 {
		return false // zero-sized copy ops are not expected to be processed by copy circuit
	}

	eventData.event.RwIdx = rwCounter
	eventData.event.Data = slices.Clone(eventData.event.Data) // avoid keeping whole EVM memory bunch in RAM

	ct.events = append(ct.events, *eventData.event)

	if eventData.finalizer != nil {
		ct.finalizer = func() {
			eventData.finalizer(&ct.events[len(ct.events)-1])
		}
	}
	return true
}

func (ct *CopyTracer) FinishPrevOpcodeTracing() {
	if ct.finalizer == nil {
		return
	}

	ct.finalizer()
	ct.finalizer = nil
}

func (ct *CopyTracer) Finalize() []CopyEvent {
	ct.FinishPrevOpcodeTracing()
	return ct.events
}

type copyEventFinalizer func(*CopyEvent)

type copyEvent struct {
	event     *CopyEvent
	finalizer copyEventFinalizer
}

func newFinalizedCopyEvent(base CopyEvent) copyEvent {
	return copyEvent{event: &base}
}

func newCopyEventWithFinalizer(
	base CopyEvent,
	finalizer copyEventFinalizer,
) copyEvent {
	return copyEvent{
		event:     &base,
		finalizer: finalizer,
	}
}

func newEmptyCopyEvent() copyEvent {
	return copyEvent{}
}

// some extended context fields required by most of the opcodes to build an event
type copyEventTraceContext struct {
	txId         uint // transaction number in block
	vmCtx        tracing.OpContext
	stack        *StackAccessor
	codeProvider CodeProvider
	returnData   []byte
}

type copyEventExtractor func(tCtx copyEventTraceContext) copyEvent

var copyEventExtractors = map[vm.OpCode]copyEventExtractor{
	vm.MCOPY: func(tCtx copyEventTraceContext) copyEvent {
		var (
			dst  = tCtx.stack.PopUint64()
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.vmCtx.MemoryData()[src : src+size]
		)

		return newFinalizedCopyEvent(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: dst,
			},
			Data: data,
		})
	},

	vm.CODECOPY: func(tCtx copyEventTraceContext) copyEvent {
		var (
			dst  = tCtx.stack.PopUint64()
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
		)

		code, hash, err := tCtx.codeProvider.GetCurrentCode()
		if err != nil {
			panic(err) // should not obtain error on fetching executing code
		}
		data := code[src : src+size]

		return newFinalizedCopyEvent(CopyEvent{
			From: CopyParticipant{
				Location:     CopyLocationBytecode,
				BytecodeHash: &hash,
				MemAddress:   src,
			},
			To: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: dst,
			},
			Data: data,
		})
	},

	vm.EXTCODECOPY: func(tCtx copyEventTraceContext) copyEvent {
		var (
			addr            types.Address
			extCodeAddrWord = tCtx.stack.Pop()
			dst             = tCtx.stack.PopUint64()
			src             = tCtx.stack.PopUint64()
			size            = tCtx.stack.PopUint64()
		)
		addr.SetBytes(extCodeAddrWord.Bytes())

		code, hash, err := tCtx.codeProvider.GetCode(addr)
		if err != nil {
			panic(err) // should not obtain error on fetching loaded code
		}
		data := code[src : src+size]

		return newFinalizedCopyEvent(CopyEvent{
			From: CopyParticipant{
				Location:     CopyLocationBytecode,
				BytecodeHash: &hash,
				MemAddress:   src,
			},
			To: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: dst,
			},
			Data: data,
		})
	},

	vm.CALLDATACOPY: func(tCtx copyEventTraceContext) copyEvent {
		var (
			dst  = tCtx.stack.PopUint64()
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.vmCtx.CallInput()[src : src+size]
		)

		return newFinalizedCopyEvent(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationCalldata,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: dst,
			},
			Data: data,
		})
	},

	vm.RETURN: func(tCtx copyEventTraceContext) copyEvent {
		var (
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.vmCtx.MemoryData()[src : src+size]
		)

		return newFinalizedCopyEvent(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location: CopyLocationReturnData,
				TxId:     &tCtx.txId,
			},
			Data: data,
		})
	},

	vm.RETURNDATACOPY: func(tCtx copyEventTraceContext) copyEvent {
		var (
			dst  = tCtx.stack.PopUint64()
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.returnData[src : src+size]
		)

		return newFinalizedCopyEvent(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationReturnData,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: dst,
			},
			Data: data,
		})
	},

	vm.CREATE: func(tCtx copyEventTraceContext) copyEvent {
		stackAfter := *tCtx.stack
		stackAfter.Skip(2) // CREATE peeks 3 args and returns 1
		finalizer := makeCreateOpCodeFinalizer(&stackAfter, tCtx.codeProvider)

		var (
			_    = tCtx.stack.Pop() // value
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.vmCtx.MemoryData()[src : src+size]
		)
		return newCopyEventWithFinalizer(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location: CopyLocationBytecode,
				// bytecode hash will be set by finalizer
			},
			Data: data,
		}, finalizer)
	},

	vm.CREATE2: func(tCtx copyEventTraceContext) copyEvent {
		stackAfter := *tCtx.stack
		stackAfter.Skip(3) // CREATE2 peeks 4 args and returns 1
		finalizer := makeCreateOpCodeFinalizer(&stackAfter, tCtx.codeProvider)

		var (
			_    = tCtx.stack.Pop() // value
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.vmCtx.MemoryData()[src : src+size]
		)

		return newCopyEventWithFinalizer(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location: CopyLocationBytecode,
				// bytecode hash will be set by finalizer
			},
			Data: data,
		}, finalizer)
	},

	vm.KECCAK256: func(tCtx copyEventTraceContext) copyEvent {
		stackAfter := *tCtx.stack
		stackAfter.Skip(1) // keccak peeks 2 arguments and returns one
		finalizer := func(event *CopyEvent) {
			var result common.Hash
			result.SetBytes(stackAfter.Pop().Bytes())
			event.To.KeccakHash = &result
		}

		var (
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = tCtx.vmCtx.MemoryData()[src : src+size]
		)

		return newCopyEventWithFinalizer(CopyEvent{
			From: CopyParticipant{
				Location:   CopyLocationMemory,
				TxId:       &tCtx.txId,
				MemAddress: src,
			},
			To: CopyParticipant{
				Location: CopyLocationKeccak,
				// keccak hash will be set by finalizer
			},
			Data: data,
		}, finalizer)
	},

	vm.LOG0: newLogCopyEvent,
	vm.LOG1: newLogCopyEvent,
	vm.LOG2: newLogCopyEvent,
	vm.LOG3: newLogCopyEvent,
	vm.LOG4: newLogCopyEvent,

	// xCALL opcodes circuit design is not finalized yet. Seems like they need to be traced in the following way:
	// - copy event [context memory --> sub-context calldata]
	// - copy event [sub-context memory --> context returndata]. It is not expected to be traced by this opcode but
	// its presence shall be guaranteed by tracing the corresponding RETURN/REVERT opcode
	// - copy event [context returndata -> context memory]
	//
	// TODO vm.CALL
	// TODO vm.CALLCODE
	// TODO vm.DELEGATECALL
	// TODO vm.STATICCALL
}

// common way to trace all LOGx opcode copy event
func newLogCopyEvent(tCtx copyEventTraceContext) copyEvent {
	var (
		src  = tCtx.stack.PopUint64()
		size = tCtx.stack.PopUint64()
	)
	if size == 0 {
		return newEmptyCopyEvent()
	}

	data := tCtx.vmCtx.MemoryData()[src : src+size]

	return newFinalizedCopyEvent(CopyEvent{
		From: CopyParticipant{
			Location:   CopyLocationMemory,
			TxId:       &tCtx.txId,
			MemAddress: src,
		},
		To: CopyParticipant{
			Location: CopyLocationLog,
			TxId:     &tCtx.txId,
		},
		Data: data,
	})
}

// provides deployed bytecode hash fetcher from the stack
func makeCreateOpCodeFinalizer(stack *StackAccessor, codeProvider CodeProvider) copyEventFinalizer {
	return func(event *CopyEvent) {
		var codeAddr types.Address
		codeAddr.SetBytes(stack.Pop().Bytes())
		_, codeHash, err := codeProvider.GetCode(codeAddr)
		if err != nil {
			panic(err) // do not expect fail after opcode execution
		}
		event.To.BytecodeHash = &codeHash
	}
}
