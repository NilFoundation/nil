package mpttracer

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAccountExecutionState struct {
	rwTx db.RwTx
}

func (es *MockAccountExecutionState) AppendToJournal(entry execution.JournalEntry) {}
func (es *MockAccountExecutionState) GetRwTx() db.RwTx {
	return es.rwTx
}

var _ execution.IAccountExecutionState = (*MockAccountExecutionState)(nil)

func TestMPTTracer_GetAccountSlotChangeTraces(t *testing.T) {
	t.Parallel()

	addr, mptTracer, rwTx := CreateTestAccountAndTracer(t)

	// Set multiple slots
	key1 := common.BytesToHash([]byte("key1"))
	value1 := common.BytesToHash([]byte("value1"))
	key2 := common.BytesToHash([]byte("key2"))
	value2 := common.BytesToHash([]byte("value2"))

	contract, err := mptTracer.GetContract(addr)
	require.NoError(t, err)
	acc, err := execution.NewAccountState(
		&MockAccountExecutionState{rwTx}, contract.Address, contract, logging.NewLogger("test"),
	)
	require.NoError(t, err)

	err = acc.SetState(key1, value1)
	require.NoError(t, err)
	err = acc.SetState(key2, value2)
	require.NoError(t, err)

	// To get correct result from GetMPTTraces we need to commit account with UpdateContracts, not acc.Commit()
	err = mptTracer.UpdateContracts(map[types.Address]*execution.AccountState{contract.Address: acc})
	require.NoError(t, err)

	mptTraces, err := mptTracer.GetMPTTraces()
	require.NoError(t, err)
	require.NotNil(t, mptTraces)
	require.NotNil(t, mptTraces.ContractTrieTraces)
	require.NotNil(t, mptTraces.StorageTracesByAccount)

	// Verify ContractTrie traces contain single change
	assert.Len(t, mptTraces.ContractTrieTraces, 1)

	// Verify both slots are included into trace for specific address
	require.NoError(t, err)
	assert.Len(t, mptTraces.StorageTracesByAccount, 1)
	accountStorageTraces, exists := mptTraces.StorageTracesByAccount[addr]
	assert.True(t, exists)
	assert.Len(t, accountStorageTraces, 2)
}

func TestMPTTracer_MultipleUpdatesToSameSlot(t *testing.T) {
	t.Parallel()

	addr, mptTracer, rwTx := CreateTestAccountAndTracer(t)

	key := common.BytesToHash([]byte("test_key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	// Set slot multiple times
	contract, err := mptTracer.GetContract(addr)
	require.NoError(t, err)
	acc, err := execution.NewAccountState(
		&MockAccountExecutionState{rwTx}, contract.Address, contract, logging.NewLogger("test"),
	)
	require.NoError(t, err)
	err = acc.SetState(key, value1)
	require.NoError(t, err)
	err = acc.SetState(key, value2)
	require.NoError(t, err)

	// Verify final value
	retrievedValue, err := acc.GetState(key)
	require.NoError(t, err)
	assert.Equal(t, value2, retrievedValue)

	initialStorageRoot := acc.StorageTree.RootHash()
	committedContract, err := acc.Commit()
	require.NoError(t, err)

	// Verify only one operation was recorded
	storageTraces, err := mptTracer.getStorageTraces(initialStorageRoot, committedContract.StorageRoot)
	require.NoError(t, err)
	assert.Len(t, storageTraces, 1)

	// Verify the trace shows the final state
	assert.Equal(t, (*types.Uint256)(value2.Uint256()), storageTraces[0].ValueAfter)
}
