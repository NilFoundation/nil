package tracer

import (
	"bytes"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/testutils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// traceExpOperation encapsulates the setup and invocation of tracing an KECCAK256 operation.
func traceKeccakOperation(t *testing.T, tracer *KeccakTracer, opCode vm.OpCode, offset, size uint64, mem []byte) bool {
	t.Helper()
	stack := []uint256.Int{*uint256.NewInt(size), *uint256.NewInt(offset)}
	context := &testutils.OpContextMock{
		StackDataFunc:  func() []uint256.Int { return stack },
		MemoryDataFunc: func() []byte { return mem },
	}

	traced, err := tracer.TraceOp(opCode, context)
	require.NoError(t, err)

	if opCode == vm.KECCAK256 {
		stack = stack[:1]
		hash := crypto.Keccak256(mem[offset : offset+size])
		var u256val uint256.Int
		u256val.SetBytes(hash)
		stack[0] = u256val
	}

	tracer.FinishPrevOpcodeTracing()
	return traced
}

func TestKeccakTracer_HandlesKeccakOperation(t *testing.T) {
	t.Parallel()
	tracer := NewKeccakTracer()

	buf := bytes.Repeat([]byte{0xFF}, 8)
	assert.True(t, traceKeccakOperation(t, tracer, vm.KECCAK256, 0, 4, buf))

	require.Len(t, tracer.hashes, 1)
	assert.Equal(t, buf[:4], tracer.hashes[0].buf)
	expectedHash, err := uint256.FromHex("0x29045A592007D0C246EF02C2223570DA9522D0CF0F73282C79A1BC8F0BB2C238")
	require.NoError(t, err)
	assert.Equal(t, expectedHash.Bytes(), tracer.hashes[0].hash.Bytes())
}

func TestKeccakTracer_IgnoresOtherOperations(t *testing.T) {
	t.Parallel()
	tracer := NewKeccakTracer()

	// Non-KECCAK256 opcode should result in no operation captured
	assert.False(t, traceKeccakOperation(t, tracer, vm.ADD, 0, 2, []byte{1, 2, 3, 4}))

	assert.Empty(t, tracer.hashes)
}

func TestKeccakTracer_MaintainsCorrectStateAcrossCalls(t *testing.T) {
	t.Parallel()
	tracer := NewKeccakTracer()

	assert.True(t, traceKeccakOperation(t, tracer, vm.KECCAK256, 1, 4, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}))
	assert.True(t, traceKeccakOperation(t, tracer, vm.KECCAK256, 0, 4, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	require.Len(t, tracer.hashes, 2)
}
