package tracer

import (
	"github.com/NilFoundation/nil/nil/common/check"
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

func (sot *ExpOpTracer) TraceOp(opCode vm.OpCode, pc uint64, scope tracing.OpContext) bool {
	if opCode != vm.EXP {
		return false
	}
	sot.opCode = opCode
	sot.pc = pc
	sot.scope = scope

	stack := NewStackAccessor(sot.scope.StackData())
	base, exponent := stack.Pop(), stack.Pop()

	check.PanicIfNotf(sot.prevOpFinisher == nil, "previous operation finisher was not called")
	sot.prevOpFinisher = func() {
		stack := NewStackAccessor(sot.scope.StackData())
		computedValue := stack.Pop()
		sot.res = append(sot.res, ExpOp{
			Base:     base,
			Exponent: exponent,
			Result:   computedValue,
			PC:       sot.pc,
			MsgId:    sot.msgId,
		})
	}
	return true
}

func (sot *ExpOpTracer) FinishPrevOpcodeTracing() {
	if sot.prevOpFinisher == nil {
		return
	}

	sot.prevOpFinisher()
	sot.prevOpFinisher = nil
}

func (sot *ExpOpTracer) Finalize() []ExpOp {
	sot.FinishPrevOpcodeTracing()
	return sot.res
}
