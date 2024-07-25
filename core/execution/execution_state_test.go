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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteExecutionState struct {
	suite.Suite

	ctx context.Context
	db  db.DB
}

func (suite *SuiteExecutionState) SetupSuite() {
	suite.ctx = context.Background()
}

func (suite *SuiteExecutionState) SetupTest() {
	var err error
	suite.db, err = db.NewBadgerDb(suite.Suite.T().TempDir() + "test.db")
	suite.Require().NoError(err)
}

func (suite *SuiteExecutionState) TearDownTest() {
	suite.db.Close()
}

func (suite *SuiteExecutionState) TestExecState() {
	const shardId types.ShardId = 5
	const numMessages types.Seqno = 10
	const code = "6004600c60003960046000f301020304"

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	addr := types.GenerateRandomAddress(shardId)
	storageKey := common.BytesToHash([]byte("storage-key"))

	es, err := NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0), 1)
	suite.Require().NoError(err)

	suite.Run("CreateAccount", func() {
		suite.Require().NoError(es.CreateAccount(addr))
		suite.Require().NoError(es.SetState(addr, storageKey, common.IntToHash(123456)))
	})

	suite.Run("DeployMessages", func() {
		from := types.GenerateRandomAddress(shardId)
		for i := range numMessages {
			Deploy(suite.T(), suite.ctx, es,
				types.BuildDeployPayload(hexutil.FromHex(code), common.BytesToHash([]byte{byte(i)})),
				shardId, from, i)
		}
	})

	var blockHash common.Hash

	suite.Run("CommitBlock", func() {
		blockHash, err = es.Commit(0)
		suite.Require().NoError(err)
	})

	suite.Run("CheckAccount", func() {
		es, err := NewExecutionState(tx, shardId, blockHash, common.NewTestTimer(0), 1)
		suite.Require().NoError(err)

		storageVal, err := es.GetState(addr, storageKey)
		suite.Require().NoError(err)
		suite.Equal(storageVal, common.IntToHash(123456))
	})

	suite.Run("CheckMessages", func() {
		block, err := db.ReadBlock(tx, shardId, blockHash)
		suite.Require().NoError(err)
		suite.Require().NotNil(block)

		messagesRoot := NewMessageTrieReader(mpt.NewReaderWithRoot(tx, es.ShardId, db.MessageTrieTable, block.InMessagesRoot))
		receiptsRoot := NewReceiptTrieReader(mpt.NewReaderWithRoot(tx, es.ShardId, db.ReceiptTrieTable, block.ReceiptsRoot))
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
	})

	suite.Run("CommitTx", func() {
		suite.Require().NoError(tx.Commit())
	})
}

func (suite *SuiteExecutionState) TestDeployAndCall() {
	shardId := types.ShardId(5)

	payload := contracts.CounterDeployPayload(suite.T())
	addrWallet := types.CreateAddress(shardId, payload)

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0), 1)
	suite.Require().NoError(err)

	suite.Run("Deploy", func() {
		seqno, err := es.GetSeqno(addrWallet)
		suite.Require().NoError(err)
		suite.EqualValues(0, seqno)

		Deploy(suite.T(), suite.ctx, es, payload, shardId, types.Address{}, 0)

		seqno, err = es.GetSeqno(addrWallet)
		suite.Require().NoError(err)
		suite.EqualValues(1, seqno)
	})

	suite.Run("Execute", func() {
		_, _, err := es.HandleExecutionMessage(suite.ctx, NewExecutionMessage(addrWallet, addrWallet, 1,
			contracts.NewCounterAddCallData(suite.T(), 47)))
		suite.Require().NoError(err)

		seqno, err := es.GetSeqno(addrWallet)
		suite.Require().NoError(err)
		suite.EqualValues(1, seqno)

		extSeqno, err := es.GetExtSeqno(addrWallet)
		suite.Require().NoError(err)
		suite.EqualValues(1, extSeqno)
	})
}

