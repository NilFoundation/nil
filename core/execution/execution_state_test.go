package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteExecutionState struct {
	suite.Suite
	db db.DB
}

func (suite *SuiteExecutionState) SetupTest() {
	var err error
	suite.db, err = db.NewBadgerDb(suite.Suite.T().TempDir() + "test.db")
	suite.Require().NoError(err)
}

func (suite *SuiteExecutionState) TestExecState() {
	tx, err := suite.db.CreateTx(context.Background())
	suite.Require().NoError(err)

	es, err := NewExecutionState(tx, 0, common.EmptyHash)
	suite.Require().NoError(err)

	addr := common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")

	err = es.CreateContract(addr, []byte("asdf"))
	suite.Require().NoError(err)

	storageKey := common.BytesToHash([]byte("storage-key"))

	err = es.SetState(addr, storageKey, common.IntToHash(123456))
	suite.Require().NoError(err)

	blockHash, err := es.Commit()
	suite.Require().NoError(err)

	es, err = NewExecutionState(tx, 0, blockHash)
	suite.Require().NoError(err)

	storageVal := es.GetState(addr, storageKey)

	suite.Equal(storageVal, common.IntToHash(123456))
}

func TestSuiteExecutionState(t *testing.T) {
	suite.Run(t, new(SuiteExecutionState))
}

func newState(t *testing.T) *ExecutionState {
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateTx(context.Background())
	require.NoError(t, err)
	state, err := NewExecutionState(tx, 0, common.EmptyHash)
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

	// FIXME: CreateContract should not be necessary here
	err := state.CreateContract(account, make([]byte, 1))
	require.NoError(t, err)

	err = state.SetBalance(account, *uint256.NewInt(100500))
	require.NoError(t, err)

	require.Equal(t, state.GetBalance(account), *uint256.NewInt(100500))
}
