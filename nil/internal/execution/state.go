package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

var logger = logging.NewLogger("execution")

const TraceBlocksEnabled = false

var ExternalMessageVerificationMaxGas types.Gas = 100_000

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

type ExecutionState struct {
	tx               db.RwTx
	Timer            common.Timer
	ContractTree     *ContractTrie
	InMessageTree    *MessageTrie
	OutMessageTree   *MessageTrie
	ReceiptTree      *ReceiptTrie
	PrevBlock        common.Hash
	MainChainHash    common.Hash
	ShardId          types.ShardId
	ChildChainBlocks map[types.ShardId]common.Hash
	// Current gas price.
	GasPrice types.Value

	InMessageHash common.Hash
	Logs          map[common.Hash][]*types.Log

	Accounts        map[types.Address]*AccountState
	InMessages      []*types.Message
	InMessageHashes []common.Hash

	// OutMessages holds outbound messages for every transaction in the executed block, where key is hash of Message that sends the message
	OutMessages map[common.Hash][]*types.OutboundMessage

	Receipts []*types.Receipt
	Errors   map[common.Hash]error

	// Transient storage
	transientStorage transientStorage

	// The refund counter, also used by state transitioning.
	refund uint64

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionId int
	revertId       int

	// If true, log every instruction execution.
	TraceVm bool

	Accessor *StateAccessor

	// Pointer to currently executed VM
	evm *vm.EVM

	gasPriceScale float64
}

type ExecutionResult struct {
	ReturnData     []byte
	Error          *types.MessageError
	FatalError     error
	CoinsUsed      types.Value
	CoinsForwarded types.Value
}

func NewExecutionResult() *ExecutionResult {
	return &ExecutionResult{
		ReturnData: []byte{},
	}
}

func (e *ExecutionResult) SetError(err *types.MessageError) *ExecutionResult {
	e.Error = err
	return e
}

func (e *ExecutionResult) SetFatal(err error) *ExecutionResult {
	e.FatalError = err
	return e
}

func (e *ExecutionResult) SetMsgErrorOrFatal(err error) *ExecutionResult {
	if msgErr := (*types.MessageError)(nil); errors.As(err, &msgErr) {
		e.SetError(msgErr)
	} else {
		e.SetFatal(err)
	}
	return e
}

func (e *ExecutionResult) SetUsed(value types.Value) *ExecutionResult {
	e.CoinsUsed = value
	return e
}

func (e *ExecutionResult) SetForwarded(value types.Value) *ExecutionResult {
	e.CoinsForwarded = value
	return e
}

func (e *ExecutionResult) SetReturnData(data []byte) *ExecutionResult {
	e.ReturnData = data
	return e
}

func (e *ExecutionResult) GetLeftOverValue(value types.Value) types.Value {
	return value.Sub(e.CoinsUsed).Sub(e.CoinsForwarded)
}

func (e *ExecutionResult) Failed() bool {
	return e.Error != nil || e.FatalError != nil
}

func (e *ExecutionResult) IsFatal() bool {
	return e.FatalError != nil
}

func (e *ExecutionResult) GetError() error {
	if e.FatalError != nil {
		return e.FatalError
	}
	if e.Error != nil {
		return e.Error.Unwrap()
	}
	return nil
}

type revision struct {
	id           int
	journalIndex int
}

var _ vm.StateDB = new(ExecutionState)

func NewAccountStateReader(account *AccountState) *AccountStateReader {
	return &AccountStateReader{
		CurrencyTrieReader: account.CurrencyTree.BaseMPTReader,
	}
}

func NewAccountState(es *ExecutionState, addr types.Address, account *types.SmartContract) (*AccountState, error) {
	shardId := addr.ShardId()

	accountState := &AccountState{
		db:           es,
		address:      addr,
		CurrencyTree: NewDbCurrencyTrie(es.tx, shardId),
		StorageTree:  NewDbStorageTrie(es.tx, shardId),

		Tx:    es.tx,
		State: make(Storage),
	}

	if account != nil {
		accountState.Balance = account.Balance
		accountState.CurrencyTree.SetRootHash(account.CurrencyRoot)
		accountState.StorageTree.SetRootHash(account.StorageRoot)
		accountState.CodeHash = account.CodeHash
		var err error
		accountState.Code, err = db.ReadCode(es.tx, shardId, account.CodeHash)
		if err != nil {
			return nil, err
		}
		accountState.ExtSeqno = account.ExtSeqno
		accountState.Seqno = account.Seqno
	}

	return accountState, nil
}

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(es *ExecutionState) (*vm.BlockContext, error) {
	header, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	lastBlockId := uint64(0)
	if header != nil {
		lastBlockId = header.Id.Uint64()
	}
	return &vm.BlockContext{
		GetHash:     getHashFn(es, header),
		BlockNumber: lastBlockId,
		Random:      &common.EmptyHash,
		BaseFee:     big.NewInt(10),
		BlobBaseFee: big.NewInt(10),
		Time:        uint64(time.Now().Second()),
	}, nil
}

func NewROExecutionState(tx db.RoTx, shardId types.ShardId, blockHash common.Hash, timer common.Timer, gasPriceScale float64) (*ExecutionState, error) {
	return NewExecutionState(&db.RwWrapper{RoTx: tx}, shardId, blockHash, timer, gasPriceScale)
}

