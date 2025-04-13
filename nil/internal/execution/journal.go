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
	DeleteAccount(addr types.Address)
	GetAccount(addr types.Address) (*AccountState, error)
	SetRefund(value uint64)
	DeleteLog(txHash common.Hash)
	SetTransientNoJournal(addr types.Address, key common.Hash, prevValue common.Hash)
	DeleteOutTransaction(index int, txnHash common.Hash)
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
	createAccountChange struct {
		account *types.Address
	}

	// accountBecameContractChange represents an account becoming a contract-account.
	// This event happens prior to executing initcode. The journal-event simply
	// manages the created-flag, in order to allow same-tx destruction.
	accountBecameContractChange struct {
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
	tokenChange struct {
		account *types.Address
		id      types.TokenId
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
	outTransactionsChange struct {
		txnHash common.Hash
		index   int
	}
	asyncContextChange struct {
		account   *types.Address
		requestId types.TransactionIndex
	}
)

func (ch createAccountChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertCreateAccount(*ch.account)
}

func (ch accountBecameContractChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertAccountBecameContractChange(ch.account)
}

func (ch selfDestructChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertSelfDestructChange(*ch.account, ch.prev, ch.prevbalance)
}

func (ch balanceChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertBalanceChange(*ch.account, ch.prev)
}

func (ch tokenChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertTokenChange(*ch.account, ch.id, ch.prev)
}

func (ch seqnoChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertSeqnoChange(*ch.account, ch.prev)
}

func (ch extSeqnoChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertExtSeqnoChange(*ch.account, ch.prev)
}

func (ch codeChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertCodeChange(*ch.account, common.BytesToHash(ch.prevhash), ch.prevcode)
}

func (ch storageChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertStorageChange(*ch.account, ch.key, ch.prevvalue)
}

func (ch refundChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertRefundChange(ch.prev)
}

func (ch addLogChange) revert(s IRevertableExecutionState) {
	reverter{s}.deleteLog(ch.txhash)
}

func (ch transientStorageChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertTransientStorageChange(*ch.account, ch.key, ch.prevalue)
}

func (ch outTransactionsChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertOutTransactionsChange(ch.index, ch.txnHash)
}

func (ch asyncContextChange) revert(s IRevertableExecutionState) {
	reverter{s}.revertAsyncContextChange(*ch.account, ch.requestId)
}

type reverter struct {
	es IRevertableExecutionState
}

func (w reverter) revertCreateAccount(addr types.Address) {
	w.es.DeleteAccount(addr)
}

func (w reverter) revertAccountBecameContractChange(addr types.Address) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.NewContract = false
	}
}

func (w reverter) revertSelfDestructChange(addr types.Address, prev bool, prevBalance types.Value) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.selfDestructed = prev
		account.setBalance(prevBalance)
	}
}

func (w reverter) revertBalanceChange(addr types.Address, prevBalance types.Value) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setBalance(prevBalance)
	}
}

func (w reverter) revertTokenChange(addr types.Address, tokenId types.TokenId, prevValue types.Value) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setTokenBalance(tokenId, prevValue)
	}
}

func (w reverter) revertSeqnoChange(addr types.Address, prevSeqno types.Seqno) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.Seqno = prevSeqno
	}
}

func (w reverter) revertExtSeqnoChange(addr types.Address, prevExtSeqno types.Seqno) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.ExtSeqno = prevExtSeqno
	}
}

func (w reverter) revertCodeChange(addr types.Address, prevCodeHash common.Hash, prevCode []byte) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setCode(prevCodeHash, prevCode)
	}
}

func (w reverter) revertStorageChange(addr types.Address, key common.Hash, prevValue common.Hash) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		account.setState(key, prevValue)
	}
}

func (w reverter) revertRefundChange(prevRefund uint64) {
	w.es.SetRefund(prevRefund)
}

func (w reverter) deleteLog(txHash common.Hash) {
	if doNotRevertLogsFeatureEnabled {
		return
	}
	w.es.DeleteLog(txHash)
}

func (w reverter) revertTransientStorageChange(addr types.Address, key common.Hash, prevValue common.Hash) {
	w.es.SetTransientNoJournal(addr, key, prevValue)
}

func (w reverter) revertOutTransactionsChange(index int, txnHash common.Hash) {
	w.es.DeleteOutTransaction(index, txnHash)
}

func (w reverter) revertAsyncContextChange(addr types.Address, requestId types.TransactionIndex) {
	account, err := w.es.GetAccount(addr)
	check.PanicIfErr(err)
	if account != nil {
		delete(account.AsyncContext, requestId)
	}
}
