package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateTx(context.TODO())
	require.NoError(t, err)
	state, err := NewExecutionState(tx, common.EmptyHash)
	require.NoError(t, err)

	account := common.HexToAddress("deadbeef")
	key := common.EmptyHash
	value := *uint256.MustFromHex("0x42")

	num, err := state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, uint256.Int{})

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

	num, err = state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, value)
}