func NewROExecutionStateForShard(tx db.RoTx, shardId types.ShardId, timer common.Timer, gasPriceScale float64) (*ExecutionState, error) {
	return NewExecutionStateForShard(&db.RwWrapper{RoTx: tx}, shardId, timer, gasPriceScale)
}

func NewExecutionState(tx db.RwTx, shardId types.ShardId, blockHash common.Hash, timer common.Timer, gasPriceScale float64) (*ExecutionState, error) {
	accessor, err := NewStateAccessor()
	if err != nil {
		return nil, err
	}
	var gasPrice types.Value
	if blockHash != common.EmptyHash {
		block, err := accessor.Access(tx, shardId).GetBlock().ByHash(blockHash)
		if err != nil {
			return nil, err
		}
		gasPrice = block.Block().GasPrice
	} else {
		gasPrice = types.DefaultGasPrice
	}

	if gasPrice.IsZero() {
		return nil, errors.New("gas price is zero")
	}

	res := &ExecutionState{
		tx:               tx,
		Timer:            timer,
		PrevBlock:        blockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[types.ShardId]common.Hash{},
		Accounts:         map[types.Address]*AccountState{},
		OutMessages:      map[common.Hash][]*types.OutboundMessage{},
		Logs:             map[common.Hash][]*types.Log{},
		Errors:           map[common.Hash]error{},
		GasPrice:         gasPrice,

		journal:          newJournal(),
		transientStorage: newTransientStorage(),

		Accessor: accessor,

		gasPriceScale: gasPriceScale,
	}
	return res, res.initTries()
}

func (es *ExecutionState) initTries() error {
	block, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	es.ContractTree = NewDbContractTrie(es.tx, es.ShardId)
	es.InMessageTree = NewDbMessageTrie(es.tx, es.ShardId)
	es.OutMessageTree = NewDbMessageTrie(es.tx, es.ShardId)
	es.ReceiptTree = NewDbReceiptTrie(es.tx, es.ShardId)
	if block != nil {
		es.ContractTree.SetRootHash(block.SmartContractsRoot)
	}

	return nil
}

func NewExecutionStateForShard(tx db.RwTx, shardId types.ShardId, timer common.Timer, gasPriceScale float64) (*ExecutionState, error) {
	hash, err := db.ReadLastBlockHash(tx, shardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed getting last block: %w", err)
	}
	return NewExecutionState(tx, shardId, hash, timer, gasPriceScale)
}

func (es *ExecutionState) GetReceipt(msgIndex types.MessageIndex) (*types.Receipt, error) {
	return es.ReceiptTree.Fetch(msgIndex)
}

func (es *ExecutionState) GetAccountReader(addr types.Address) (*AccountStateReader, error) {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return nil, err
	}

	return NewAccountStateReader(acc), nil
}

func (es *ExecutionState) GetAccount(addr types.Address) (*AccountState, error) {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc, nil
	}

	addrHash := addr.Hash()

	data, err := es.ContractTree.Fetch(addrHash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	acc, err = NewAccountState(es, addr, data)
	if err != nil {
		return nil, err
	}
	es.Accounts[addr] = acc
	return acc, nil
}

func (es *ExecutionState) setAccountObject(acc *AccountState) {
	es.Accounts[acc.address] = acc
}

func (es *ExecutionState) AddAddressToAccessList(addr types.Address) {
}

// AddBalance adds amount to the account associated with addr.
func (es *ExecutionState) AddBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	stateObject, err := es.getOrNewAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	return stateObject.AddBalance(amount, reason)
}

// SubBalance subtracts amount from the account associated with addr.
func (es *ExecutionState) SubBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	stateObject, err := es.getOrNewAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	return stateObject.SubBalance(amount, reason)
}

func (es *ExecutionState) AddLog(log *types.Log) {
	es.journal.append(addLogChange{txhash: es.InMessageHash})
	es.Logs[es.InMessageHash] = append(es.Logs[es.InMessageHash], log)
}

// AddRefund adds gas to the refund counter
func (es *ExecutionState) AddRefund(gas uint64) {
	es.journal.append(refundChange{prev: es.refund})
	es.refund += gas
}

// GetRefund returns the current value of the refund counter.
func (es *ExecutionState) GetRefund() uint64 {
	return es.refund
}

func (es *ExecutionState) AddSlotToAccessList(addr types.Address, slot common.Hash) {
}

func (es *ExecutionState) AddressInAccessList(addr types.Address) bool {
	return true // FIXME
}

func (es *ExecutionState) Empty(addr types.Address) (bool, error) {
	acc, err := es.GetAccount(addr)
	return acc == nil || acc.empty(), err
}

func (es *ExecutionState) Exists(addr types.Address) (bool, error) {
	acc, err := es.GetAccount(addr)
	return acc != nil, err
}

func (es *ExecutionState) GetCode(addr types.Address) ([]byte, common.Hash, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return nil, common.Hash{}, err
	}
	return acc.Code, acc.CodeHash, nil
}

func (es *ExecutionState) GetCommittedState(types.Address, common.Hash) common.Hash {
	return common.EmptyHash
}

