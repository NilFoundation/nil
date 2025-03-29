package tracer

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/testutils"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	TestAddress = types.ShardAndHexToAddress(types.BaseShardId, "111111111111111111111111111111111110")
	PrevValue   = common.BytesToHash([]byte("123"))
)

// traceStorageOperation encapsulates the setup and invocation of tracing storage operation.
func traceStorageOperation(t *testing.T, tracer *StorageOpTracer, opCode vm.OpCode, pc uint64, val uint256.Int) bool {
	t.Helper()
	stack := []uint256.Int{val, val} // used as address at SLOAD, as new value in SSTORE
	context := &testutils.OpContextMock{
		StackDataFunc: func() []uint256.Int { return stack },
		AddressFunc: func() types.Address {
			return TestAddress
		},
	}

	traced, err := tracer.TraceOp(opCode, pc, context)
	require.NoError(t, err)

	if opCode == vm.SSTORE {
		// mimic `opSstore`
		stack = stack[:0]
	} else if opCode == vm.SLOAD {
		// mimic `opSload`
		stack = []uint256.Int{val}
	}

	tracer.FinishPrevOpcodeTracing()

	return traced
}

func newTracerWithMockedGetter(t *testing.T) *StorageOpTracer {
	t.Helper()
	return NewStorageOpTracer(
		&RwCounter{},
		0,
		&StateGetterMock{
			GetStateFunc: func(addr types.Address, key common.Hash) (common.Hash, error) {
				return PrevValue, nil
			},
		},
	)
}

func TestIgnoresNonStorageOperations(t *testing.T) {
	t.Parallel()
	tracer := newTracerWithMockedGetter(t)

	// Non-storage opcode should result in no operation captured
	assert.False(t, traceStorageOperation(t, tracer, vm.ADD, 0, *uint256.NewInt(0)))

	assert.Empty(t, tracer.storageOps)
}

func TestMultipleOpcodes(t *testing.T) {
	t.Parallel()
	tracer := newTracerWithMockedGetter(t)

	val := *uint256.NewInt(1)
	assert.True(t, traceStorageOperation(t, tracer, vm.SLOAD, 0, val))
	assert.True(t, traceStorageOperation(t, tracer, vm.SSTORE, 1, val))
	assert.False(t, traceStorageOperation(t, tracer, vm.ADD, 0, val))

	require.Len(t, tracer.storageOps, 2)
}

func TestSloadExistent(t *testing.T) {
	t.Parallel()
	tracer := newTracerWithMockedGetter(t)

	val := *uint256.NewInt(1)
	assert.True(t, traceStorageOperation(t, tracer, vm.SLOAD, 0, val))

	require.Len(t, tracer.storageOps, 1)

	sloadTrace := tracer.storageOps[0]
	assert.True(t, sloadTrace.IsRead)
	assert.Equal(t, common.Hash(val.Bytes32()), sloadTrace.Key)
	assert.Equal(t, types.Uint256(val), sloadTrace.Value)
	assert.Equal(t, types.Uint256(val), sloadTrace.PrevValue) // value and prev value are the same in SLOAD trace
	assert.Equal(t, uint64(0), sloadTrace.PC)
	assert.Equal(t, uint(0), sloadTrace.TxnId)
	assert.Equal(t, uint(0), sloadTrace.RwIdx)
	assert.Equal(t, TestAddress, sloadTrace.Addr)
}

func TestSstore(t *testing.T) {
	t.Parallel()
	tracer := newTracerWithMockedGetter(t)

	val := *uint256.NewInt(1)
	assert.True(t, traceStorageOperation(t, tracer, vm.SSTORE, 0, val))

	sstoreTrace := tracer.storageOps[0]
	assert.False(t, sstoreTrace.IsRead)
	assert.Equal(t, common.Hash(val.Bytes32()), sstoreTrace.Key)
	assert.Equal(t, types.Uint256(val), sstoreTrace.Value)
	assert.Equal(t, types.Uint256(*PrevValue.Uint256()), sstoreTrace.PrevValue)
	assert.Equal(t, uint64(0), sstoreTrace.PC)
	assert.Equal(t, uint(0), sstoreTrace.TxnId)
	assert.Equal(t, uint(0), sstoreTrace.RwIdx)
	assert.Equal(t, TestAddress, sstoreTrace.Addr)
}
