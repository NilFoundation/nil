package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	ssz "github.com/ferranbt/fastssz"
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

func (s *SuiteExecutionState) TearDownTest() {
	s.db.Close()
}

func (suite *SuiteExecutionState) TestExecState() {
	tx, err := suite.db.CreateRwTx(context.Background())
	suite.Require().NoError(err)

	shardId := types.ShardId(5)
	es, err := NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	addr := types.GenerateRandomAddress(shardId)

	es.CreateAccount(addr)

	storageKey := common.BytesToHash([]byte("storage-key"))

	es.SetState(addr, storageKey, common.IntToHash(123456))

	const numMessages uint8 = 10

	from := types.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	blockContext := NewEVMBlockContext(es)
	for i := range numMessages {
		deploy := types.DeployMessage{
			ShardId: shardId,
			Seqno:   uint64(i),

			// constructor that generates the code "01020304"
			Code: hexutil.FromHex("6004600c60003960046000f301020304"),
		}
		data, err := deploy.MarshalSSZ()
		suite.Require().NoError(err)

		msg := types.Message{Data: data, From: from, Seqno: uint64(i)}
		es.AddInMessage(&msg)
		suite.Require().NoError(es.HandleDeployMessage(&msg, &blockContext))
	}

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)

	es, err = NewExecutionState(tx, shardId, blockHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	storageVal := es.GetState(addr, storageKey)

	suite.Equal(storageVal, common.IntToHash(123456))

	block := db.ReadBlock(tx, shardId, blockHash)
	suite.Require().NotNil(block)

	messageTrieTable := db.MessageTrieTable
	receiptTrieTable := db.ReceiptTrieTable
	messagesRoot := mpt.NewMerklePatriciaTrieWithRoot(tx, es.ShardId, messageTrieTable, block.InMessagesRoot)
	receiptsRoot := mpt.NewMerklePatriciaTrieWithRoot(tx, es.ShardId, receiptTrieTable, block.ReceiptsRoot)
	var messageIndex uint64 = 0

	for {
		k := ssz.MarshalUint64(nil, messageIndex)
		suite.Require().NoError(err)

		mRaw, err := messagesRoot.Get(k)
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		}
		suite.Require().NoError(err)

		rRaw, err := receiptsRoot.Get(k)
		suite.Require().NoError(err)

		var m types.Message
		suite.Require().NoError(m.UnmarshalSSZ(mRaw))

		deploy := types.DeployMessage{
			ShardId: shardId,
			Seqno:   messageIndex,
			// constructor that generates the code "01020304"
			Code: hexutil.FromHex("6004600c60003960046000f301020304"),
		}
		data, err := deploy.MarshalSSZ()
		suite.Require().NoError(err)
		suite.Equal(types.Code(data), m.Data)

		var r types.Receipt
		suite.Require().NoError(r.UnmarshalSSZ(rRaw))
		suite.NotZero(len(r.ContractAddress))

		messageIndex += 1
	}
	suite.Equal(numMessages, uint8(messageIndex))
}

func (suite *SuiteExecutionState) TestExecStateMultipleBlocks() {
	tx, err := suite.db.CreateRwTx(context.Background())
	suite.Require().NoError(err)

	es, err := NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	msg1 := types.Message{Data: []byte{1}, Seqno: uint64(1)}
	msg2 := types.Message{Data: []byte{2}, Seqno: uint64(2)}

	es.AddInMessage(&msg1)
	blockHash1, err := es.Commit(0)
	suite.Require().NoError(err)

	es, err = NewExecutionState(tx, types.BaseShardId, blockHash1, common.NewTestTimer(0))
	suite.Require().NoError(err)

	es.AddInMessage(&msg2)
	blockHash2, err := es.Commit(1)
	suite.Require().NoError(err)

	check := func(blockHash common.Hash, msg *types.Message) {
		block := db.ReadBlock(tx, types.BaseShardId, blockHash)
		suite.Require().NotNil(block)

		messagesRoot := mpt.NewMerklePatriciaTrieWithRoot(tx, es.ShardId, db.MessageTrieTable, block.InMessagesRoot)
		var msgRead types.Message

		msgRaw, err := messagesRoot.Get(ssz.MarshalUint64(nil, 0))
		suite.Require().NoError(err)
		suite.Require().NoError(msgRead.UnmarshalSSZ(msgRaw))

		suite.Equal(*msg, msgRead)
	}

	check(blockHash1, &msg1)
	check(blockHash2, &msg2)
}

func TestSuiteExecutionState(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteExecutionState))
}

func newState(t *testing.T) *ExecutionState {
	t.Helper()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	tx, err := database.CreateRwTx(context.Background())
	require.NoError(t, err)
	state, err := NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0))
	require.NoError(t, err)

	err = state.GenerateZeroState(context.Background())
	require.NoError(t, err)
	return state
}

func TestStorage(t *testing.T) {
	t.Parallel()

	state := newState(t)
	account := types.HexToAddress("deadbeef")
	key := common.EmptyHash
	value := common.IntToHash(42)

	num := state.GetState(account, key)
	require.Equal(t, num, common.EmptyHash)

	require.False(t, state.ContractExists(account))

	state.CreateAccount(account)

	require.True(t, state.ContractExists(account))

	state.SetState(account, key, value)

	num = state.GetState(account, key)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	t.Parallel()

	state := newState(t)
	account := types.HexToAddress("deadbeef")

	state.SetBalance(account, *uint256.NewInt(100500))

	require.Equal(t, *state.GetBalance(account), *uint256.NewInt(100500))
}

func TestSnapshot(t *testing.T) {
	t.Parallel()
	stateobjaddr := types.BytesToAddress([]byte("aa"))
	var storageaddr common.Hash
	data1 := common.BytesToHash([]byte{42})
	data2 := common.BytesToHash([]byte{43})
	s := newState(t)

	// snapshot the genesis state
	genesis := s.Snapshot()

	// set initial state object value
	s.SetState(stateobjaddr, storageaddr, data1)
	snapshot := s.Snapshot()

	// set a new state object value, revert it and ensure correct content
	s.SetState(stateobjaddr, storageaddr, data2)
	s.RevertToSnapshot(snapshot)

	if v := s.GetState(stateobjaddr, storageaddr); v != data1 {
		t.Errorf("wrong storage value %v, want %v", v, data1)
	}
	if v := s.GetCommittedState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong committed storage value %v, want %v", v, common.Hash{})
	}

	// revert up to the genesis state and ensure correct content
	s.RevertToSnapshot(genesis)
	if v := s.GetState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong storage value %v, want %v", v, common.Hash{})
	}
	if v := s.GetCommittedState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong committed storage value %v, want %v", v, common.Hash{})
	}
}

func TestSnapshotEmpty(t *testing.T) {
	t.Parallel()
	s := newState(t)
	s.RevertToSnapshot(s.Snapshot())
}

func TestCreateObjectRevert(t *testing.T) {
	t.Parallel()
	state := newState(t)
	addr := types.BytesToAddress([]byte("so0"))
	snap := state.Snapshot()

	state.CreateAccount(addr)

	so0 := state.GetAccount(addr)
	so0.SetBalance(*uint256.NewInt(42))
	so0.SetSeqno(43)
	code := types.Code([]byte{'c', 'a', 'f', 'e'})
	so0.SetCode(code.Hash(), code)
	state.setAccountObject(so0)

	state.RevertToSnapshot(snap)
	if state.Exist(addr) {
		t.Error("Unexpected account after revert")
	}
}