func (suite *SuiteExecutionState) TestExecStateMultipleBlocks() {
	msg1 := types.NewEmptyMessage()
	msg1.Data = []byte{1}
	msg1.Seqno = 1
	msg2 := types.NewEmptyMessage()
	msg2.Data = []byte{2}
	msg2.Seqno = 2
	blockHash1 := GenerateBlockFromMessagesWithoutExecution(suite.T(), context.Background(),
		types.BaseShardId, 0, common.EmptyHash, suite.db, msg1, msg2)
	blockHash2 := GenerateBlockFromMessagesWithoutExecution(suite.T(), context.Background(),
		types.BaseShardId, 1, blockHash1, suite.db, msg2)

	tx, err := suite.db.CreateRoTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	check := func(blockHash common.Hash, idx types.MessageIndex, msg *types.Message) {
		block, err := db.ReadBlock(tx, types.BaseShardId, blockHash)
		suite.Require().NoError(err)
		suite.Require().NotNil(block)

		messagesRoot := NewMessageTrieReader(mpt.NewReaderWithRoot(tx, types.BaseShardId, db.MessageTrieTable, block.InMessagesRoot))
		msgRead, err := messagesRoot.Fetch(idx)
		suite.Require().NoError(err)

		suite.EqualValues(msg, msgRead)
	}

	check(blockHash1, 0, msg1)
	check(blockHash1, 1, msg2)
	check(blockHash2, 0, msg2)
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
	state, err := NewExecutionState(tx, types.BaseShardId, common.EmptyHash, common.NewTestTimer(0), 1)
	require.NoError(t, err)

	err = state.GenerateZeroState(DefaultZeroStateConfig)
	require.NoError(t, err)
	return state
}

func TestStorage(t *testing.T) {
	t.Parallel()

	state := newState(t)
	defer state.tx.Rollback()

	account := types.GenerateRandomAddress(types.BaseShardId)
	key := common.EmptyHash
	value := common.IntToHash(42)

	num, err := state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, common.EmptyHash)

	exists, err := state.Exists(account)
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, state.CreateAccount(account))

	exists, err = state.Exists(account)
	require.NoError(t, err)
	require.True(t, exists)

	require.NoError(t, state.SetState(account, key, value))

	num, err = state.GetState(account, key)
	require.NoError(t, err)
	require.Equal(t, num, value)
}

func TestBalance(t *testing.T) {
	t.Parallel()

	state := newState(t)
	defer state.tx.Rollback()
	account := types.GenerateRandomAddress(types.BaseShardId)

	require.NoError(t, state.SetBalance(account, types.NewValueFromUint64(100500)))

	balance, err := state.GetBalance(account)
	require.NoError(t, err)
	require.Equal(t, types.NewValueFromUint64(100500), balance)
}

func TestSnapshot(t *testing.T) {
	t.Parallel()
	stateobjaddr := types.GenerateRandomAddress(types.BaseShardId)
	var storageaddr common.Hash
	data1 := common.BytesToHash([]byte{42})
	data2 := common.BytesToHash([]byte{43})
	s := newState(t)
	defer s.tx.Rollback()

	// snapshot the genesis state
	genesis := s.Snapshot()

	// set initial state object value
	require.NoError(t, s.SetState(stateobjaddr, storageaddr, data1))
	snapshot := s.Snapshot()

	// set a new state object value, revert it and ensure correct content
	require.NoError(t, s.SetState(stateobjaddr, storageaddr, data2))
	s.RevertToSnapshot(snapshot)

	v, err := s.GetState(stateobjaddr, storageaddr)
	require.NoError(t, err)
	assert.Equal(t, data1, v)

	if v := s.GetCommittedState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong committed storage value %v, want %v", v, common.Hash{})
	}

	// revert up to the genesis state and ensure correct content
	s.RevertToSnapshot(genesis)
	v, err = s.GetState(stateobjaddr, storageaddr)
	require.NoError(t, err)
	assert.Empty(t, v)
	if v := s.GetCommittedState(stateobjaddr, storageaddr); v != (common.Hash{}) {
		t.Errorf("wrong committed storage value %v, want %v", v, common.Hash{})
	}
}

