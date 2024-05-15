package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/ssz"
	"github.com/NilFoundation/nil/core/types"
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
	tx, err := suite.db.CreateRwTx(context.Background())
	suite.Require().NoError(err)

	es, err := NewExecutionState(tx, 0, common.EmptyHash)
	suite.Require().NoError(err)

	addr := common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")

	err = es.CreateContract(addr, []byte("asdf"))
	suite.Require().NoError(err)

	storageKey := common.BytesToHash([]byte("storage-key"))

	err = es.SetState(addr, storageKey, common.IntToHash(123456))
	suite.Require().NoError(err)

	const numMessages uint8 = 10

	for i := range numMessages {
		msg := types.Message{ShardInfo: types.Shard{Id: 10}, Data: []byte{i}}
		es.AddMessage(&msg)
	}

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)

	es, err = NewExecutionState(tx, 0, blockHash)
	suite.Require().NoError(err)

	storageVal := es.GetState(addr, storageKey)

	suite.Equal(storageVal, common.IntToHash(123456))

	block := db.ReadBlock(tx, blockHash)
	suite.Require().NotNil(block)

	messagesRoot := mpt.NewMerklePatriciaTrieWithRoot(tx, db.MessageTrieTable, block.MessagesRoot)
	var messageIndex uint64 = 0

	for {
		k, err := ssz.MarshalSSZ(nil, messageIndex)
		suite.Require().NoError(err)

		mRaw, err := messagesRoot.Get(k)

		if errors.Is(err, db.ErrKeyNotFound) {
			break
		}
		var m types.Message
		suite.Require().NoError(m.DecodeSSZ(mRaw, 0))
		suite.Equal(types.Code{byte(messageIndex)}, m.Data)
		messageIndex += 1
	}
	suite.Equal(numMessages, uint8(messageIndex))
}

func TestSuiteExecutionState(t *testing.T) {
	suite.Run(t, new(SuiteExecutionState))
}

func newState(t *testing.T) *ExecutionState {
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(context.Background())
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

	require.False(t, state.ContractExists(account))

	err := state.CreateContract(account, nil)
	require.NoError(t, err)

	require.True(t, state.ContractExists(account))

	err = state.SetState(account, key, value)
	require.NoError(t, err)

	num = state.GetState(account, key)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	state := newState(t)
	account := common.HexToAddress("deadbeef")

	state.SetBalance(account, *uint256.NewInt(100500))

	require.Equal(t, state.GetBalance(account), *uint256.NewInt(100500))
}
