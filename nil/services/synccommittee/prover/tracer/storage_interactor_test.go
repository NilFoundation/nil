package tracer

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test account with storage trie
func createTestAccount(t *testing.T) *Account {
	t.Helper()

	ctx := context.Background()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)

	rwTx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)

	storageTrie := execution.NewDbStorageTrie(rwTx, types.ShardId(0))

	return &Account{
		Address:     types.Address{1, 2, 3}, // example address
		StorageTrie: storageTrie,
		Balance:     types.Value{},
		Code:        types.Code{},
		Seqno:       0,
		ExtSeqno:    0,
	}
}

func TestMessageStorageInteractor_SetAndGetSlot(t *testing.T) {
	t.Parallel()

	account := createTestAccount(t)
	rwCounter := &RwCounter{}
	pc := uint64(100)
	msgId := uint(1)

	interactor := NewMessageStorageInteractor(rwCounter, func() uint64 { return pc }, msgId)

	// Test setting and getting a slot
	key := common.BytesToHash([]byte("test_key"))
	value := common.BytesToHash([]byte("test_value"))

	// Set the slot
	err := interactor.SetSlot(account, key, value)
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

func TestMessageStorageInteractor_GetSlotNonExistent(t *testing.T) {
	t.Parallel()

	account := createTestAccount(t)
	rwCounter := &RwCounter{}
	interactor := NewMessageStorageInteractor(rwCounter, func() uint64 { return 100 }, 1)

	// Try to get a non-existent slot
	key := common.BytesToHash([]byte("non_existent_key"))
	value, err := interactor.GetSlot(account, key)
	require.NoError(t, err)
	assert.Equal(t, common.EmptyHash, value)
}

func TestMessageStorageInteractor_GetAccountSlotChangeTraces(t *testing.T) {
	t.Parallel()

	account := createTestAccount(t)
	rwCounter := &RwCounter{}
	interactor := NewMessageStorageInteractor(rwCounter, func() uint64 { return 100 }, 1)

	// Set multiple slots
	key1 := common.BytesToHash([]byte("key1"))
	value1 := common.BytesToHash([]byte("value1"))
	key2 := common.BytesToHash([]byte("key2"))
	value2 := common.BytesToHash([]byte("value2"))

	err := interactor.SetSlot(account, key1, value1)
	require.NoError(t, err)
	err = interactor.SetSlot(account, key2, value2)
	require.NoError(t, err)

	// Get traces
	traces, err := interactor.GetAccountSlotChangeTraces(account)
	require.NoError(t, err)
	require.NotNil(t, traces)

	// Verify traces contain both changes
	assert.Len(t, traces, 2)

	// Verify affected accounts
	affectedAccounts := interactor.GetAffectedAccountsAddresses()
	assert.Len(t, affectedAccounts, 1)
	assert.Equal(t, account.Address, affectedAccounts[0])
}

func TestMessageStorageInteractor_MultipleUpdatesToSameSlot(t *testing.T) {
	t.Parallel()

	account := createTestAccount(t)
	rwCounter := &RwCounter{}
	interactor := NewMessageStorageInteractor(rwCounter, func() uint64 { return 100 }, 1)

	key := common.BytesToHash([]byte("test_key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	// Set slot multiple times
	err := interactor.SetSlot(account, key, value1)
	require.NoError(t, err)
	err = interactor.SetSlot(account, key, value2)
	require.NoError(t, err)

	// Verify final value
	retrievedValue, err := interactor.GetSlot(account, key)
	require.NoError(t, err)
	assert.Equal(t, value2, retrievedValue)

	// Verify operations were recorded
	ops := interactor.GetStorageOps()
	assert.Len(t, ops, 3) // Two sets and one get

	// Get traces
	traces, err := interactor.GetAccountSlotChangeTraces(account)
	require.NoError(t, err)
	require.Len(t, traces, 1) // Should only have one trace for the key

	// Verify the trace shows the final state
	assert.Equal(t, types.Uint256(*value2.Uint256()), traces[0].ValueAfter)
}
