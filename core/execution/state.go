package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

var logger = logging.NewLogger("execution")

const TraceBlocksEnabled = false

const ExternalMessageVerificationMaxGas = 100000

var blocksTracer *BlocksTracer

type Storage map[common.Hash]common.Hash

func init() {
	if TraceBlocksEnabled {
		var err error
		blocksTracer, err = NewBlocksTracer()
		if err != nil || blocksTracer == nil {
			panic("Can not create Blocks tracer")
		}
	}
}

type AccountState struct {
	db      *ExecutionState
	address types.Address // address of the ethereum account

	Tx          db.RwTx
	Balance     uint256.Int
	Code        types.Code
	CodeHash    common.Hash
	Seqno       types.Seqno
	StorageTree *StorageTrie

	State Storage

	// Flag whether the account was marked as self-destructed. The self-destructed
	// account is still accessible in the scope of same transaction.
	selfDestructed bool

	// This is an EIP-6780 flag indicating whether the object is eligible for
	// self-destruct, according to EIP-6780. The flag could be set either when
	// the contract is just created within the current transaction, or when the
	// object was previously existent and is being deployed as a contract within
	// the current transaction.
	newContract bool
}

type ExecutionState struct {
	tx               db.RwTx
	Timer            common.Timer
	ContractTree     *ContractTrie
	InMessageTree    *MessageTrie
	OutMessageTree   *MessageTrie
	ReceiptTree      *ReceiptTrie
	PrevBlock        common.Hash
	MasterChain      common.Hash
	ShardId          types.ShardId
	ChildChainBlocks map[types.ShardId]common.Hash

	InMessageHash common.Hash
	Logs          map[common.Hash][]*types.Log

	Accounts   map[types.Address]*AccountState
	InMessages []*types.Message

	// OutMessages holds outbound messages for every transaction in the executed block, where key is hash of Message that sends the message
	OutMessages map[common.Hash][]*types.Message

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

	// If true, log every instruction execution.
	TraceVm bool

	Accessor *StateAccessor
}

type revision struct {
	id           int
	journalIndex int
}

var _ vm.StateDB = new(ExecutionState)

func (s *AccountState) empty() bool {
	return s.Seqno == 0 && s.Balance.IsZero() && len(s.Code) == 0
}

func NewAccountState(es *ExecutionState, addr types.Address, tx db.RwTx, account *types.SmartContract) (*AccountState, error) {
	shardId := addr.ShardId()

	// TODO: store storage of each contract in separate table
	root := NewStorageTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.StorageTrieTable, account.StorageRoot))

	code, err := db.ReadCode(tx, shardId, account.CodeHash)
	if err != nil {
		return nil, err
	}

	return &AccountState{
		db:      es,
		address: addr,

		Tx:          tx,
		Balance:     account.Balance.Int,
		StorageTree: root,
		CodeHash:    account.CodeHash,
		Code:        code,
		Seqno:       account.Seqno,
		State:       make(Storage),
	}, nil
}

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(es *ExecutionState) vm.BlockContext {
	header := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
	lastBlockId := uint64(0)
	if header != nil {
		lastBlockId = header.Id.Uint64()
	}
	return vm.BlockContext{
		GetHash:     getHashFn(es, header),
		BlockNumber: lastBlockId,
	}
}

func NewROExecutionState(tx db.RoTx, shardId types.ShardId, blockHash common.Hash, timer common.Timer) (*ExecutionState, error) {
	return NewExecutionState(&db.RwWrapper{RoTx: tx}, shardId, blockHash, timer)
}

func NewROExecutionStateForShard(tx db.RoTx, shardId types.ShardId, timer common.Timer) (*ExecutionState, error) {
	return NewExecutionStateForShard(&db.RwWrapper{RoTx: tx}, shardId, timer)
}

