package execution

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/NilFoundation/nil/common"
	db "github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	ssz "github.com/ferranbt/fastssz"
	"github.com/holiman/uint256"
)

var logger = common.NewLogger("execution", false /* noColor */)

type Storage map[common.Hash]common.Hash

type AccountState struct {
	db      *ExecutionState
	address common.Address // address of ethereum account

	Tx          db.Tx
	Balance     uint256.Int
	Code        types.Code
	CodeHash    common.Hash
	Seqno       uint64
	StorageRoot *mpt.MerklePatriciaTrie
	ShardId     types.ShardId

	State Storage

	// Flag whether the account was marked as self-destructed. The self-destructed
	// account is still accessible in the scope of same transaction.
	selfDestructed bool

	// This is an EIP-6780 flag indicating whether the object is eligible for
	// self-destruct according to EIP-6780. The flag could be set either when
	// the contract is just created within the current transaction, or when the
	// object was previously existent and is being deployed as a contract within
	// the current transaction.
	newContract bool
}

type ExecutionState struct {
	tx               db.Tx
	ContractRoot     *mpt.MerklePatriciaTrie
	MessageRoot      *mpt.MerklePatriciaTrie
	ReceiptRoot      *mpt.MerklePatriciaTrie
	PrevBlock        common.Hash
	MasterChain      common.Hash
	ShardId          types.ShardId
	ChildChainBlocks map[uint64]common.Hash

	MessageHash common.Hash
	Logs        map[common.Hash][]*types.Log

	Accounts map[common.Address]*AccountState
	Messages []*types.Message
	Receipts []*types.Receipt

	// Transient storage
	transientStorage transientStorage

	// The refund counter, also used by state transitioning.
	refund uint64

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionId int
}

type revision struct {
	id           int
	journalIndex int
}

func (s *AccountState) empty() bool {
	return s.Seqno == 0 && s.Balance.IsZero() && len(s.Code) == 0
}

func NewAccountState(es *ExecutionState, addr common.Address, tx db.Tx, shardId types.ShardId, data []byte) (*AccountState, error) {
	account := new(types.SmartContract)

	if err := account.UnmarshalSSZ(data); err != nil {
		logger.Fatal().Msg("Invalid SSZ while decoding account")
	}

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.StorageTrieTable, account.StorageRoot)

	code, err := db.ReadCode(tx, shardId, account.CodeHash)
	if err != nil {
		return nil, err
	}

	return &AccountState{
		db:      es,
		address: addr,

		Tx:          tx,
		StorageRoot: root,
		CodeHash:    account.CodeHash,
		Code:        code,
		ShardId:     shardId,
		Seqno:       account.Seqno,
		State:       make(Storage),
	}, nil
}

func NewExecutionState(tx db.Tx, shardId types.ShardId, blockHash common.Hash) (*ExecutionState, error) {
	block := db.ReadBlock(tx, shardId, blockHash)

	var contractRoot, messageRoot, receiptRoot *mpt.MerklePatriciaTrie
	contractTrieTable := db.ContractTrieTable
	messageTrieTable := db.MessageTrieTable
	receiptTrieTable := db.ReceiptTrieTable
	if block != nil {
		contractRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, contractTrieTable, block.SmartContractsRoot)
		messageRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, messageTrieTable, block.MessagesRoot)
		receiptRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, receiptTrieTable, block.ReceiptsRoot)
	} else {
		contractRoot = mpt.NewMerklePatriciaTrie(tx, shardId, contractTrieTable)
		messageRoot = mpt.NewMerklePatriciaTrie(tx, shardId, messageTrieTable)
		receiptRoot = mpt.NewMerklePatriciaTrie(tx, shardId, receiptTrieTable)
	}

	return &ExecutionState{
		tx:               tx,
		ContractRoot:     contractRoot,
		MessageRoot:      messageRoot,
		ReceiptRoot:      receiptRoot,
		PrevBlock:        blockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[uint64]common.Hash{},
		Accounts:         map[common.Address]*AccountState{},
		Messages:         []*types.Message{},
		Receipts:         []*types.Receipt{},
		Logs:             map[common.Hash][]*types.Log{},

		journal:          newJournal(),
		transientStorage: newTransientStorage(),
	}, nil
}

func (es *ExecutionState) GetAccount(addr common.Address) *AccountState {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc
	}

	addrHash := addr.Hash()

	data, err := es.ContractRoot.Get(addrHash[:])
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		panic(fmt.Sprintf("failed to fetch account %v: %v", addrHash, err))
	}

	if data == nil {
		return nil
	}

	acc, err = NewAccountState(es, addr, es.tx, es.ShardId, data)
	if err != nil {
		panic(fmt.Sprintf("failed to create account on shard %v: %v", es.ShardId, err))
	}
	es.Accounts[addr] = acc
	return acc
}