// Snapshot returns an identifier for the current revision of the state.
func (es *ExecutionState) Snapshot() int {
	id := es.nextRevisionId
	es.nextRevisionId++
	es.validRevisions = append(es.validRevisions, revision{id, es.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (es *ExecutionState) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(es.validRevisions), func(i int) bool {
		return es.validRevisions[i].id >= revid
	})
	if idx == len(es.validRevisions) || es.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := es.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	es.journal.revert(es, snapshot)
	es.validRevisions = es.validRevisions[:idx]
}

func (es *ExecutionState) GetStorageRoot(addr types.Address) (common.Hash, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return common.Hash{}, err
	}
	return acc.StorageTree.RootHash(), nil
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (es *ExecutionState) SetTransientState(addr types.Address, key, value common.Hash) {
	prev := es.GetTransientState(addr, key)
	if prev == value {
		return
	}
	es.journal.append(transientStorageChange{
		account:  &addr,
		key:      key,
		prevalue: prev,
	})
	es.setTransientState(addr, key, value)
}

// setTransientState is a lower level setter for transient storage. It
// is called during a revert to prevent modifications to the journal.
func (es *ExecutionState) setTransientState(addr types.Address, key, value common.Hash) {
	es.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (es *ExecutionState) GetTransientState(addr types.Address, key common.Hash) common.Hash {
	return es.transientStorage.Get(addr, key)
}

// SelfDestruct marks the given account as self-destructed.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// GetAccount will return a non-nil account after SelfDestruct.
func (es *ExecutionState) selfDestruct(stateObject *AccountState) {
	es.journal.append(selfDestructChange{
		account:     &stateObject.address,
		prev:        stateObject.selfDestructed,
		prevbalance: stateObject.Balance,
	})
	stateObject.selfDestructed = true
	stateObject.Balance = types.Value{}
}

func (es *ExecutionState) Selfdestruct6780(addr types.Address) error {
	stateObject, err := es.GetAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	if stateObject.newContract {
		es.selfDestruct(stateObject)
	}
	return nil
}

func (es *ExecutionState) HasSelfDestructed(addr types.Address) (bool, error) {
	stateObject, err := es.GetAccount(addr)
	if err != nil || stateObject == nil {
		return false, err
	}
	return stateObject.selfDestructed, nil
}

func (es *ExecutionState) SetCode(addr types.Address, code []byte) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.SetCode(types.Code(code).Hash(), code)
	return nil
}

func (es *ExecutionState) EnableVmTracing() {
	es.evm.Config.Tracer = &tracing.Hooks{
		OnOpcode: func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
			for i, item := range scope.StackData() {
				logger.Debug().Msgf("     %d: %s", i, item.String())
			}
			logger.Debug().Msgf("%04x: %s", pc, vm.OpCode(op).String())
		},
	}
}

func (es *ExecutionState) SetInitState(addr types.Address, message *types.Message) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.Seqno = message.Seqno

	if err := es.newVm(message.IsInternal(), message.From); err != nil {
		return err
	}
	defer es.resetVm()

	_, deployAddr, _, err := es.evm.Deploy(addr, vm.AccountRef{}, message.Data, uint64(100000) /* gas */, uint256.NewInt(0))
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
func (es *ExecutionState) SubRefund(gas uint64) {
	es.journal.append(refundChange{prev: es.refund})
	if gas > es.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, es.refund))
	}
	es.refund -= gas
}

func (es *ExecutionState) GetState(addr types.Address, key common.Hash) (common.Hash, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return common.EmptyHash, err
	}
	return acc.GetState(key)
}

func (es *ExecutionState) SetState(addr types.Address, key common.Hash, val common.Hash) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	return acc.SetState(key, val)
}

// SetStorage replaces the entire storage for the specified account with given
// storage. This function should only be used for debugging.
func (es *ExecutionState) SetStorage(addr types.Address, storage Storage) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.SetStorage(storage)
	return nil
}

func (es *ExecutionState) GetBalance(addr types.Address) (types.Value, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return types.Value{}, err
	}
	return acc.Balance, nil
}

func (es *ExecutionState) GetSeqno(addr types.Address) (types.Seqno, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return 0, err
	}
	return acc.Seqno, nil
}

func (es *ExecutionState) GetExtSeqno(addr types.Address) (types.Seqno, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return 0, err
	}
	return acc.ExtSeqno, nil
}

func (es *ExecutionState) getOrNewAccount(addr types.Address) (*AccountState, error) {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return acc, nil
	}
	return es.createAccount(addr)
}

func (es *ExecutionState) SetBalance(addr types.Address, balance types.Value) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.SetBalance(balance)
	return nil
}

func (es *ExecutionState) SetSeqno(addr types.Address, seqno types.Seqno) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.SetSeqno(seqno)
	return nil
}

func (es *ExecutionState) SetExtSeqno(addr types.Address, seqno types.Seqno) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.SetExtSeqno(seqno)
	return nil
}

func (es *ExecutionState) SetMainChainHash(hash common.Hash) {
	es.MainChainHash = hash
}

func (es *ExecutionState) SetShardHash(shardId types.ShardId, hash common.Hash) {
	es.ChildChainBlocks[shardId] = hash
}

func (es *ExecutionState) CreateAccount(addr types.Address) error {
	_, err := es.createAccount(addr)
	return err
}

func (es *ExecutionState) createAccount(addr types.Address) (*AccountState, error) {
	if addr.ShardId() != es.ShardId {
		return nil, fmt.Errorf("Attempt to create account %v from %v shard on %v shard", addr, addr.ShardId(), es.ShardId)
	}
	acc, err := es.GetAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return nil, errors.New("account already exists")
	}

	es.journal.append(createObjectChange{account: &addr})

	accountState, err := NewAccountState(es, addr, nil)
	if err != nil {
		return nil, err
	}
	es.Accounts[addr] = accountState
	return accountState, nil
}

