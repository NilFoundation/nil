package mpttracer

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMPTTracer_GetAccountSlotChangeTraces(t *testing.T) {
	t.Parallel()

	account, mptTracer := CreateTestAccount(t)

	// Set multiple slots
	key1 := common.BytesToHash([]byte("key1"))
	value1 := common.BytesToHash([]byte("value1"))
	key2 := common.BytesToHash([]byte("key2"))
	value2 := common.BytesToHash([]byte("value2"))

	err := mptTracer.SetSlot(account, key1, value1)
	require.NoError(t, err)
	err = mptTracer.SetSlot(account, key2, value2)
	require.NoError(t, err)

	accountTrieTraces, err := mptTracer.GetAccountTrieTraces()
	require.NoError(t, err)
	require.NotNil(t, accountTrieTraces)

	// Verify ContractTrie traces contain single change
	assert.Len(t, accountTrieTraces, 1)

	// Verify both slots are included into trace for specific address
	storageTracesByAccount, err := mptTracer.GetAccountsStorageUpdatesTraces()
	require.NoError(t, err)
	assert.Len(t, storageTracesByAccount, 1)
	accountStorageTraces, exists := storageTracesByAccount[account]
	assert.True(t, exists)
	assert.Len(t, accountStorageTraces, 2)
}

func TestMPTTracer_MultipleUpdatesToSameSlot(t *testing.T) {
	t.Parallel()

	account, mptTracer := CreateTestAccount(t)

	key := common.BytesToHash([]byte("test_key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	// Set slot multiple times
	err := mptTracer.SetSlot(account, key, value1)
	require.NoError(t, err)
	err = mptTracer.SetSlot(account, key, value2)
	require.NoError(t, err)

	// Verify final value
	retrievedValue, err := mptTracer.GetSlot(account, key)
	require.NoError(t, err)
	assert.Equal(t, value2, retrievedValue)

	// Verify only one operation was recorded
	storageTracesByAccount, err := mptTracer.GetAccountsStorageUpdatesTraces()
	require.NoError(t, err)
	assert.Len(t, storageTracesByAccount, 1)
	accountStorageTraces, exists := storageTracesByAccount[account]
	assert.True(t, exists)
	assert.Len(t, accountStorageTraces, 1)

	// Verify the trace shows the final state
	assert.Equal(t, types.Uint256(*value2.Uint256()), accountStorageTraces[0].ValueAfter)
}
