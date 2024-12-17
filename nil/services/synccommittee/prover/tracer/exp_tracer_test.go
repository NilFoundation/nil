package tracer

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockOpContext struct {
	stack []uint256.Int
}

func (m *MockOpContext) MemoryData() []byte {
	return nil
}

func (m *MockOpContext) StackData() []uint256.Int {
	return m.stack
}

func (m *MockOpContext) Caller() types.Address {
	return types.Address{}
}

func (m *MockOpContext) Address() types.Address {
	return types.Address{}
}

func (m *MockOpContext) CallValue() *uint256.Int {
	return uint256.NewInt(0)
}

func (m *MockOpContext) CallInput() []byte {
	return nil
}

// traceExpOperation encapsulates the setup and invocation of tracing an EXP operation.
func traceExpOperation(tracer *ExpOpTracer, opCode vm.OpCode, pc uint64, base, exponent uint64) {
	base256 := uint256.NewInt(base)
	context := &MockOpContext{
		stack: []uint256.Int{*uint256.NewInt(exponent), *base256},
	}

	tracer.TraceOp(opCode, pc, context)

	if opCode == vm.EXP {
		// mimic `opExp`
		context.stack = context.stack[:1]
		context.stack[0].Exp(base256, &context.stack[0])
	}

	tracer.FinishPrevOpcodeTracing()
}

func TestExpOpTracer_HandlesExpOperation(t *testing.T) {
	t.Parallel()
	tracer := &ExpOpTracer{}

	traceExpOperation(tracer, vm.EXP, 0, 2, 3)

	require.Len(t, tracer.res, 1)
	op := tracer.res[0]
	assert.Equal(t, uint256.NewInt(2), op.Base)
	assert.Equal(t, uint256.NewInt(3), op.Exponent)
	assert.Equal(t, uint256.NewInt(8), op.Result)
}

func TestExpOpTracer_IgnoresNonExpOperations(t *testing.T) {
	t.Parallel()
	tracer := &ExpOpTracer{}

	traceExpOperation(tracer, vm.ADD, 0, 2, 3) // Non-EXP opcode should result in no operation captured

	assert.Empty(t, tracer.res)
}

func TestExpOpTracer_MaintainsCorrectStateAcrossCalls(t *testing.T) {
	t.Parallel()
	tracer := &ExpOpTracer{}

	traceExpOperation(tracer, vm.EXP, 0, 2, 3)
	traceExpOperation(tracer, vm.EXP, 1, 3, 4)

	require.Len(t, tracer.res, 2)
	assert.Equal(t, uint256.NewInt(3), tracer.res[1].Base)
	assert.Equal(t, uint256.NewInt(4), tracer.res[1].Exponent)
	assert.Equal(t, uint256.NewInt(81), tracer.res[1].Result)
}
