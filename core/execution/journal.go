package execution

import (
	"github.com/NilFoundation/nil/common"
	"github.com/holiman/uint256"
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
		account *common.Address
	}

	// createContractChange represents an account becoming a contract-account.
	// This event happens prior to executing initcode. The journal-event simply
	// manages the created-flag, in order to allow same-tx destruction.
	createContractChange struct {
		account common.Address
	}

	selfDestructChange struct {
		account     *common.Address
		prev        bool // whether account had already self-destructed
		prevbalance *uint256.Int
	}

	// Changes to individual accounts.
	balanceChange struct {
		account *common.Address
		prev    *uint256.Int
	}
	seqnoChange struct {
		account *common.Address
		prev    uint64
	}
	storageChange struct {
		account   *common.Address
		key       common.Hash
		prevvalue common.Hash
	}
	codeChange struct {
		account            *common.Address
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
		account       *common.Address
		key, prevalue common.Hash
	}
)

func (ch createObjectChange) revert(s *ExecutionState) {
	delete(s.Accounts, *ch.account)
}

func (ch createContractChange) revert(s *ExecutionState) {
	s.GetAccount(ch.account).newContract = false
}

func (ch selfDestructChange) revert(s *ExecutionState) {
	obj := s.GetAccount(*ch.account)
	if obj != nil {
		obj.selfDestructed = ch.prev
		obj.setBalance(ch.prevbalance)
	}
}

func (ch balanceChange) revert(s *ExecutionState) {
	s.GetAccount(*ch.account).setBalance(ch.prev)
}

func (ch seqnoChange) revert(s *ExecutionState) {
	s.GetAccount(*ch.account).setSeqno(ch.prev)
}

func (ch codeChange) revert(s *ExecutionState) {
	s.GetAccount(*ch.account).setCode(common.BytesToHash(ch.prevhash), ch.prevcode)
}

func (ch storageChange) revert(s *ExecutionState) {
	s.GetAccount(*ch.account).setState(ch.key, ch.prevvalue)
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