func (es *ExecutionState) setAccountObject(acc *AccountState) {
	es.Accounts[acc.address] = acc
}

func (es *ExecutionState) AddAddressToAccessList(addr common.Address) {
}

// AddBalance adds amount to the account associated with addr.
func (s *ExecutionState) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
	stateObject := s.getOrNewAccount(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount, reason)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (s *ExecutionState) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
	stateObject := s.getOrNewAccount(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount, reason)
	}
}

// AddBalance adds amount to s's balance.
// It is used to add funds to the destination account of a transfer.
func (s *AccountState) AddBalance(amount *uint256.Int, reason tracing.BalanceChangeReason) {
	if amount.IsZero() {
		return
	}
	s.SetBalance(*new(uint256.Int).Add(&s.Balance, amount))
}

// SubBalance removes amount from s's balance.
// It is used to remove funds from the origin account of a transfer.
func (s *AccountState) SubBalance(amount *uint256.Int, reason tracing.BalanceChangeReason) {
	if amount.IsZero() {
		return
	}
	s.SetBalance(*new(uint256.Int).Sub(&s.Balance, amount))
}

func (es *ExecutionState) AddLog(log *types.Log) {
	es.journal.append(addLogChange{txhash: es.MessageHash})
	es.Logs[es.MessageHash] = append(es.Logs[es.MessageHash], log)
}

// AddRefund adds gas to the refund counter
func (s *ExecutionState) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

// GetRefund returns the current value of the refund counter.
func (s *ExecutionState) GetRefund() uint64 {
	return s.refund
}

func (es *ExecutionState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
}

func (es *ExecutionState) AddressInAccessList(addr common.Address) bool {
	return true // FIXME
}

func (es *ExecutionState) Empty(addr common.Address) bool {
	acc := es.GetAccount(addr)
	return acc == nil || acc.empty()
}

func (es *ExecutionState) Exist(addr common.Address) bool {
	acc := es.GetAccount(addr)
	return acc != nil
}

func (es *ExecutionState) GetCode(addr common.Address) []byte {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.Code
	}
	return nil
}

func (es *ExecutionState) GetCodeHash(addr common.Address) common.Hash {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.CodeHash
	}
	return common.EmptyHash
}

func (es *ExecutionState) GetCodeSize(addr common.Address) int {
	acc := es.GetAccount(addr)
	if acc != nil {
		return len(acc.Code)
	}
	return 0
}

func (es *ExecutionState) GetCommittedState(common.Address, common.Hash) common.Hash {
	return common.EmptyHash
}

// Snapshot returns an identifier for the current revision of the state.
func (s *ExecutionState) Snapshot() int {
	id := s.nextRevisionId
	s.nextRevisionId++
	s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (s *ExecutionState) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(s.validRevisions), func(i int) bool {
		return s.validRevisions[i].id >= revid
	})
	if idx == len(s.validRevisions) || s.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	s.journal.revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

func (es *ExecutionState) GetStorageRoot(addr common.Address) common.Hash {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.StorageRoot.RootHash()
	}
	return common.EmptyHash
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (s *ExecutionState) SetTransientState(addr common.Address, key, value common.Hash) {
	prev := s.GetTransientState(addr, key)
	if prev == value {
		return
	}
	s.journal.append(transientStorageChange{
		account:  &addr,
		key:      key,
		prevalue: prev,
	})
	s.setTransientState(addr, key, value)
}

// setTransientState is a lower level setter for transient storage. It
// is called during a revert to prevent modifications to the journal.
func (s *ExecutionState) setTransientState(addr common.Address, key, value common.Hash) {
	s.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (s *ExecutionState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.transientStorage.Get(addr, key)
}

// SelfDestruct marks the given account as selfdestructed.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// GetAccount will return a non-nil account after SelfDestruct.
func (s *ExecutionState) selfDestruct(stateObject *AccountState) {
	var (
		prev = new(uint256.Int).Set(&stateObject.Balance)
		n    = new(uint256.Int)
	)
	s.journal.append(selfDestructChange{
		account:     &stateObject.address,
		prev:        stateObject.selfDestructed,
		prevbalance: prev,
	})
	stateObject.selfDestructed = true
	stateObject.Balance = *n
}

func (s *ExecutionState) Selfdestruct6780(addr common.Address) {
	stateObject := s.GetAccount(addr)
	if stateObject == nil {
		return
	}
	if stateObject.newContract {
		s.selfDestruct(stateObject)
	}
}

func (s *ExecutionState) HasSelfDestructed(addr common.Address) bool {
	stateObject := s.GetAccount(addr)
	if stateObject != nil {
		return stateObject.selfDestructed
	}
	return false
}

func (es *ExecutionState) SetCode(addr common.Address, code []byte) {
	acc := es.GetAccount(addr)
	acc.SetCode(types.Code(code).Hash(), code)
}

func (es *ExecutionState) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return true, true // FIXME
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (s *ExecutionState) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, s.refund))
	}
	s.refund -= gas
}

