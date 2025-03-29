package tracer

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/testutils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// traceExpOperation encapsulates the setup and invocation of tracing an EXP operation.
func traceExpOperation(t *testing.T, tracer *ExpOpTracer, opCode vm.OpCode, pc uint64, base, exponent uint64) bool {
	t.Helper()
	base256 := uint256.NewInt(base)
	stack := []uint256.Int{*uint256.NewInt(exponent), *base256}
	context := &testutils.OpContextMock{
		StackDataFunc: func() []uint256.Int { return stack },
	}

	traced, err := tracer.TraceOp(opCode, pc, context)
	require.NoError(t, err)

	if opCode == vm.EXP {
		// mimic `opExp`
		stack = stack[:1]
		stack[0].Exp(base256, &stack[0])
	}

	tracer.FinishPrevOpcodeTracing()
	return traced
}

func TestExpOpTracer_HandlesExpOperation(t *testing.T) {
	t.Parallel()
	tracer := &ExpOpTracer{}

	assert.True(t, traceExpOperation(t, tracer, vm.EXP, 0, 2, 3))

	require.Len(t, tracer.res, 1)
	op := tracer.res[0]
	assert.Equal(t, uint256.NewInt(2), op.Base)
	assert.Equal(t, uint256.NewInt(3), op.Exponent)
	assert.Equal(t, uint256.NewInt(8), op.Result)
}

func TestExpOpTracer_IgnoresNonExpOperations(t *testing.T) {
	t.Parallel()
	tracer := &ExpOpTracer{}

	// Non-EXP opcode should result in no operation captured
	assert.False(t, traceExpOperation(t, tracer, vm.ADD, 0, 2, 3))

	assert.Empty(t, tracer.res)
}

func TestExpOpTracer_MaintainsCorrectStateAcrossCalls(t *testing.T) {
	t.Parallel()
	tracer := &ExpOpTracer{}

	assert.True(t, traceExpOperation(t, tracer, vm.EXP, 0, 2, 3))
	assert.True(t, traceExpOperation(t, tracer, vm.EXP, 1, 3, 4))

	require.Len(t, tracer.res, 2)
	assert.Equal(t, uint256.NewInt(3), tracer.res[1].Base)
	assert.Equal(t, uint256.NewInt(4), tracer.res[1].Exponent)
	assert.Equal(t, uint256.NewInt(81), tracer.res[1].Result)
}
