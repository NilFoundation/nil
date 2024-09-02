package execution

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// journalEntry is a modification entry in the state change journal that can be
// reverted on demand.
type journalEntry interface {
	// revert undoes the changes introduced by this journal entry.
	revert(*ExecutionState)
}

// journal contains the list of state modifications applied since the last state
// commit. These are tracked to be able to be reverted in the case of an execution
// exception or request for reversal.
type journal struct {
	entries []journalEntry // Current changes tracked by the journal
}

// newJournal creates a new initialized journal.
func newJournal() *journal {
	return &journal{}
}

// append inserts a new modification entry to the end of the change journal.
func (j *journal) append(entry journalEntry) {
	j.entries = append(j.entries, entry)
}

// revert undoes a batch of journalled modifications
func (j *journal) revert(statedb *ExecutionState, snapshot int) {
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

func (ch createObjectChange) revert(s *ExecutionState) {
	delete(s.Accounts, *ch.account)
}

func (ch createContractChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.newContract = false
	}
}

func (ch selfDestructChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.selfDestructed = ch.prev
		account.setBalance(ch.prevbalance)
	}
}

func (ch balanceChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.setBalance(ch.prev)
	}
}

func (ch currencyChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.setCurrencyBalance(ch.id, ch.prev)
	}
}

func (ch seqnoChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.Seqno = ch.prev
	}
}

func (ch extSeqnoChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.ExtSeqno = ch.prev
	}
}

func (ch codeChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.setCode(common.BytesToHash(ch.prevhash), ch.prevcode)
	}
}

func (ch storageChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		account.setState(ch.key, ch.prevvalue)
	}
}

func (ch refundChange) revert(s *ExecutionState) {
	s.refund = ch.prev
}

func (ch addLogChange) revert(s *ExecutionState) {
	logs := s.Logs[ch.txhash]
	if len(logs) == 1 {
		delete(s.Logs, ch.txhash)
	} else {
		s.Logs[ch.txhash] = logs[:len(logs)-1]
	}
}

func (ch transientStorageChange) revert(s *ExecutionState) {
	s.setTransientState(*ch.account, ch.key, ch.prevalue)
}

func (ch outMessagesChange) revert(s *ExecutionState) {
	outMessages, ok := s.OutMessages[ch.msgHash]
	check.PanicIfNot(ok)

	// Probably it is possible that the message is not the last in the list, but let's assume it is for a now.
	// And catch opposite case with this assert.
	check.PanicIfNot(ch.index == len(outMessages)-1)

	s.OutMessages[ch.msgHash] = outMessages[:ch.index]
}

func (ch asyncContextChange) revert(s *ExecutionState) {
	account, err := s.GetAccount(*ch.account)
	check.PanicIfErr(err)
	if account != nil {
		delete(account.AsyncContext, ch.requestId)
	}
}
