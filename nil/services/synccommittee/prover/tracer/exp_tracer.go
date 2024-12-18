package tracer

import (
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/holiman/uint256"
)

type ExpOp struct {
	Base     *uint256.Int
	Exponent *uint256.Int
	Result   *uint256.Int
	PC       uint64
	MsgId    uint
}

type ExpOpTracer struct {
	res   []ExpOp
	msgId uint

	opCode         vm.OpCode
	pc             uint64
	scope          tracing.OpContext
	prevOpFinisher func()
}

func NewExpOpTracer(msgId uint) *ExpOpTracer {
	return &ExpOpTracer{
		msgId: msgId,
	}
}

func (eot *ExpOpTracer) TraceOp(opCode vm.OpCode, pc uint64, scope tracing.OpContext) (bool, error) {
	if opCode != vm.EXP {
		return false, nil
	}
	if eot.prevOpFinisher != nil {
		return false, ErrTraceNotFinalized
	}

	eot.opCode = opCode
	eot.pc = pc
	eot.scope = scope

	stack := NewStackAccessor(eot.scope.StackData())
	base, exponent := stack.Pop(), stack.Pop()

	eot.prevOpFinisher = func() {
		stack := NewStackAccessor(eot.scope.StackData())
		computedValue := stack.Pop()
		eot.res = append(eot.res, ExpOp{
			Base:     base,
			Exponent: exponent,
			Result:   computedValue,
			PC:       eot.pc,
			MsgId:    eot.msgId,
		})
	}
	return true, nil
}

func (eot *ExpOpTracer) FinishPrevOpcodeTracing() {
	if eot.prevOpFinisher == nil {
		return
	}

	eot.prevOpFinisher()
	eot.prevOpFinisher = nil
}

func (eot *ExpOpTracer) Finalize() []ExpOp {
	eot.FinishPrevOpcodeTracing()
	return eot.res
}