// CreateContract is used whenever a contract is created. This may be preceded
// by CreateAccount, but that is not required if it already existed in the
// state due to funds sent beforehand.
// This operation sets the 'newContract'-flag, which is required in order to
// correctly handle EIP-6780 'delete-in-same-transaction' logic.
func (es *ExecutionState) CreateContract(addr types.Address) error {
	obj, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if !obj.newContract {
		obj.newContract = true
		es.journal.append(createContractChange{account: addr})
	}
	return nil
}

// Contract is regarded as existent if any of these three conditions is met:
// - the nonce is non-zero
// - the code is non-empty
// - the storage is non-empty
func (es *ExecutionState) ContractExists(address types.Address) (bool, error) {
	_, contractHash, err := es.GetCode(address)
	if err != nil {
		return false, err
	}
	storageRoot, err := es.GetStorageRoot(address)
	if err != nil {
		return false, err
	}
	seqno, err := es.GetSeqno(address)
	if err != nil {
		return false, err
	}
	return seqno != 0 ||
		(contractHash != common.EmptyHash) || // non-empty code
		(storageRoot != common.EmptyHash), nil // non-empty storage
}

func (es *ExecutionState) AddInMessage(message *types.Message) {
	// We store a copy of the message, because the original message will be modified.
	es.InMessages = append(es.InMessages, common.CopyPtr(message))
	es.InMessageHash = message.Hash()
	es.InMessageHashes = append(es.InMessageHashes, es.InMessageHash)
}

func (es *ExecutionState) DropInMessage() {
	es.InMessages = es.InMessages[:len(es.InMessages)-1]
	es.InMessageHashes = es.InMessageHashes[:len(es.InMessageHashes)-1]
	check.PanicIfNotf(len(es.InMessages) == len(es.InMessageHashes), "InMessages and InMessageHashes should have the same length")
	if len(es.InMessageHashes) > 0 {
		es.InMessageHash = es.InMessageHashes[len(es.InMessages)-1]
	} else {
		es.InMessageHash = common.EmptyHash
	}
}

func (es *ExecutionState) AppendOutMessageForTx(txId common.Hash, msg *types.Message) {
	outMsg := &types.OutboundMessage{Message: msg, ForwardKind: types.ForwardKindNone}
	es.OutMessages[txId] = append(es.OutMessages[txId], outMsg)
}

func (es *ExecutionState) AddOutInternal(caller types.Address, payload *types.InternalMessagePayload) (*types.Message, error) {
	seqno, err := es.GetSeqno(caller)
	if err != nil {
		return nil, err
	}

	// TODO:	This happens when we send refunds from uninitialized accounts when transferring money to them.
	//			For now we will write all such refunds with identical zero seqno, because we can't change seqno of uninitialized accounts.
	//			In future we should add transfer that is free on the reciepient's side, so that these transfers won't require refunds.
	if seqno != 0 {
		if seqno+1 < seqno {
			return nil, vm.ErrNonceUintOverflow
		}
		if err := es.SetSeqno(caller, seqno+1); err != nil {
			return nil, err
		}
	}

	msg := payload.ToMessage(caller, seqno)

	// In case of bounce message, we don't debit currency from account
	// In case of refund message, we don't transfer currencies
	if !msg.IsBounce() && !msg.IsRefund() {
		acc, err := es.GetAccount(msg.From)
		if err != nil {
			return nil, err
		}
		for _, currency := range msg.Currency {
			balance := acc.GetCurrencyBalance(currency.Currency)
			if balance.Cmp(currency.Balance) < 0 {
				return nil, fmt.Errorf("%w: %s < %s, currency %s",
					vm.ErrInsufficientBalance, balance, currency.Balance, currency.Currency)
			}
			if err := es.SubCurrency(msg.From, currency.Currency, currency.Balance); err != nil {
				return nil, err
			}
		}
	}

	logger.Trace().
		Stringer(logging.FieldMessageFrom, msg.From).
		Stringer(logging.FieldMessageTo, msg.To).
		Msg("Outbound message added")

	es.journal.append(outMessagesChange{
		msgHash: es.InMessageHash,
		index:   len(es.OutMessages[es.InMessageHash]),
	})

	outMsg := &types.OutboundMessage{Message: msg, ForwardKind: payload.ForwardKind}
	es.OutMessages[es.InMessageHash] = append(es.OutMessages[es.InMessageHash], outMsg)

	return msg, nil
}

func (es *ExecutionState) sendBounceMessage(msg *types.Message, execResult *ExecutionResult) (bool, error) {
	if msg.Value.IsZero() && len(msg.Currency) == 0 {
		return false, nil
	}
	if msg.BounceTo == types.EmptyAddress {
		logger.Debug().Stringer(logging.FieldMessageHash, msg.Hash()).Msg("Bounce message not sent, no bounce address")
		return false, nil
	}

	data, err := contracts.NewCallData(contracts.NameNilBounceable, "bounce", execResult.Error.Error())
	if err != nil {
		return false, err
	}

	check.PanicIfNotf(execResult.CoinsForwarded.IsZero(), "CoinsForwarded should be zero when sending bounce message")
	toReturn := msg.FeeCredit.Sub(execResult.CoinsUsed)

	bounceMsg := &types.InternalMessagePayload{
		Bounce:    true,
		To:        msg.BounceTo,
		RefundTo:  msg.RefundTo,
		Value:     msg.Value,
		Currency:  msg.Currency,
		Data:      data,
		FeeCredit: toReturn,
	}
	if _, err = es.AddOutInternal(msg.To, bounceMsg); err != nil {
		return false, err
	}
	logger.Debug().Stringer(logging.FieldMessageFrom, msg.To).Stringer(logging.FieldMessageTo, msg.BounceTo).Msg("Bounce message sent")
	return true, nil
}

