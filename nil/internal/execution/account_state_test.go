package execution

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

func setupTestAccountState(t *testing.T) (JournaledAccountState, *[]JournalEntry, db.DB, db.RwTx) {
	t.Helper()
	ctx := t.Context()
	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)

	tx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)

	addr := types.ShardAndHexToAddress(1, "0x123")

	code := types.Code{0x12}
	require.NoError(t, db.WriteCode(tx, addr.ShardId(), code.Hash(), code))

	var journalSlice []JournalEntry
	mockAppender := &JournalAppenderMock{
		AppendToJournalFunc: func(entry JournalEntry) {
			journalSlice = append(journalSlice, entry)
		},
	}

	txProvider := &DbRwTxProviderMock{
		GetRwTxFunc: func() db.RwTx { return tx },
	}
	logger := logging.NewLogger("accoun_state_test")

	account := &types.SmartContract{
		Address:  addr,
		Balance:  types.NewValueFromUint64(100),
		Seqno:    5,
		ExtSeqno: 10,
		CodeHash: code.Hash(),
	}

	accountState, err := NewAccountState(txProvider, addr, account, logger)
	require.NoError(t, err)

	journaledState := NewJournaledAccountStateFromRaw(mockAppender, accountState, logger)

	return journaledState, &journalSlice, database, tx
}

func TestJournaledAddBalance(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialBalance := journaledState.GetBalance()
	addAmount := types.NewValueFromUint64(50)

	err := journaledState.JournaledAddBalance(addAmount, tracing.BalanceIncreaseSelfdestruct)
	require.NoError(t, err)

	// Check balance was updated
	newBalance := journaledState.GetBalance()
	expectedBalance, _ := initialBalance.AddOverflow(addAmount)
	require.Equal(t, expectedBalance, newBalance)

	// Check journal entry was created
	require.Len(t, (*journalSlice), 1)
	entry, ok := (*journalSlice)[0].(balanceChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialBalance, entry.prev)
}

func TestJournaledAddBalance_ZeroAmount(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialBalance := journaledState.GetBalance()
	zeroAmount := types.NewValueFromUint64(0)

	err := journaledState.JournaledAddBalance(zeroAmount, tracing.BalanceIncreaseRefund)
	require.NoError(t, err)

	// Balance should remain unchanged
	require.Equal(t, initialBalance, journaledState.GetBalance())

	// No journal entry should be created for zero amount
	require.Empty(t, *journalSlice)
}

func TestJournaledSubBalance(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialBalance := journaledState.GetBalance()
	subAmount := types.NewValueFromUint64(30)

	err := journaledState.JournaledSubBalance(subAmount, tracing.BalanceDecreaseGasBuy)
	require.NoError(t, err)

	// Check balance was updated
	newBalance := journaledState.GetBalance()
	expectedBalance, _ := initialBalance.SubOverflow(subAmount)
	require.Equal(t, expectedBalance, newBalance)

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(balanceChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialBalance, entry.prev)
}

func TestJournaledSubBalance_ZeroAmount(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialBalance := journaledState.GetBalance()
	zeroAmount := types.NewValueFromUint64(0)

	err := journaledState.JournaledSubBalance(zeroAmount, tracing.BalanceDecreaseDaoAccount)
	require.NoError(t, err)

	// Balance should remain unchanged
	require.Equal(t, initialBalance, journaledState.GetBalance())

	// No journal entry should be created for zero amount
	require.Empty(t, *journalSlice)
}

func TestJournaledSetBalance(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialBalance := journaledState.GetBalance()
	newAmount := types.NewValueFromUint64(200)

	journaledState.JournaledSetBalance(newAmount)

	// Check balance was updated
	require.Equal(t, newAmount, journaledState.GetBalance())

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(balanceChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialBalance, entry.prev)
}

func TestJournaledSetState(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	key := common.HexToHash("0x1234")
	value := common.HexToHash("0x5678")

	// Get initial state (should be empty)
	initialValue, err := journaledState.GetState(key)
	require.NoError(t, err)

	err = journaledState.JournaledSetState(key, value)
	require.NoError(t, err)

	// Check state was updated
	newValue, err := journaledState.GetState(key)
	require.NoError(t, err)
	require.Equal(t, value, newValue)

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(storageChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, key, entry.key)
	require.Equal(t, initialValue, entry.prevvalue)
}

func TestJournaledSetState_SameValue(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	key := common.HexToHash("0x1234")
	value := common.HexToHash("0x5678")

	// Set initial value
	err := journaledState.JournaledSetState(key, value)
	require.NoError(t, err)
	require.Len(t, *journalSlice, 1)

	// Try to set the same value again
	err = journaledState.JournaledSetState(key, value)
	require.NoError(t, err)

	// No additional journal entry should be created
	require.Len(t, *journalSlice, 1)
}