func (as *AccountState) GetState(key common.Hash) common.Hash {
	val, ok := as.State[key]
	if ok {
		return val
	}

	newVal := as.GetCommittedState(key)
	as.State[key] = newVal

	return newVal
}

func (s *AccountState) SetBalance(amount uint256.Int) {
	s.db.journal.append(balanceChange{
		account: &s.address,
		prev:    new(uint256.Int).Set(&s.Balance),
	})
	s.setBalance(&amount)
}

func (s *AccountState) setBalance(amount *uint256.Int) {
	s.Balance = *amount
}

func (as *AccountState) SetSeqno(seqno uint64) {
	as.db.journal.append(seqnoChange{
		account: &as.address,
		prev:    as.Seqno,
	})
	as.setSeqno(seqno)
}

func (as *AccountState) setSeqno(seqno uint64) {
	as.Seqno = seqno
}

func (s *AccountState) SetCode(codeHash common.Hash, code []byte) {
	prevcode := s.Code
	s.db.journal.append(codeChange{
		account:  &s.address,
		prevhash: s.CodeHash[:],
		prevcode: prevcode,
	})
	s.setCode(codeHash, code)
}

func (s *AccountState) setCode(codeHash common.Hash, code []byte) {
	s.Code = code
	s.CodeHash = common.Hash(codeHash[:])
}

func (s *AccountState) SetState(key common.Hash, value common.Hash) {
	// If the new value is the same as old, don't set. Otherwise, track only the
	// dirty changes, supporting reverting all of it back to no change.
	prev := s.GetState(key)
	if prev == value {
		return
	}
	// New value is different, update and journal the change
	s.db.journal.append(storageChange{
		account:   &s.address,
		key:       key,
		prevvalue: prev,
	})
	s.setState(key, value)
}

func (s *AccountState) setState(key common.Hash, value common.Hash) {
	s.State[key] = value
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (s *AccountState) GetCommittedState(key common.Hash) common.Hash {
	rawVal, err := s.StorageRoot.Get(key[:])
	if errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash
	}
	if err != nil {
		panic(fmt.Sprintf("unexpected error fetching storage: %v", err))
	}

	var newVal uint256.Int
	if err := newVal.UnmarshalSSZ(rawVal); err != nil {
		panic("failed to unmarshal storage cell")
	}
	return newVal.Bytes32()
}

func (as *AccountState) Commit() ([]byte, error) {
	for k, v := range as.State {
		vv, err := v.Uint256().MarshalSSZ()
		if err != nil {
			return nil, err
		}
		if err := as.StorageRoot.Set(k[:], vv); err != nil {
			return nil, err
		}
	}

	acc := types.SmartContract{
		Balance:     types.Uint256{Int: as.Balance},
		StorageRoot: as.StorageRoot.RootHash(),
		CodeHash:    as.CodeHash,
		Seqno:       as.Seqno,
	}

	data, err := acc.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	if err := db.WriteCode(as.Tx, as.ShardId, as.Code); err != nil {
		return nil, err
	}

	return data, nil
}

func (es *ExecutionState) GetState(addr common.Address, key common.Hash) common.Hash {
	acc := es.GetAccount(addr)
	if acc == nil {
		return common.EmptyHash
	}

	return acc.GetState(key)
}

func (es *ExecutionState) SetState(addr common.Address, key common.Hash, val common.Hash) {
	acc := es.getOrNewAccount(addr)
	acc.SetState(key, val)
}

func (es *ExecutionState) GetBalance(addr common.Address) *uint256.Int {
	acc := es.GetAccount(addr)
	if acc == nil {
		return uint256.NewInt(0)
	}
	return &acc.Balance
}

func (es *ExecutionState) GetSeqno(addr common.Address) uint64 {
	acc := es.GetAccount(addr)
	if acc == nil {
		return 0
	}
	return acc.Seqno
}

func (es *ExecutionState) getOrNewAccount(addr common.Address) *AccountState {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc
	}
	err := es.CreateAccount(addr)
	if err != nil {
		panic(err)
	}
	return es.GetAccount(addr)
}

func (es *ExecutionState) SetBalance(addr common.Address, balance uint256.Int) {
	acc := es.getOrNewAccount(addr)
	acc.SetBalance(balance)
}

func (es *ExecutionState) SetSeqno(addr common.Address, seqno uint64) {
	acc := es.getOrNewAccount(addr)
	acc.SetSeqno(seqno)
}

