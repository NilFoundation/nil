package execution

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	doNotRevertLogsFeatureEnabled = false
)

type IRevertableExecutionState interface {
	deleteObject(addr types.Address)
	deleteContractChange(addr types.Address)
	revertSelfDestructChange(addr types.Address, prev bool, prevBalance types.Value)
	revertBalanceChange(addr types.Address, prevBalance types.Value)
	revertCurrencyChange(addr types.Address, currencyId types.CurrencyId, prevValue types.Value)
	revertSeqnoChange(addr types.Address, prevSeqno types.Seqno)
	revertExtSeqnoChange(addr types.Address, prevExtSeqno types.Seqno)
	revertCodeChange(addr types.Address, prevCodeHash common.Hash, prevCode []byte)
	revertStorageChange(addr types.Address, key common.Hash, prevValue common.Hash)
	revertRefundChange(prevRefund uint64)
	deleteLog(txHash common.Hash)
	revertTransientStorageChange(addr types.Address, key common.Hash, prevValue common.Hash)
	revertOutMessagesChange(index int, msgHash common.Hash)
	revertAsyncContextChange(addr types.Address, requestId types.MessageIndex)
}

// JournalEntry is a modification entry in the state change journal that can be
// reverted on demand.
type JournalEntry interface {
	// revert undoes the changes introduced by this journal entry.
	revert(IRevertableExecutionState)
}

// journal contains the list of state modifications applied since the last state
// commit. These are tracked to be able to be reverted in the case of an execution
// exception or request for reversal.
type journal struct {
	entries []JournalEntry // Current changes tracked by the journal
}

// newJournal creates a new initialized journal.
func newJournal() *journal {
	return &journal{}
}

// append inserts a new modification entry to the end of the change journal.
func (j *journal) append(entry JournalEntry) {
	j.entries = append(j.entries, entry)
}

// revert undoes a batch of journalled modifications
func (j *journal) revert(statedb IRevertableExecutionState, snapshot int) {
	for i := len(j.entries) - 1; i >= snapshot; i-- {
		// Undo the changes made by the operation
		j.entries[i].revert(statedb)
	}
	j.entries = j.entries[:snapshot]
}

// length returns the current number of entries in the journal.
func (j *journal) length() int {
	return len(j.entries)
}

type (
	// Changes to the account trie.
	createObjectChange struct {
		account *types.Address
	}

	// createContractChange represents an account becoming a contract-account.
	// This event happens prior to executing initcode. The journal-event simply
	// manages the created-flag, in order to allow same-tx destruction.
	createContractChange struct {
		account types.Address
	}

	selfDestructChange struct {
		account     *types.Address
		prev        bool // whether account had already self-destructed
		prevbalance types.Value
	}

	// Changes to individual accounts.
	balanceChange struct {
		account *types.Address
		prev    types.Value
	}
	currencyChange struct {
		account *types.Address
		id      types.CurrencyId
		prev    types.Value
	}
	seqnoChange struct {
		account *types.Address
		prev    types.Seqno
	}
	extSeqnoChange struct {
		account *types.Address
		prev    types.Seqno
	}
	storageChange struct {
		account   *types.Address
		key       common.Hash
		prevvalue common.Hash
	}
	codeChange struct {
		account            *types.Address
		prevcode, prevhash []byte
	}

	// Changes to other state values.
	refundChange struct {
		prev uint64
	}

	addLogChange struct {
		txhash common.Hash
	}

	// Changes to transient storage
	transientStorageChange struct {
		account       *types.Address
		key, prevalue common.Hash
	}
	outMessagesChange struct {
		msgHash common.Hash
		index   int
	}
	asyncContextChange struct {
		account   *types.Address
		requestId types.MessageIndex
	}
)

func (ch createObjectChange) revert(s IRevertableExecutionState) {
	s.deleteObject(*ch.account)
}

func (ch createContractChange) revert(s IRevertableExecutionState) {
	s.deleteContractChange(ch.account)
}

func (ch selfDestructChange) revert(s IRevertableExecutionState) {
	s.revertSelfDestructChange(*ch.account, ch.prev, ch.prevbalance)
}

func (ch balanceChange) revert(s IRevertableExecutionState) {
	s.revertBalanceChange(*ch.account, ch.prev)
}

func (ch currencyChange) revert(s IRevertableExecutionState) {
	s.revertCurrencyChange(*ch.account, ch.id, ch.prev)
}

