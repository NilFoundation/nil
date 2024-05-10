package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func newState(t *testing.T) *ExecutionState {
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateTx(context.TODO())
	require.NoError(t, err)
	state, err := NewExecutionState(tx, common.EmptyHash)
	require.NoError(t, err)
	return state
}

func TestStorage(t *testing.T) {
	state := newState(t)
	account := common.HexToAddress("deadbeef")
	key := common.EmptyHash
	value := common.IntToHash(42)

	num := state.GetState(account, key)
	require.Equal(t, num, common.EmptyHash)

	exists, err := state.ContractExists(account)
	require.NoError(t, err)
	require.False(t, exists)

	err = state.SetState(account, key, value)
	require.Error(t, err)

	err = state.CreateContract(account, make([]byte, 1))
	require.NoError(t, err)

	err = state.SetState(account, key, value)
	require.NoError(t, err)

	exists, err = state.ContractExists(account)
	require.NoError(t, err)
	require.True(t, exists)

	num = state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	state := newState(t)
	account := common.HexToAddress("deadbeef")

	// TODO: CreateContract should not be necessary here
	err := state.CreateContract(account, make([]byte, 1))
	require.NoError(t, err)

	err = state.SetBalance(account, *uint256.NewInt(100500))
	require.NoError(t, err)

	require.Equal(t, state.GetBalance(account), *uint256.NewInt(100500))
}