func (es *ExecutionState) SetMasterchainHash(masterChainHash common.Hash) {
	es.MasterChain = masterChainHash
}

func (es *ExecutionState) SetShardHash(shardId uint64, hash common.Hash) {
	es.ChildChainBlocks[shardId] = hash
}

func (es *ExecutionState) CreateAccount(addr common.Address) error {
	acc := es.GetAccount(addr)

	if acc != nil {
		return errors.New("account already exists")
	}

	es.journal.append(createObjectChange{account: &addr})

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.StorageTrieTable)

	es.Accounts[addr] = &AccountState{
		db:      es,
		address: addr,

		Tx:          es.tx,
		StorageRoot: root,
		CodeHash:    common.EmptyHash,
		Code:        nil,
		ShardId:     es.ShardId,
		State:       map[common.Hash]common.Hash{},
	}

	return nil
}

// CreateContract is used whenever a contract is created. This may be preceded
// by CreateAccount, but that is not required if it already existed in the
// state due to funds sent beforehand.
// This operation sets the 'newContract'-flag, which is required in order to
// correctly handle EIP-6780 'delete-in-same-transaction' logic.
func (s *ExecutionState) CreateContract(addr common.Address) {
	obj := s.GetAccount(addr)
	if !obj.newContract {
		obj.newContract = true
		s.journal.append(createContractChange{account: addr})
	}
}

func (es *ExecutionState) ContractExists(addr common.Address) bool {
	acc := es.GetAccount(addr)
	return acc != nil
}

// CreateAddress creates an ethereum address given the bytes and the nonce.
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data := []byte{}
	copy(data, b.Bytes())
	data = ssz.MarshalUint64(data, nonce)
	return common.BytesToAddress(data)
}

func (es *ExecutionState) AddMessage(message *types.Message) {
	message.Index = uint64(len(es.Messages))
	es.Messages = append(es.Messages, message)

	// Deploy message
	if bytes.Equal(message.To[:], common.EmptyAddress[:]) {
		addr := CreateAddress(message.From, message.Seqno)

		var r types.Receipt
		r.Success = true
		r.ContractAddress = addr
		r.MsgHash = message.Hash()
		r.MsgIndex = message.Index

		// TODO: gasUsed
		if err := es.CreateAccount(addr); err != nil {
			logger.Fatal().Err(err).Msgf("Failed to create account")
		}
		es.SetCode(addr, message.Data)

		es.Receipts = append(es.Receipts, &r)
	}
}

func (es *ExecutionState) Commit(blockId uint64) (common.Hash, error) {
	for k, acc := range es.Accounts {
		v, err := acc.Commit()
		if err != nil {
			return common.EmptyHash, err
		}

		kHash := k.Hash()
		if err = es.ContractRoot.Set(kHash[:], v); err != nil {
			return common.EmptyHash, err
		}
	}

	treeShardsRootHash := common.EmptyHash
	if len(es.ChildChainBlocks) > 0 {
		treeShards := mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.ShardBlocksTrieTableName(blockId))
		for k, hash := range es.ChildChainBlocks {
			key := strconv.AppendUint(nil, k, 10)
			if err := treeShards.Set(key, hash.Bytes()); err != nil {
				return common.EmptyHash, err
			}
		}
		treeShardsRootHash = treeShards.RootHash()
	}

	for _, m := range es.Messages {
		v, err := m.MarshalSSZ()
		if err != nil {
			return common.EmptyHash, err
		}
		k := ssz.MarshalUint64(nil, m.Index)
		if err := es.MessageRoot.Set(k, v); err != nil {
			return common.EmptyHash, err
		}
		if err := db.WriteMessage(es.tx, es.ShardId, m); err != nil {
			return common.EmptyHash, err
		}
	}

	for _, r := range es.Receipts {
		r.BlockNumber = blockId
		v, err := r.MarshalSSZ()
		if err != nil {
			return common.EmptyHash, err
		}
		k := ssz.MarshalUint64(nil, r.MsgIndex)
		if err := es.ReceiptRoot.Set(k, v); err != nil {
			return common.EmptyHash, err
		}
	}

	block := types.Block{
		Id:                  blockId,
		PrevBlock:           es.PrevBlock,
		SmartContractsRoot:  es.ContractRoot.RootHash(),
		MessagesRoot:        es.MessageRoot.RootHash(),
		ReceiptsRoot:        es.ReceiptRoot.RootHash(),
		ChildBlocksRootHash: treeShardsRootHash,
		MasterChainHash:     es.MasterChain,
	}

	blockHash := block.Hash()

	err := db.WriteBlock(es.tx, es.ShardId, &block)
	if err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}