func (es *ExecutionState) HandleMessage(ctx context.Context, msg *types.Message, payer Payer) *ExecutionResult {
	if err := buyGas(payer, msg); err != nil {
		return NewExecutionResult().SetError(types.NewMessageError(types.MessageStatusBuyGas, err))
	}
	if err := msg.VerifyFlags(); err != nil {
		return NewExecutionResult().SetError(types.NewMessageError(types.MessageStatusValidation, err))
	}

	var res *ExecutionResult
	switch {
	case msg.IsRefund():
		return NewExecutionResult().SetFatal(es.HandleRefundMessage(ctx, msg))
	case msg.IsDeploy():
		res = es.HandleDeployMessage(ctx, msg)
	default:
		res = es.HandleExecutionMessage(ctx, msg)
	}
	bounced := false
	if res.Error != nil {
		revString := decodeRevertMessage(res.ReturnData)
		if revString != "" {
			res.Error.Inner = fmt.Errorf("%w: %s", res.Error, revString)
		}
		if msg.IsBounce() {
			logger.Error().Err(res.Error).Msg("VM returns error during bounce message processing")
		} else {
			logger.Error().Err(res.Error).Msg("execution msg failed")
			if msg.IsInternal() {
				var bounceErr error
				if bounced, bounceErr = es.sendBounceMessage(msg, res); bounceErr != nil {
					logger.Error().Err(bounceErr).Msg("Bounce message sent failed")
					return res.SetFatal(bounceErr)
				}
			}
		}
	} else {
		availableGas := msg.FeeCredit.Sub(res.CoinsUsed)
		var err error
		if res.CoinsForwarded, err = es.CalculateGasForwarding(availableGas); err != nil {
			es.RevertToSnapshot(es.revertId)
			res.Error = types.NewMessageError(types.MessageStatusForwardingFailed, err)
		}
	}
	// Gas is already refunded with the bounce message
	if !bounced {
		leftOverCredit := res.GetLeftOverValue(msg.FeeCredit)
		if msg.RefundTo == msg.To {
			acc, err := es.GetAccount(msg.To)
			check.PanicIfErr(err)
			check.PanicIfErr(acc.AddBalance(leftOverCredit, tracing.BalanceIncreaseRefund))
		} else {
			refundGas(payer, leftOverCredit)
		}
	}

	return res
}

func (es *ExecutionState) HandleDeployMessage(_ context.Context, message *types.Message) *ExecutionResult {
	addr := message.To
	deployMsg := types.ParseDeployPayload(message.Data)

	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Stringer(logging.FieldShardId, es.ShardId).
		Msg("Handling deploy message...")

	if err := es.newVm(message.IsInternal(), message.From); err != nil {
		return NewExecutionResult().SetFatal(err)
	}
	defer es.resetVm()

	gas := message.FeeCredit.ToGas(es.GasPrice)
	ret, addr, leftOver, err := es.evm.Deploy(addr, (vm.AccountRef)(message.From), deployMsg.Code(), gas.Uint64(), message.Value.Int())

	event := logger.Debug().Stringer(logging.FieldMessageTo, addr)
	if err != nil {
		event.Err(err).Msg("Contract deployment failed.")
	} else {
		event.Msg("Created new contract.")
	}

	return NewExecutionResult().
		SetMsgErrorOrFatal(es.evmToMessageError(err)).
		SetUsed((gas - types.Gas(leftOver)).ToValue(es.GasPrice)).
		SetReturnData(ret)
}

func (es *ExecutionState) HandleExecutionMessage(_ context.Context, message *types.Message) *ExecutionResult {
	check.PanicIfNot(message.IsExecution())
	addr := message.To
	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Msg("Handling execution message...")
	if err := es.newVm(message.IsInternal(), message.From); err != nil {
		return NewExecutionResult().SetFatal(err)
	}
	defer es.resetVm()

	if es.TraceVm {
		es.EnableVmTracing()
	}

	if message.IsExternal() {
		seqno, err := es.GetExtSeqno(addr)
		if err != nil {
			return NewExecutionResult().SetFatal(err)
		}
		if err := es.SetExtSeqno(addr, seqno+1); err != nil {
			return NewExecutionResult().SetFatal(err)
		}
	}

	es.revertId = es.Snapshot()

	es.evm.SetCurrencyTransfer(message.Currency)
	gas := message.FeeCredit.ToGas(es.GasPrice)
	ret, leftOver, err := es.evm.Call((vm.AccountRef)(message.From), addr, message.Data, gas.Uint64(), message.Value.Int())

	return NewExecutionResult().
		SetMsgErrorOrFatal(es.evmToMessageError(err)).
		SetUsed((gas - types.Gas(leftOver)).ToValue(es.GasPrice)).
		SetReturnData(ret)
}

// decodeRevertMessage decodes the revert message from the EVM revert data
func decodeRevertMessage(data []byte) string {
	if len(data) < 68 {
		return ""
	}

	data = data[68:]

	revString := string(data[:bytes.IndexByte(data, 0)])

	return revString
}

