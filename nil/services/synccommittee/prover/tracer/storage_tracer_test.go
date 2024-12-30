package tracer

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageOpTracer_SetAndGetSlot(t *testing.T) {
	t.Parallel()

	account, mptTracer := mpttracer.CreateTestAccount(t)
	rwCounter := &RwCounter{}
	pc := uint64(100)
	msgId := uint(1)

	interactor := NewStorageOpTracer(
		mptTracer, rwCounter,
		func() (uint64, error) { return pc, nil },
		msgId,
	)

	// Test setting and getting a slot
	key := common.BytesToHash([]byte("test_key"))
	value := common.BytesToHash([]byte("test_value"))

	// Set the slot
	_, err := interactor.SetSlot(account, key, value)
	require.NoError(t, err)

	// Get the slot
	retrievedValue, err := interactor.GetSlot(account, key)
	require.NoError(t, err)
	assert.Equal(t, value, retrievedValue)

	// Verify storage operations were recorded
	ops := interactor.GetStorageOps()
	require.Len(t, ops, 2) // One set, one get operation

	// Verify set operation
	assert.False(t, ops[0].IsRead)
	assert.Equal(t, key, ops[0].Key)
	assert.Equal(t, types.Uint256(*value.Uint256()), ops[0].Value)
	// assert.True(t, value.Uint256().Eq(ops[0].Value.Int())) // if prev equal does not work as expected
	assert.Equal(t, pc, ops[0].PC)
	assert.Equal(t, uint(0), ops[0].RwIdx)
	assert.Equal(t, msgId, ops[0].MsgId)

	// Verify get operation
	assert.True(t, ops[1].IsRead)
	assert.Equal(t, key, ops[1].Key)
	assert.Equal(t, types.Uint256(*value.Uint256()), ops[1].Value)
	assert.Equal(t, pc, ops[1].PC)
	assert.Equal(t, uint(1), ops[1].RwIdx)
	assert.Equal(t, msgId, ops[1].MsgId)
}

func TestStorageOpTracer_GetSlotNonExistent(t *testing.T) {
	t.Parallel()

	account, mptTracer := mpttracer.CreateTestAccount(t)
	rwCounter := &RwCounter{}
	interactor := NewStorageOpTracer(mptTracer, rwCounter,
		func() (uint64, error) { return 100, nil }, 1,
	)

	// Try to get a non-existent slot
	key := common.BytesToHash([]byte("non_existent_key"))
	value, err := interactor.GetSlot(account, key)
	require.NoError(t, err)
	assert.Equal(t, common.EmptyHash, value)
}
