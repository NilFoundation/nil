package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
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
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	shardId := types.ShardId(5)
	es, err := NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	addr := types.GenerateRandomAddress(shardId)

	es.CreateAccount(addr)

	storageKey := common.BytesToHash([]byte("storage-key"))

	es.SetState(addr, storageKey, common.IntToHash(123456))

	const numMessages types.Seqno = 10

	// constructor that generates the code "01020304"
	code := "6004600c60003960046000f301020304"

	from := types.GenerateRandomAddress(shardId)

	for i := range numMessages {
		deploy := types.BuildDeployPayload(hexutil.FromHex(code), common.BytesToHash([]byte{byte(i)}))

		msg := &types.Message{
			Data:     deploy.Bytes(),
			From:     from,
			Seqno:    i,
			GasLimit: *types.NewUint256(10000),
			To:       types.CreateAddress(shardId, deploy.Bytes()),
		}
		es.AddInMessage(msg)
		_, err = es.HandleDeployMessage(ctx, msg)
		suite.Require().NoError(err)
	}

	blockHash, err := es.Commit(0)
	suite.Require().NoError(err)

	es, err = NewExecutionState(tx, shardId, blockHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	storageVal := es.GetState(addr, storageKey)

	suite.Equal(storageVal, common.IntToHash(123456))

	block, err := db.ReadBlock(tx, shardId, blockHash)
	suite.Require().NoError(err)
	suite.Require().NotNil(block)

	messageTrieTable := db.MessageTrieTable
	receiptTrieTable := db.ReceiptTrieTable
	messagesRoot := NewMessageTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, es.ShardId, messageTrieTable, block.InMessagesRoot))
	receiptsRoot := NewReceiptTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, es.ShardId, receiptTrieTable, block.ReceiptsRoot))
	var messageIndex types.MessageIndex

	for {
		m, err := messagesRoot.Fetch(messageIndex)
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		}
		suite.Require().NoError(err)

		deploy := types.BuildDeployPayload(hexutil.FromHex(code), common.BytesToHash([]byte{byte(messageIndex)}))
		suite.Equal(types.Code(deploy.Bytes()), m.Data)

		_, err = receiptsRoot.Fetch(messageIndex)
		suite.Require().NoError(err)

		messageIndex++
	}
	suite.Equal(types.MessageIndex(numMessages), messageIndex)
	suite.Require().NoError(tx.Commit())
}

func (suite *SuiteExecutionState) TestDeployAndCall() {
	shardId := types.ShardId(5)

	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	code, err := contracts.GetCode("tests/Counter")
	suite.Require().NoError(err)

	addrWallet := types.CreateAddress(shardId, code)

	es, err := NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	deployMsg := types.BuildDeployPayload(code, common.EmptyHash)
	message := &types.Message{
		Internal: true,
		Kind:     types.DeployMessageKind,
		Seqno:    1,
		GasLimit: types.Uint256{Int: *uint256.NewInt(100000)},
		To:       addrWallet,
		Data:     types.Code(deployMsg),
	}

	suite.EqualValues(0, es.GetSeqno(addrWallet))

	es.AddInMessage(message)
	_, err = es.HandleDeployMessage(ctx, message)
	suite.Require().NoError(err)

	// Check that initially seqno is 1
	suite.EqualValues(1, es.GetSeqno(addrWallet))

	abiCalee, err := contracts.GetAbi("tests/Counter")
	suite.Require().NoError(err)
	calldata, err := abiCalee.Pack("add", int32(11))
	suite.Require().NoError(err)
	messageToSend := &types.Message{
		Data:     calldata,
		Seqno:    1,
		From:     addrWallet,
		To:       addrWallet,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
	}
	_, _, err = es.HandleExecutionMessage(ctx, messageToSend)
	suite.Require().NoError(err)

	// Check that seqno is increased
	suite.EqualValues(2, es.GetSeqno(addrWallet))
}

func (suite *SuiteExecutionState) TestExecStateMultipleBlocks() {
	tx, err := suite.db.CreateRwTx(context.Background())
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0))
	suite.Require().NoError(err)

	msg1 := &types.Message{Data: []byte{1}, Seqno: 1}
	msg2 := &types.Message{Data: []byte{2}, Seqno: 2}

	es.AddInMessage(msg1)
	es.AddReceipt(0, nil)
	blockHash1, err := es.Commit(0)
	suite.Require().NoError(err)

	es, err = NewExecutionState(tx, types.BaseShardId, blockHash1, common.NewTestTimer(0))
	suite.Require().NoError(err)

	es.AddInMessage(msg2)
	es.AddReceipt(0, nil)
	blockHash2, err := es.Commit(1)
	suite.Require().NoError(err)

	check := func(blockHash common.Hash, msg *types.Message) {
		block, err := db.ReadBlock(tx, types.BaseShardId, blockHash)
		suite.Require().NoError(err)
		suite.Require().NotNil(block)

		messagesRoot := NewMessageTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, es.ShardId, db.MessageTrieTable, block.InMessagesRoot))
		msgRead, err := messagesRoot.Fetch(0)
		suite.Require().NoError(err)

		if len(msgRead.Signature) == 0 {
			msgRead.Signature = nil
		}
		suite.Equal(msg, msgRead)
	}

	check(blockHash1, msg1)
	check(blockHash2, msg2)
	suite.Require().NoError(tx.Commit())
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

	err = state.GenerateZeroState(DefaultZeroStateConfig)
	require.NoError(t, err)
	return state
}

func TestStorage(t *testing.T) {
	t.Parallel()

	state := newState(t)
	account := types.GenerateRandomAddress(types.BaseShardId)
	key := common.EmptyHash
	value := common.IntToHash(42)

	num := state.GetState(account, key)
	require.Equal(t, num, common.EmptyHash)

	require.False(t, state.accountExists(account))

	state.CreateAccount(account)

	require.True(t, state.accountExists(account))

	state.SetState(account, key, value)

	num = state.GetState(account, key)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	t.Parallel()

	state := newState(t)
	account := types.GenerateRandomAddress(types.BaseShardId)

	state.SetBalance(account, *uint256.NewInt(100500))

	require.Equal(t, *state.GetBalance(account), *uint256.NewInt(100500))
}

func TestSnapshot(t *testing.T) {
	t.Parallel()
	stateobjaddr := types.GenerateRandomAddress(types.BaseShardId)
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
	addr := types.GenerateRandomAddress(types.BaseShardId)
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

func TestAccountState(t *testing.T) {
	t.Parallel()
	state := newState(t)
	addr := types.GenerateRandomAddress(types.BaseShardId)

	state.CreateAccount(addr)

	balance := *uint256.NewInt(42)
	acc := state.GetAccount(addr)
	acc.SetBalance(balance)
	acc.SetSeqno(43)
	code := types.Code([]byte{'c', 'a', 'f', 'e'})
	acc.SetCode(code.Hash(), code)

	_, err := state.Commit(0)
	require.NoError(t, err)

	// Drop local state account cache
	delete(state.Accounts, addr)

	acc = state.GetAccount(addr)
	require.NotNil(t, acc)
	assert.Equal(t, balance, acc.Balance)
}
