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
	GetCode(types.Address) ([]byte, common.Hash, error)
}

type CopyTracer struct {
	codeProvider CodeProvider
	txnId        uint // transaction id in block

	// array of recorded events
	events []CopyEvent

	// initialized during TraceOp if the event requires to be enriched with some data from stack
	// or memory after actual op execution
	finalizer func() error
}

func NewCopyTracer(codeProvider CodeProvider, txnId uint) *CopyTracer {
	return &CopyTracer{
		codeProvider: codeProvider,
		txnId:        txnId,
	}
}

func (ct *CopyTracer) TraceOp(
	opCode vm.OpCode,
	rwCounter uint, // current global RW counter
	opCtx tracing.OpContext,
	returnData []byte,
) (bool, error) {
	extractEvent, ok := copyEventExtractors[opCode]
	if !ok {
		return false, nil // no copy events for opcode
	}
	if ct.finalizer != nil {
		return false, ErrTraceNotFinalized
	}

	tCtx := copyEventTraceContext{
		txId:         ct.txnId,
		vmCtx:        opCtx,
		returnData:   returnData,
		codeProvider: ct.codeProvider,
		stack:        NewStackAccessor(opCtx.StackData()),
	}

	eventData, err := extractEvent(tCtx)
	if err != nil {
		return false, err
	}
	if eventData.isEmptyCopy() {
		return false, nil
	}

	eventData.event.RwIdx = rwCounter
	eventData.event.Data = slices.Clone(eventData.event.Data) // avoid keeping whole EVM memory bunch in RAM

	ct.events = append(ct.events, *eventData.event)

	if eventData.finalizer != nil {
		ct.finalizer = func() error {
			if len(ct.events) == 0 {
				return errors.New("unexpected finlalized call on empty tracer")
			}
			return eventData.finalizer(&ct.events[len(ct.events)-1])
		}
	}
	return true, nil
}

func (ct *CopyTracer) FinishPrevOpcodeTracing() error {
	if ct.finalizer == nil {
		return nil
	}

	err := ct.finalizer()
	ct.finalizer = nil
	return err
}

func (ct *CopyTracer) Finalize() ([]CopyEvent, error) {
	err := ct.FinishPrevOpcodeTracing()
	if err != nil {
		return nil, err
	}
	return ct.events, nil
}

type copyEventFinalizer func(*CopyEvent) error

type copyEvent struct {
	event     *CopyEvent
	finalizer copyEventFinalizer
}

func (ev *copyEvent) isEmptyCopy() bool {
	return ev.event == nil || // event extractor found that no copy is done actually
		len(ev.event.Data) == 0 // zero-sized copies should not be traced as copy events
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

type copyEventExtractor func(tCtx copyEventTraceContext) (copyEvent, error)

var copyEventExtractors = map[vm.OpCode]copyEventExtractor{
	vm.MCOPY: func(tCtx copyEventTraceContext) (copyEvent, error) {
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
		}), nil
	},

	vm.CODECOPY: func(tCtx copyEventTraceContext) (copyEvent, error) {
		var (
			dst  = tCtx.stack.PopUint64()
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
		)

		code := tCtx.vmCtx.Code()
		data := getFixedSizeDataSafe(code, src, size)
		hash := getCodeHash(code)

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
		}), nil
	},

	vm.EXTCODECOPY: func(tCtx copyEventTraceContext) (copyEvent, error) {
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
			return copyEvent{}, err
		}
		data := getFixedSizeDataSafe(code, src, size)

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
		}), nil
	},

	vm.CALLDATACOPY: func(tCtx copyEventTraceContext) (copyEvent, error) {
		var (
			dst  = tCtx.stack.PopUint64()
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = getFixedSizeDataSafe(tCtx.vmCtx.CallInput(), src, size)
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
		}), nil
	},

	vm.RETURN: func(tCtx copyEventTraceContext) (copyEvent, error) {
		var (
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = getDataIfRangeValid(tCtx.vmCtx.MemoryData(), src, size)
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
		}), nil
	},

	vm.RETURNDATACOPY: func(tCtx copyEventTraceContext) (copyEvent, error) {
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
		}), nil
	},

	vm.CREATE: func(tCtx copyEventTraceContext) (copyEvent, error) {
		stackAfter := *tCtx.stack
		stackAfter.Skip(2) // CREATE peeks 3 args and returns 1
		finalizer := makeCreateOpCodeFinalizer(&stackAfter, tCtx.codeProvider)

		var (
			_    = tCtx.stack.Pop() // value
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = getDataIfRangeValid(tCtx.vmCtx.MemoryData(), src, size)
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
		}, finalizer), nil
	},

	vm.CREATE2: func(tCtx copyEventTraceContext) (copyEvent, error) {
		stackAfter := *tCtx.stack
		stackAfter.Skip(3) // CREATE2 peeks 4 args and returns 1
		finalizer := makeCreateOpCodeFinalizer(&stackAfter, tCtx.codeProvider)

		var (
			_    = tCtx.stack.Pop() // value
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = getDataIfRangeValid(tCtx.vmCtx.MemoryData(), src, size)
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
		}, finalizer), nil
	},

	vm.KECCAK256: func(tCtx copyEventTraceContext) (copyEvent, error) {
		stackAfter := *tCtx.stack
		stackAfter.Skip(1) // keccak peeks 2 arguments and returns one
		finalizer := func(event *CopyEvent) error {
			var result common.Hash
			result.SetBytes(stackAfter.Pop().Bytes())
			event.To.KeccakHash = &result
			return nil
		}

		var (
			src  = tCtx.stack.PopUint64()
			size = tCtx.stack.PopUint64()
			data = getDataIfRangeValid(tCtx.vmCtx.MemoryData(), src, size)
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
		}, finalizer), nil
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
func newLogCopyEvent(tCtx copyEventTraceContext) (copyEvent, error) {
	var (
		src  = tCtx.stack.PopUint64()
		size = tCtx.stack.PopUint64()
	)
	if size == 0 {
		return newEmptyCopyEvent(), nil
	}

	data := getDataIfRangeValid(tCtx.vmCtx.MemoryData(), src, size)

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
	}), nil
}

// provides deployed bytecode hash fetcher from the stack
func makeCreateOpCodeFinalizer(stack *StackAccessor, codeProvider CodeProvider) copyEventFinalizer {
	return func(event *CopyEvent) error {
		var codeAddr types.Address
		codeAddr.SetBytes(stack.Pop().Bytes())
		_, codeHash, err := codeProvider.GetCode(codeAddr)
		if err != nil {
			return err
		}

		event.To.BytecodeHash = &codeHash
		return nil
	}
}