func (es *ExecutionState) HandleRefundMessage(_ context.Context, message *types.Message) error {
	err := es.AddBalance(message.To, message.Value, tracing.BalanceIncreaseRefund)
	logger.Debug().Err(err).Msgf("Refunded %s to %v", message.Value, message.To)
	return err
}

func (es *ExecutionState) AddReceipt(execResult *ExecutionResult) {
	status := types.MessageStatusSuccess
	if execResult.Failed() {
		status = execResult.Error.Status
	}

	r := &types.Receipt{
		Success:         !execResult.Failed(),
		Status:          status,
		GasUsed:         execResult.CoinsUsed.ToGas(es.GasPrice),
		Forwarded:       execResult.CoinsForwarded,
		MsgHash:         es.InMessageHash,
		Logs:            es.Logs[es.InMessageHash],
		ContractAddress: es.GetInMessage().To,
	}
	if execResult.Failed() {
		es.Errors[es.InMessageHash] = execResult.Error
	}
	es.Receipts = append(es.Receipts, r)
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
		treeShards := NewDbShardBlocksTrie(es.tx, es.ShardId, blockId)
		for k, hash := range es.ChildChainBlocks {
			if err := treeShards.Update(k, &hash); err != nil {
				return common.EmptyHash, err
			}
		}
		treeShardsRootHash = treeShards.RootHash()
	}

	var outMsgIndex types.MessageIndex
	for i, msg := range es.InMessages {
		if err := es.InMessageTree.Update(types.MessageIndex(i), msg); err != nil {
			return common.EmptyHash, err
		}
		// Put all outbound messages into the trie
		for _, m := range es.OutMessages[es.InMessageHashes[i]] {
			if err := es.OutMessageTree.Update(outMsgIndex, m.Message); err != nil {
				return common.EmptyHash, err
			}
			outMsgIndex++
		}
	}
	// Put all outbound messages transmitted over the topology into the trie
	for _, m := range es.OutMessages[common.EmptyHash] {
		if err := es.OutMessageTree.Update(outMsgIndex, m.Message); err != nil {
			return common.EmptyHash, err
		}
		outMsgIndex++
	}

	if assert.Enable {
		// Check that each outbound message belongs to some inbound message
		for outMsgHash := range es.OutMessages {
			if outMsgHash == common.EmptyHash {
				// Skip messages transmitted over the topology
				continue
			}
			found := false
			for _, msgHash := range es.InMessageHashes {
				if msgHash == outMsgHash {
					found = true
					break
				}
			}
			if !found {
				return common.EmptyHash, fmt.Errorf("outbound message %v does not belong to any inbound message", outMsgHash)
			}
		}
	}
	if len(es.InMessages) != len(es.Receipts) {
		return common.EmptyHash, fmt.Errorf("number of messages does not match number of receipts: %d != %d", len(es.InMessages), len(es.Receipts))
	}
	for i, msgHash := range es.InMessageHashes {
		if msgHash != es.Receipts[i].MsgHash {
			return common.EmptyHash, fmt.Errorf("receipt hash doesn't match its message #%d", i)
		}
	}

	// Update receipts trie
	msgStart := 0
	for i, r := range es.Receipts {
		msgHash := es.InMessageHashes[i]
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
		OutMessagesNum:      outMsgIndex,
		ReceiptsRoot:        es.ReceiptTree.RootHash(),
		ChildBlocksRootHash: treeShardsRootHash,
		MainChainHash:       es.MainChainHash,
		GasPrice:            es.GasPrice,
		// TODO(@klonD90): remove this field after changing explorer
		Timestamp: 0,
	}

	if TraceBlocksEnabled {
		blocksTracer.Trace(es, &block)
	}

	for k, v := range es.Errors {
		if err := db.WriteError(es.tx, k, v.Error()); err != nil {
			return common.EmptyHash, err
		}
	}

	blockHash := block.Hash()

	err := db.WriteBlock(es.tx, es.ShardId, &block)
	if err != nil {
		return common.EmptyHash, err
	}

	logger.Trace().
		Stringer(logging.FieldShardId, es.ShardId).
		Stringer(logging.FieldBlockNumber, blockId).
		Stringer(logging.FieldBlockHash, blockHash).
		Msgf("Committed new block with %d in-msgs and %d out-msgs", len(es.InMessages), block.OutMessagesNum)

	return blockHash, nil
}