func NewExecutionState(tx db.RwTx, shardId types.ShardId, blockHash common.Hash, timer common.Timer) (*ExecutionState, error) {
	block := db.ReadBlock(tx, shardId, blockHash)

	var contractRoot, messageRoot, outMessagesTrie, receiptRoot *mpt.MerklePatriciaTrie
	contractTrieTable := db.ContractTrieTable
	messageTrieTable := db.MessageTrieTable
	receiptTrieTable := db.ReceiptTrieTable
	if block != nil {
		contractRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, contractTrieTable, block.SmartContractsRoot)
	} else {
		contractRoot = mpt.NewMerklePatriciaTrie(tx, shardId, contractTrieTable)
	}
	messageRoot = mpt.NewMerklePatriciaTrie(tx, shardId, messageTrieTable)
	outMessagesTrie = mpt.NewMerklePatriciaTrie(tx, shardId, messageTrieTable)
	receiptRoot = mpt.NewMerklePatriciaTrie(tx, shardId, receiptTrieTable)

	accessor, err := NewStateAccessor()
	if err != nil {
		return nil, err
	}

	return &ExecutionState{
		tx:               tx,
		Timer:            timer,
		ContractTree:     NewContractTrie(contractRoot),
		InMessageTree:    NewMessageTrie(messageRoot),
		OutMessageTree:   NewMessageTrie(outMessagesTrie),
		ReceiptTree:      NewReceiptTrie(receiptRoot),
		PrevBlock:        blockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[types.ShardId]common.Hash{},
		Accounts:         map[types.Address]*AccountState{},
		OutMessages:      map[common.Hash][]*types.Message{},
		Logs:             map[common.Hash][]*types.Log{},

		journal:          newJournal(),
		transientStorage: newTransientStorage(),

		Accessor: accessor,
	}, nil
}

func NewExecutionStateForShard(tx db.RwTx, shardId types.ShardId, timer common.Timer) (*ExecutionState, error) {
	lastBlockHashBytes, err := tx.Get(db.LastBlockTable, shardId.Bytes())
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed getting last block: %w", err)
	}

	lastBlockHash := common.EmptyHash
	// No previous blocks yet
	if lastBlockHashBytes != nil {
		lastBlockHash = common.Hash(lastBlockHashBytes)
	}

	return NewExecutionState(tx, shardId, lastBlockHash, timer)
}

func (es *ExecutionState) GetReceipt(msgIndex types.MessageIndex) (*types.Receipt, error) {
	return es.ReceiptTree.Fetch(msgIndex)
}

func (es *ExecutionState) GetAccount(addr types.Address) *AccountState {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc
	}

	addrHash := addr.Hash()

	data, err := es.ContractTree.Fetch(addrHash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		panic(fmt.Sprintf("failed to fetch account %v: %v", addrHash, err))
	}

	acc, err = NewAccountState(es, addr, es.tx, data)
	if err != nil {
		panic(fmt.Sprintf("failed to create account on shard %v: %v", es.ShardId, err))
	}
	es.Accounts[addr] = acc
	return acc
}

func (es *ExecutionState) setAccountObject(acc *AccountState) {
	es.Accounts[acc.address] = acc
}

func (es *ExecutionState) AddAddressToAccessList(addr types.Address) {
}

// AddBalance adds amount to the account associated with addr.
func (s *ExecutionState) AddBalance(addr types.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
	stateObject := s.getOrNewAccount(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount, reason)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (s *ExecutionState) SubBalance(addr types.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
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
	es.journal.append(addLogChange{txhash: es.InMessageHash})
	es.Logs[es.InMessageHash] = append(es.Logs[es.InMessageHash], log)
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

func (es *ExecutionState) AddSlotToAccessList(addr types.Address, slot common.Hash) {
}

func (es *ExecutionState) AddressInAccessList(addr types.Address) bool {
	return true // FIXME
}

func (es *ExecutionState) Empty(addr types.Address) bool {
	acc := es.GetAccount(addr)
	return acc == nil || acc.empty()
}

func (es *ExecutionState) Exist(addr types.Address) bool {
	acc := es.GetAccount(addr)
	return acc != nil
}

func (es *ExecutionState) GetCode(addr types.Address) []byte {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.Code
	}
	return nil
}

func (es *ExecutionState) GetCodeHash(addr types.Address) common.Hash {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.CodeHash
	}
	return common.EmptyHash
}

func (es *ExecutionState) GetCodeSize(addr types.Address) int {
	acc := es.GetAccount(addr)
	if acc != nil {
		return len(acc.Code)
	}
	return 0
}