func TestJournaledSetTokenBalance(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	tokenId := types.TokenId{0xa, 0xb, 0xc}
	newAmount := types.NewValueFromUint64(500)

	// Get initial token balance (should be nil)
	initialBalance := journaledState.GetTokenBalance(tokenId)

	journaledState.JournaledSetTokenBalance(tokenId, newAmount)

	// Check token balance was updated
	updatedBalance := journaledState.GetTokenBalance(tokenId)
	require.NotNil(t, updatedBalance)
	require.Equal(t, newAmount, *updatedBalance)

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(tokenChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, tokenId, entry.id)
	if initialBalance != nil {
		require.Equal(t, *initialBalance, entry.prev)
	} else {
		require.Equal(t, types.Value{}, entry.prev)
	}
}

func TestJournaledSetSeqno(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialSeqno := journaledState.GetSeqno()
	newSeqno := types.Seqno(15)

	journaledState.JournaledSetSeqno(newSeqno)

	// Check seqno was updated
	require.Equal(t, newSeqno, journaledState.GetSeqno())

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(seqnoChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialSeqno, entry.prev)
}

func TestJournaledSetExtSeqno(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialExtSeqno := journaledState.GetExtSeqno()
	newExtSeqno := types.Seqno(25)

	journaledState.JournaledSetExtSeqno(newExtSeqno)

	// Check ext seqno was updated
	require.Equal(t, newExtSeqno, journaledState.GetExtSeqno())

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(extSeqnoChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialExtSeqno, entry.prev)
}

func TestJournaledSetCode(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialCodeHash := journaledState.GetCodeHash()
	initialCode := journaledState.GetCode()

	newCodeHash := common.HexToHash("0xabcd")
	newCode := types.Code{0x60, 0x60, 0x60, 0x40}

	journaledState.JournaledSetCode(newCodeHash, newCode)

	// Check code was updated
	require.Equal(t, newCodeHash, journaledState.GetCodeHash())
	require.Equal(t, newCode, journaledState.GetCode())

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(codeChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialCodeHash.Bytes(), entry.prevhash)
	require.Equal(t, initialCode, types.Code(entry.prevcode))
}

func TestJournaledSetAsyncContext(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	txIndex := types.TransactionIndex(123)
	asyncCtx := &types.AsyncContext{
		// Add appropriate fields based on AsyncContext definition
	}

	journaledState.JournaledSetAsyncContext(txIndex, asyncCtx)

	// Check async context was set
	retrievedCtx, err := journaledState.GetAsyncContext(txIndex)
	require.NoError(t, err)
	require.Equal(t, asyncCtx, retrievedCtx)

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(asyncContextChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, txIndex, entry.requestId)
}

func TestJournaledSetIsNew(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialIsNew := journaledState.IsNew()
	require.False(t, initialIsNew) // Should be false initially

	journaledState.JournaledSetIsNew(true)

	// Check isNew flag was updated
	require.True(t, journaledState.IsNew())

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(accountBecameContractChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
}

func TestJournaledSetIsSelfDestructed(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	initialIsSelfDestructed := journaledState.IsSelfDestructed()
	initialBalance := journaledState.GetBalance()

	journaledState.JournaledSetIsSelfDestructed(true)

	// Check self-destructed flag was updated
	require.True(t, journaledState.IsSelfDestructed())

	// Check balance was cleared
	require.True(t, journaledState.GetBalance().IsZero())

	// Check journal entry was created
	require.Len(t, *journalSlice, 1)
	entry, ok := (*journalSlice)[0].(selfDestructChange)
	require.True(t, ok)
	require.Equal(t, journaledState.GetAddress(), entry.account)
	require.Equal(t, initialIsSelfDestructed, entry.prev)
	require.Equal(t, initialBalance, entry.prevbalance)
}

func TestMultipleJournaledOperations(t *testing.T) {
	t.Parallel()
	journaledState, journalSlice, database, tx := setupTestAccountState(t)
	defer database.Close()
	defer tx.Rollback()

	// Perform multiple operations
	err := journaledState.JournaledAddBalance(types.NewValueFromUint64(50), tracing.BalanceIncreaseRewardTransactionFee)
	require.NoError(t, err)

	journaledState.JournaledSetSeqno(types.Seqno(20))

	key := common.HexToHash("0x1111")
	value := common.HexToHash("0x2222")
	err = journaledState.JournaledSetState(key, value)
	require.NoError(t, err)

	// Check all journal entries were created
	require.Len(t, *journalSlice, 3)

	// Verify journal entry types
	_, ok1 := (*journalSlice)[0].(balanceChange)
	require.True(t, ok1)

	_, ok2 := (*journalSlice)[1].(seqnoChange)
	require.True(t, ok2)

	_, ok3 := (*journalSlice)[2].(storageChange)
	require.True(t, ok3)
}
