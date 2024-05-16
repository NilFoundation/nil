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

	es.SetState(addr, storageKey, common.IntToHash(123456))

	const numMessages uint8 = 10

	from := common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	for i := range numMessages {
		msg := types.Message{ShardId: types.ShardId(10), Data: []byte{i}, From: from, Seqno: uint64(i)}
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

	messageTrieTable := db.MessageTrieTableName(0)
	receiptTrieTable := db.ReceiptTrieTableName(0)
	messagesRoot := mpt.NewMerklePatriciaTrieWithRoot(tx, messageTrieTable, block.MessagesRoot)
	receiptsRoot := mpt.NewMerklePatriciaTrieWithRoot(tx, receiptTrieTable, block.ReceiptsRoot)
	var messageIndex uint64 = 0

	for {
		k, err := ssz.MarshalSSZ(nil, messageIndex)
		suite.Require().NoError(err)

		mRaw, err := messagesRoot.Get(k)
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		} else if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to get message %v from trie", messageIndex)
		}

		rRaw, err := receiptsRoot.Get(k)
		if err != nil {
			logger.Fatal().Err(err).Msgf("Failed to get receipt %v from trie", messageIndex)
		}

		var m types.Message
		suite.Require().NoError(m.DecodeSSZ(mRaw, 0))
		suite.Equal(types.Code{byte(messageIndex)}, m.Data)

		var r types.Receipt
		suite.Require().NoError(r.UnmarshalSSZ(rRaw))
		suite.Equal(messageIndex, r.MsgIndex)
		suite.NotZero(len(r.ContractAddress))

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

	state.SetState(account, key, value)

	num = state.GetState(account, key)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	state := newState(t)
	account := common.HexToAddress("deadbeef")

	state.SetBalance(account, *uint256.NewInt(100500))

	require.Equal(t, *state.GetBalance(account), *uint256.NewInt(100500))
}