func (es *ExecutionState) CalculateGasForwarding(initialAvailValue types.Value) (types.Value, error) {
	if len(es.OutMessages) == 0 {
		return types.NewZeroValue(), nil
	}
	var overflow bool

	availValue := initialAvailValue

	remainingFwdMessages := make([]*types.OutboundMessage, 0, len(es.OutMessages[es.InMessageHash]))
	percentageFwdMessages := make([]*types.OutboundMessage, 0, len(es.OutMessages[es.InMessageHash]))

	for _, msg := range es.OutMessages[es.InMessageHash] {
		switch msg.ForwardKind {
		case types.ForwardKindValue:
			diff, overflow := availValue.SubOverflow(msg.Message.FeeCredit)
			if overflow {
				return types.NewZeroValue(), fmt.Errorf("%w: not enough credit for ForwardKindValue: %v < %v",
					ErrMsgFeeForwarding, availValue, msg.Message.FeeCredit)
			}
			availValue = diff
		case types.ForwardKindPercentage:
			percentageFwdMessages = append(percentageFwdMessages, msg)
		case types.ForwardKindRemaining:
			remainingFwdMessages = append(remainingFwdMessages, msg)
		case types.ForwardKindNone:
			// Do nothing for non-forwarding message and do not set refundTo
			continue
		}
		if msg.RefundTo.IsEmpty() {
			msg.RefundTo = es.GetInMessage().RefundTo
		}
	}

	if len(percentageFwdMessages) != 0 {
		availValue0 := availValue
		for _, msg := range percentageFwdMessages {
			if !msg.FeeCredit.IsUint64() || msg.FeeCredit.Uint64() > 100 {
				return types.NewZeroValue(), fmt.Errorf("%w: invalid percentage value %v", ErrMsgFeeForwarding, msg.FeeCredit)
			}
			msg.FeeCredit = availValue0.Mul(msg.FeeCredit).Div(types.NewValueFromUint64(100))

			availValue, overflow = availValue.SubOverflow(msg.Message.FeeCredit)
			if overflow {
				return types.NewZeroValue(), fmt.Errorf("%w: sum of percentage is more than 100", ErrMsgFeeForwarding)
			}
		}
	}

	if len(remainingFwdMessages) != 0 {
		availValue0 := availValue
		portion := availValue0.Div(types.NewValueFromUint64(uint64(len(remainingFwdMessages))))
		for _, msg := range remainingFwdMessages {
			msg.FeeCredit = portion
			availValue = availValue.Sub(portion)
		}
		if !availValue.IsZero() {
			// If there is some remaining value due to division inaccuracy, credit it to the first message.
			remainingFwdMessages[0].FeeCredit = remainingFwdMessages[0].FeeCredit.Add(availValue)
			availValue = types.NewZeroValue()
		}
	}

	return initialAvailValue.Sub(availValue), nil
}

func (es *ExecutionState) IsInternalMessage() bool {
	// If contract calls another contract using EVM's call(depth > 1), we treat it as an internal message.
	return es.GetInMessage().IsInternal() || es.evm.GetDepth() > 1
}

func (es *ExecutionState) GetInMessage() *types.Message {
	if len(es.InMessages) == 0 {
		return nil
	}
	return es.InMessages[len(es.InMessages)-1]
}

func (es *ExecutionState) GetShardID() types.ShardId {
	return es.ShardId
}

func (es *ExecutionState) RoTx() db.RoTx {
	return es.tx
}

func (es *ExecutionState) CallVerifyExternal(message *types.Message, account *AccountState) *ExecutionResult {
	methodSignature := "verifyExternal(uint256,bytes)"
	methodSelector := crypto.Keccak256([]byte(methodSignature))[:4]
	argSpec := vm.VerifySignatureArgs()[1:] // skip first arg (pubkey)
	hash, err := message.SigningHash()
	if err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("message.SigningHash() failed: %w", err))
	}
	argData, err := argSpec.Pack(hash.Big(), ([]byte)(message.Signature))
	if err != nil {
		logger.Error().Err(err).Msg("failed to pack arguments")
		return NewExecutionResult().SetFatal(err)
	}
	calldata := append(methodSelector, argData...) //nolint:gocritic

	if err := es.newVm(message.IsInternal(), message.From); err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("newVm failed: %w", err))
	}
	defer es.resetVm()

	gasCreditLimit := ExternalMessageVerificationMaxGas
	gasAvailable := account.Balance.ToGas(es.GasPrice)

	if gasAvailable.Lt(gasCreditLimit) {
		gasCreditLimit = gasAvailable
	}

	ret, leftOverGas, err := es.evm.StaticCall((vm.AccountRef)(account.address), account.address, calldata, gasCreditLimit.Uint64())
	if err != nil {
		msgErr := types.NewMessageError(types.MessageStatusExternalVerificationFailed, err)
		return NewExecutionResult().SetError(msgErr)
	}
	if !bytes.Equal(ret, common.LeftPadBytes([]byte{1}, 32)) {
		return NewExecutionResult().SetError(types.NewMessageError(types.MessageStatusExternalVerificationFailed, ErrExternalMsgVerification))
	}
	res := NewExecutionResult()
	spentGas := gasCreditLimit.Sub(types.Gas(leftOverGas))
	res.CoinsUsed = spentGas.ToValue(es.GasPrice)
	check.PanicIfErr(account.SubBalance(res.CoinsUsed, tracing.BalanceDecreaseVerifyExternal))
	return res
}

func (es *ExecutionState) AddCurrency(addr types.Address, currencyId types.CurrencyId, amount types.Value) error {
	logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", common.Hash(currencyId)).
		Msg("Add currency")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetCurrencyBalance(currencyId)
	newBalance := balance.Add(amount)
	// Amount can be negative(currency burning). So, if the new balance is negative, set it to 0
	if newBalance.Cmp(types.Value{}) < 0 {
		newBalance = types.Value{}
	}
	acc.SetCurrencyBalance(currencyId, newBalance)

	return nil
}

func (es *ExecutionState) SubCurrency(addr types.Address, currencyId types.CurrencyId, amount types.Value) error {
	logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", common.Hash(currencyId)).
		Msg("Sub currency")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetCurrencyBalance(currencyId)
	if balance.Cmp(amount) < 0 {
		return fmt.Errorf("%w: %s < %s, currency %s",
			vm.ErrInsufficientBalance, balance, amount, currencyId)
	}
	acc.SetCurrencyBalance(currencyId, balance.Sub(amount))

	return nil
}