func (ch seqnoChange) revert(s IRevertableExecutionState) {
	s.revertSeqnoChange(*ch.account, ch.prev)
}

func (ch extSeqnoChange) revert(s IRevertableExecutionState) {
	s.revertExtSeqnoChange(*ch.account, ch.prev)
}

func (ch codeChange) revert(s IRevertableExecutionState) {
	s.revertCodeChange(*ch.account, common.BytesToHash(ch.prevhash), ch.prevcode)
}

func (ch storageChange) revert(s IRevertableExecutionState) {
	s.revertStorageChange(*ch.account, ch.key, ch.prevvalue)
}

func (ch refundChange) revert(s IRevertableExecutionState) {
	s.revertRefundChange(ch.prev)
}

func (ch addLogChange) revert(s IRevertableExecutionState) {
	s.deleteLog(ch.txhash)
}

func (ch transientStorageChange) revert(s IRevertableExecutionState) {
	s.revertTransientStorageChange(*ch.account, ch.key, ch.prevalue)
}

func (ch outMessagesChange) revert(s IRevertableExecutionState) {
	s.revertOutMessagesChange(ch.index, ch.msgHash)
}

func (ch asyncContextChange) revert(s IRevertableExecutionState) {
	s.revertAsyncContextChange(*ch.account, ch.requestId)
}

type ExecutionStateRevertableWrapper struct {
	es *ExecutionState
}

var _ IRevertableExecutionState = new(ExecutionStateRevertableWrapper)

func (w *ExecutionStateRevertableWrapper) deleteObject(addr types.Address) {
	delete(w.es.Accounts, addr)
}

func (w *ExecutionStateRevertableWrapper) deleteContractChange(addr types.Address) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.NewContract = false
	}
}

func (w *ExecutionStateRevertableWrapper) revertSelfDestructChange(addr types.Address, prev bool, prevBalance types.Value) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.selfDestructed = prev
		account.setBalance(prevBalance)
	}
}

func (w *ExecutionStateRevertableWrapper) revertBalanceChange(addr types.Address, prevBalance types.Value) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setBalance(prevBalance)
	}
}

func (w *ExecutionStateRevertableWrapper) revertCurrencyChange(addr types.Address, currencyId types.CurrencyId, prevValue types.Value) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setCurrencyBalance(currencyId, prevValue)
	}
}

func (w *ExecutionStateRevertableWrapper) revertSeqnoChange(addr types.Address, prevSeqno types.Seqno) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.Seqno = prevSeqno
	}
}

func (w *ExecutionStateRevertableWrapper) revertExtSeqnoChange(addr types.Address, prevExtSeqno types.Seqno) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.ExtSeqno = prevExtSeqno
	}
}

func (w *ExecutionStateRevertableWrapper) revertCodeChange(addr types.Address, prevCodeHash common.Hash, prevCode []byte) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setCode(prevCodeHash, prevCode)
	}
}

func (w *ExecutionStateRevertableWrapper) revertStorageChange(addr types.Address, key common.Hash, prevValue common.Hash) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setState(key, prevValue)
	}
}

func (w *ExecutionStateRevertableWrapper) revertRefundChange(prevRefund uint64) {
	w.es.refund = prevRefund
}

func (w *ExecutionStateRevertableWrapper) deleteLog(txHash common.Hash) {
	if doNotRevertLogsFeatureEnabled {
		return
	}
	logs := w.es.Logs[txHash]
	if len(logs) == 1 {
		delete(w.es.Logs, txHash)
	} else {
		w.es.Logs[txHash] = logs[:len(logs)-1]
	}
}

func (w *ExecutionStateRevertableWrapper) revertTransientStorageChange(addr types.Address, key common.Hash, prevValue common.Hash) {
	w.es.setTransientState(addr, key, prevValue)
}

func (w *ExecutionStateRevertableWrapper) revertOutMessagesChange(index int, msgHash common.Hash) {
	outMessages, ok := w.es.OutMessages[msgHash]
	check.PanicIfNot(ok)

	// Probably it is possible that the message is not the last in the list, but let's assume it is for a now.
	// And catch opposite case with this assert.
	check.PanicIfNot(index == len(outMessages)-1)

	w.es.OutMessages[msgHash] = outMessages[:index]
}

func (w *ExecutionStateRevertableWrapper) revertAsyncContextChange(addr types.Address, requestId types.MessageIndex) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		delete(account.AsyncContext, requestId)
	}
}