func (es *ExecutionState) GetCommittedState(types.Address, common.Hash) common.Hash {
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

func (es *ExecutionState) GetStorageRoot(addr types.Address) common.Hash {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.StorageTree.RootHash()
	}
	return common.EmptyHash
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (s *ExecutionState) SetTransientState(addr types.Address, key, value common.Hash) {
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
func (s *ExecutionState) setTransientState(addr types.Address, key, value common.Hash) {
	s.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (s *ExecutionState) GetTransientState(addr types.Address, key common.Hash) common.Hash {
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

func (s *ExecutionState) Selfdestruct6780(addr types.Address) {
	stateObject := s.GetAccount(addr)
	if stateObject == nil {
		return
	}
	if stateObject.newContract {
		s.selfDestruct(stateObject)
	}
}

func (s *ExecutionState) HasSelfDestructed(addr types.Address) bool {
	stateObject := s.GetAccount(addr)
	if stateObject != nil {
		return stateObject.selfDestructed
	}
	return false
}

func (es *ExecutionState) SetCode(addr types.Address, code []byte) {
	acc := es.GetAccount(addr)
	acc.SetCode(types.Code(code).Hash(), code)
}

func (es *ExecutionState) EnableVmTracing(evm *vm.EVM) {
	evm.Config.Tracer = &tracing.Hooks{
		OnOpcode: func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
			for i, item := range scope.StackData() {
				logger.Debug().Msgf("     %d: %s", i, item.String())
			}
			logger.Debug().Msgf("%04x: %s", pc, vm.OpCode(op).String())
		},
	}
}

func (es *ExecutionState) SetInitState(addr types.Address, message *types.Message) error {
	acc := es.GetAccount(addr)
	acc.setSeqno(message.Seqno)

	evm := vm.NewEVM(NewEVMBlockContext(es), es)
	var from types.Address
	var value uint256.Int
	var err error
	_, deployAddr, _, err := evm.Deploy(addr, (vm.AccountRef)(from), message.Data, uint64(100000) /* gas */, &value)
	if err != nil {
		return err
	}
	if addr != deployAddr {
		return errors.New("deploy address is not correct")
	}
	return nil
}

func (es *ExecutionState) SlotInAccessList(addr types.Address, slot common.Hash) (addressOk bool, slotOk bool) {
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

func (as *AccountState) SetSeqno(seqno types.Seqno) {
	as.db.journal.append(seqnoChange{
		account: &as.address,
		prev:    as.Seqno,
	})
	as.setSeqno(seqno)
}

func (as *AccountState) setSeqno(seqno types.Seqno) {
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
	res, err := s.StorageTree.Fetch(key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash
	}
	if err != nil {
		panic(fmt.Sprintf("unexpected error fetching storage: %v", err))
	}

	return res.Bytes32()
}

func (as *AccountState) Commit() (*types.SmartContract, error) {
	for k, v := range as.State {
		if err := as.StorageTree.Update(k, v.Uint256()); err != nil {
			return nil, err
		}
	}

	acc := &types.SmartContract{
		Address:     as.address,
		Balance:     types.Uint256{Int: as.Balance},
		StorageRoot: as.StorageTree.RootHash(),
		CodeHash:    as.CodeHash,
		Seqno:       as.Seqno,
	}

	if err := db.WriteCode(as.Tx, as.address.ShardId(), as.Code); err != nil {
		return nil, err
	}

	return acc, nil
}

func (es *ExecutionState) GetState(addr types.Address, key common.Hash) common.Hash {
	acc := es.GetAccount(addr)
	if acc == nil {
		return common.EmptyHash
	}

	return acc.GetState(key)
}

func (es *ExecutionState) SetState(addr types.Address, key common.Hash, val common.Hash) {
	acc := es.getOrNewAccount(addr)
	acc.SetState(key, val)
}

func (es *ExecutionState) GetBalance(addr types.Address) *uint256.Int {
	acc := es.GetAccount(addr)
	if acc == nil {
		return uint256.NewInt(0)
	}
	return &acc.Balance
}

func (es *ExecutionState) GetSeqno(addr types.Address) types.Seqno {
	acc := es.GetAccount(addr)
	if acc == nil {
		return 0
	}
	return acc.Seqno
}

func (es *ExecutionState) getOrNewAccount(addr types.Address) *AccountState {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc
	}
	es.CreateAccount(addr)
	return es.GetAccount(addr)
}

func (es *ExecutionState) SetBalance(addr types.Address, balance uint256.Int) {
	acc := es.getOrNewAccount(addr)
	acc.SetBalance(balance)
}

func (es *ExecutionState) SetSeqno(addr types.Address, seqno types.Seqno) {
	acc := es.getOrNewAccount(addr)
	acc.SetSeqno(seqno)
}

func (es *ExecutionState) SetMasterchainHash(masterChainHash common.Hash) {
	es.MasterChain = masterChainHash
}

func (es *ExecutionState) SetShardHash(shardId types.ShardId, hash common.Hash) {
	es.ChildChainBlocks[shardId] = hash
}

func (es *ExecutionState) CreateAccount(addr types.Address) {
	acc := es.GetAccount(addr)

	if acc != nil {
		panic("account already exists")
	}

	es.journal.append(createObjectChange{account: &addr})

	// TODO: store storage of each contract in separate table
	root := NewStorageTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.StorageTrieTable))

	es.Accounts[addr] = &AccountState{
		db:      es,
		address: addr,

		Tx:          es.tx,
		StorageTree: root,
		CodeHash:    common.EmptyHash,
		Code:        nil,
		State:       map[common.Hash]common.Hash{},
	}
}

// CreateContract is used whenever a contract is created. This may be preceded
// by CreateAccount, but that is not required if it already existed in the
// state due to funds sent beforehand.
// This operation sets the 'newContract'-flag, which is required in order to
// correctly handle EIP-6780 'delete-in-same-transaction' logic.
func (s *ExecutionState) CreateContract(addr types.Address) {
	obj := s.GetAccount(addr)
	if !obj.newContract {
		obj.newContract = true
		s.journal.append(createContractChange{account: addr})
	}
}

func (es *ExecutionState) accountExists(addr types.Address) bool {
	acc := es.GetAccount(addr)
	return acc != nil
}

// Contract is regarded as existent if any of these three conditions is met:
// - the nonce is non-zero
// - the code is non-empty
// - the storage is non-empty
func (es *ExecutionState) ContractExists(address types.Address) bool {
	contractHash := es.GetCodeHash(address)
	storageRoot := es.GetStorageRoot(address)
	return es.GetSeqno(address) != 0 ||
		(contractHash != common.EmptyHash) || // non-empty code
		(storageRoot != common.EmptyHash) // non-empty storage
}

func (es *ExecutionState) AddInMessage(message *types.Message) types.MessageIndex {
	index := len(es.InMessages)
	es.InMessages = append(es.InMessages, message)
	return types.MessageIndex(index)
}

func (es *ExecutionState) AddOutMessage(txId common.Hash, msg *types.Message) {
	es.OutMessages[txId] = append(es.OutMessages[txId], msg)
}

func (es *ExecutionState) HandleDeployMessage(
	_ context.Context, message *types.Message, deployMsg *types.DeployPayload, blockContext *vm.BlockContext,
) (uint64, error) {
	addr := message.To

	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Msg("Handling deploy message...")

	gas := message.GasLimit.Uint64()

	evm := vm.NewEVM(*blockContext, es)
	_, addr, leftOverGas, err := evm.Deploy(addr, (vm.AccountRef)(message.From), deployMsg.Code(), gas, &message.Value.Int)

	r := &types.Receipt{
		Success:         err == nil,
		ContractAddress: addr,
		MsgHash:         es.InMessageHash,
		GasUsed:         uint32(gas - leftOverGas),
	}

	es.AddReceipt(r)

	event := logger.Debug().Stringer(logging.FieldMessageTo, addr)
	if err != nil {
		event.Err(err).Msg("Contract deployment failed.")
	} else {
		event.Msg("Created new contract.")
	}

	return leftOverGas, err
}

func (es *ExecutionState) HandleExecutionMessage(_ context.Context, message *types.Message, blockContext *vm.BlockContext) (uint64, []byte, error) {
	addr := message.To
	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Msg("Handling execution message...")

	gas := message.GasLimit.Uint64()

	evm := vm.NewEVM(*blockContext, es)

	if es.TraceVm {
		es.EnableVmTracing(evm)
	}

	es.SetSeqno(addr, es.GetSeqno(addr)+1)
	ret, leftOverGas, err := evm.Call((vm.AccountRef)(message.From), addr, message.Data, gas, &message.Value.Int)
	if err != nil {
		logger.Error().Err(err).Msg("execution message failed")
	}
	r := types.Receipt{
		Success:         err == nil,
		GasUsed:         uint32(gas - leftOverGas),
		Logs:            es.Logs[es.InMessageHash],
		MsgHash:         es.InMessageHash,
		ContractAddress: addr,
	}
	es.AddReceipt(&r)
	return leftOverGas, ret, err
}

func (es *ExecutionState) AddReceipt(receipt *types.Receipt) {
	es.Receipts = append(es.Receipts, receipt)
}

func (es *ExecutionState) Commit(blockId types.BlockNumber) (common.Hash, error) {
	for k, acc := range es.Accounts {
		v, err := acc.Commit()
		if err != nil {
			return common.EmptyHash, err
		}

		kHash := k.Hash()
		if err = es.ContractTree.Update(kHash, v); err != nil {
			return common.EmptyHash, err
		}
	}

	treeShardsRootHash := common.EmptyHash
	if len(es.ChildChainBlocks) > 0 {
		treeShards := NewShardBlocksTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.ShardBlocksTrieTableName(blockId)))
		for k, hash := range es.ChildChainBlocks {
			if err := treeShards.Update(k, hash.Uint256()); err != nil {
				return common.EmptyHash, err
			}
		}
		treeShardsRootHash = treeShards.RootHash()
	}

	for i, m := range es.InMessages {
		if err := es.InMessageTree.Update(types.MessageIndex(i), m); err != nil {
			return common.EmptyHash, err
		}
	}

	var msgIndex types.MessageIndex
	for _, messages := range es.OutMessages {
		for _, m := range messages {
			if err := es.OutMessageTree.Update(msgIndex, m); err != nil {
				return common.EmptyHash, err
			}
			msgIndex++
		}
	}

	if len(es.InMessages) != len(es.Receipts) {
		return common.EmptyHash, fmt.Errorf("number of messages does not match number of receipts: %d != %d", len(es.InMessages), len(es.Receipts))
	}
	for i, msg := range es.InMessages {
		if msg.Hash() != es.Receipts[i].MsgHash {
			return common.EmptyHash, fmt.Errorf("receipt hash doesn't match its message #%d", i)
		}
	}

	msgStart := 0
	for i, r := range es.Receipts {
		msgHash := es.InMessages[i].Hash()
		r.OutMsgIndex = uint32(msgStart)
		r.OutMsgNum = uint32(len(es.OutMessages[msgHash]))

		if err := es.ReceiptTree.Update(types.MessageIndex(i), r); err != nil {
			return common.EmptyHash, err
		}
		msgStart += len(es.OutMessages[msgHash])
	}

	block := types.Block{
		Id:                  blockId,
		PrevBlock:           es.PrevBlock,
		SmartContractsRoot:  es.ContractTree.RootHash(),
		InMessagesRoot:      es.InMessageTree.RootHash(),
		OutMessagesRoot:     es.OutMessageTree.RootHash(),
		OutMessagesNum:      msgIndex,
		ReceiptsRoot:        es.ReceiptTree.RootHash(),
		ChildBlocksRootHash: treeShardsRootHash,
		MasterChainHash:     es.MasterChain,
		Timestamp:           es.Timer.Now(),
	}

	if TraceBlocksEnabled {
		blocksTracer.Trace(es, &block)
	}

	blockHash := block.Hash()

	err := db.WriteBlock(es.tx, es.ShardId, &block)
	if err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}

func (es *ExecutionState) GetInMessageHash() common.Hash {
	return es.InMessageHash
}

func (es *ExecutionState) RoTx() db.RoTx {
	return es.tx
}

func (es *ExecutionState) CallVerifyExternal(message *types.Message, account *AccountState) bool {
	methodSignature := "verifyExternal(uint256,bytes)"
	methodSelector := crypto.Keccak256([]byte(methodSignature))[:4]
	argSpec := vm.VerifySignatureArgs()[1:] // skip first arg (pubkey)
	hash, err := message.SigningHash()
	if err != nil {
		return false
	}
	argData, err := argSpec.Pack(hash.Big(), ([]byte)(message.Signature))
	if err != nil {
		logger.Error().Err(err).Msg("failed to pack arguments")
		return false
	}
	calldata := append(methodSelector, argData...) //nolint:gocritic

	blockContext := NewEVMBlockContext(es)
	evm := vm.NewEVM(blockContext, es)
	ret, _, err := evm.StaticCall((vm.AccountRef)(account.address), account.address, calldata, ExternalMessageVerificationMaxGas)
	if err != nil || !bytes.Equal(ret, common.LeftPadBytes([]byte{1}, 32)) {
		return false
	}
	return true
}