func (es *ExecutionState) GetCurrencies(addr types.Address) map[types.CurrencyId]types.Value {
	acc, err := es.GetAccountReader(addr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get account")
		return nil
	}
	if acc == nil {
		return nil
	}

	res := make(map[types.CurrencyId]types.Value)
	for _, kv := range acc.CurrencyTrieReader.Iterate() {
		var c types.CurrencyBalance
		c.Currency = types.CurrencyId(kv.Key)
		if err := c.Balance.UnmarshalSSZ(kv.Value); err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal currency balance")
			continue
		}
		res[c.Currency] = c.Balance
	}

	return res
}

func (es *ExecutionState) GetGasPrice(shardId types.ShardId) (types.Value, error) {
	return db.ReadGasPerShard(es.tx, shardId)
}

func (es *ExecutionState) SetCurrencyTransfer(currencies []types.CurrencyBalance) {
	es.evm.SetCurrencyTransfer(currencies)
}

func (es *ExecutionState) newVm(internal bool, origin types.Address) error {
	blockContext, err := NewEVMBlockContext(es)
	if err != nil {
		return err
	}
	es.evm = vm.NewEVM(blockContext, es, origin)
	es.evm.IsAsyncCall = internal
	return nil
}

func (es *ExecutionState) SetVm(evm *vm.EVM) {
	es.evm = evm
}

func (es *ExecutionState) resetVm() {
	es.evm = nil
}

func (es *ExecutionState) evmToMessageError(err error) error {
	if !vm.IsVMError(err) {
		return err
	}

	switch {
	case errors.Is(err, vm.ErrOutOfGas):
		return types.NewMessageError(types.MessageStatusOutOfGas, err)
	case errors.Is(err, vm.ErrCodeStoreOutOfGas):
		return types.NewMessageError(types.MessageStatusCodeStoreOutOfGas, err)
	case errors.Is(err, vm.ErrDepth):
		return types.NewMessageError(types.MessageStatusDepth, err)
	case errors.Is(err, vm.ErrInsufficientBalance):
		return types.NewMessageError(types.MessageStatusInsufficientBalance, err)
	case errors.Is(err, vm.ErrContractAddressCollision):
		return types.NewMessageError(types.MessageStatusContractAddressCollision, err)
	case errors.Is(err, vm.ErrExecutionReverted):
		return types.NewMessageError(types.MessageStatusExecutionReverted, err)
	case errors.Is(err, vm.ErrMaxCodeSizeExceeded):
		return types.NewMessageError(types.MessageStatusMaxCodeSizeExceeded, err)
	case errors.Is(err, vm.ErrMaxInitCodeSizeExceeded):
		return types.NewMessageError(types.MessageStatusMaxInitCodeSizeExceeded, err)
	case errors.Is(err, vm.ErrInvalidJump):
		return types.NewMessageError(types.MessageStatusInvalidJump, err)
	case errors.Is(err, vm.ErrWriteProtection):
		return types.NewMessageError(types.MessageStatusWriteProtection, err)
	case errors.Is(err, vm.ErrReturnDataOutOfBounds):
		return types.NewMessageError(types.MessageStatusReturnDataOutOfBounds, err)
	case errors.Is(err, vm.ErrGasUintOverflow):
		return types.NewMessageError(types.MessageStatusGasUintOverflow, err)
	case errors.Is(err, vm.ErrInvalidCode):
		return types.NewMessageError(types.MessageStatusInvalidCode, err)
	case errors.Is(err, vm.ErrNonceUintOverflow):
		return types.NewMessageError(types.MessageStatusNonceUintOverflow, err)
	case errors.Is(err, vm.ErrInvalidInputLength):
		return types.NewMessageError(types.MessageStatusInvalidInputLength, err)
	case errors.Is(err, vm.ErrCrossShardMessage):
		return types.NewMessageError(types.MessageStatusCrossShardMessage, err)
	case errors.Is(err, vm.ErrMessageToMainShard):
		return types.NewMessageError(types.MessageStatusMessageToMainShard, err)
	}
	return types.NewMessageError(types.MessageStatusExecution, err)
}

func (es *ExecutionState) UpdateGasPrice() {
	block, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
	if err != nil {
		// If we can't read the previous block, we don't change the gas price
		es.GasPrice = types.DefaultGasPrice
		logger.Error().Err(err).Msg("failed to read previous block, gas price won't be changed")
		return
	}

	es.GasPrice = block.GasPrice

	decreasePerBlock := types.NewValueFromUint64(1)
	maxGasPrice := types.NewValueFromUint64(100)

	gasIncrease := uint64(math.Ceil(float64(block.OutMessagesNum) * es.gasPriceScale))
	var overflow bool
	es.GasPrice, overflow = es.GasPrice.AddOverflow(types.NewValueFromUint64(gasIncrease))
	// Check if new gas price is less than the current one (overflow case) or greater than the max allowed
	if overflow || es.GasPrice.Cmp(maxGasPrice) > 0 {
		es.GasPrice = maxGasPrice
	}
	if es.GasPrice.Cmp(decreasePerBlock) >= 0 {
		es.GasPrice = es.GasPrice.Sub(decreasePerBlock)
	}
	if es.GasPrice.Cmp(types.DefaultGasPrice) < 0 {
		es.GasPrice = types.DefaultGasPrice
	}
}