func TestSnapshotEmpty(t *testing.T) {
	t.Parallel()
	s := newState(t)
	defer s.tx.Rollback()
	s.RevertToSnapshot(s.Snapshot())
}

func TestCreateObjectRevert(t *testing.T) {
	t.Parallel()
	state := newState(t)
	defer state.tx.Rollback()
	addr := types.GenerateRandomAddress(types.BaseShardId)
	snap := state.Snapshot()

	require.NoError(t, state.CreateAccount(addr))

	so0, err := state.GetAccount(addr)
	require.NoError(t, err)
	so0.SetBalance(types.NewValueFromUint64(42))
	so0.SetSeqno(43)
	code := types.Code([]byte{'c', 'a', 'f', 'e'})
	so0.SetCode(code.Hash(), code)
	state.setAccountObject(so0)

	state.RevertToSnapshot(snap)
	exists, err := state.Exists(addr)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAccountState(t *testing.T) {
	t.Parallel()
	state := newState(t)
	defer state.tx.Rollback()
	addr := types.GenerateRandomAddress(types.BaseShardId)

	require.NoError(t, state.CreateAccount(addr))

	balance := types.NewValueFromUint64(42)
	acc, err := state.GetAccount(addr)
	require.NoError(t, err)
	acc.SetBalance(balance)
	acc.SetSeqno(43)
	code := types.Code([]byte{'c', 'a', 'f', 'e'})
	acc.SetCode(code.Hash(), code)

	_, err = state.Commit(0)
	require.NoError(t, err)

	// Drop local state account cache
	delete(state.Accounts, addr)

	acc, err = state.GetAccount(addr)
	require.NoError(t, err)
	require.NotNil(t, acc)
	assert.Equal(t, balance, acc.Balance)
}

func (suite *SuiteExecutionState) TestMessageStatus() {
	shardId := types.ShardId(5)

	payload := contracts.CounterDeployPayload(suite.T())
	addrWallet := types.CreateAddress(shardId, payload)

	tx, err := suite.db.CreateRwTx(suite.ctx)
	suite.Require().NoError(err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, common.EmptyHash, common.NewTestTimer(0), 1)
	suite.Require().NoError(err)

	suite.Run("Deploy", func() {
		seqno, err := es.GetSeqno(addrWallet)
		suite.Require().NoError(err)
		suite.EqualValues(0, seqno)

		Deploy(suite.T(), suite.ctx, es, payload, shardId, types.Address{}, 0)

		seqno, err = es.GetSeqno(addrWallet)
		suite.Require().NoError(err)
		suite.EqualValues(1, seqno)
	})

	suite.Run("ExecuteOutOfGas", func() {
		msg := &types.Message{
			From:      addrWallet,
			To:        addrWallet,
			Data:      contracts.NewCounterAddCallData(suite.T(), 47),
			Seqno:     1,
			FeeCredit: toGasCredit(0),
		}
		_, _, err := es.HandleExecutionMessage(suite.ctx, msg)
		merr := &types.MessageError{}
		suite.Require().ErrorAs(err, &merr)
		suite.EqualValues(types.MessageStatusOutOfGas, merr.Status)
	})

	suite.Run("ExecuteReverted", func() {
		msg := &types.Message{
			From:      addrWallet,
			To:        addrWallet,
			Data:      []byte("wrong calldata"),
			Seqno:     1,
			FeeCredit: toGasCredit(1_000_000),
		}
		_, _, err := es.HandleExecutionMessage(suite.ctx, msg)
		merr := &types.MessageError{}
		suite.Require().ErrorAs(err, &merr)
		suite.EqualValues(types.MessageStatusExecutionReverted, merr.Status)
	})
}
