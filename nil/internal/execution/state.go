package execution

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
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
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/config"
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

const ExternalMessageVerificationMaxGas = types.Gas(100_000)

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
	DebugLogs     map[common.Hash][]*types.DebugLog

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

	shardAccessor *shardAccessor
	mainAccessor  *shardAccessor

	// Pointer to currently executed VM
	evm *vm.EVM

	gasPriceScale float64

	// wasSentRequest is true if the VM execution ended with sending a request message
	wasSentRequest bool

	configAccessor *config.ConfigAccessor
}

type ExecutionResult struct {
	ReturnData     []byte
	Error          types.ExecError
	FatalError     error
	CoinsUsed      types.Value
	CoinsForwarded types.Value
	DebugInfo      *vm.DebugInfo
}

func NewExecutionResult() *ExecutionResult {
	return &ExecutionResult{
		ReturnData: []byte{},
	}
}

func (e *ExecutionResult) SetError(err types.ExecError) *ExecutionResult {
	e.Error = err
	return e
}

func (e *ExecutionResult) SetFatal(err error) *ExecutionResult {
	e.FatalError = err
	return e
}

func (e *ExecutionResult) SetMsgErrorOrFatal(err error) *ExecutionResult {
	if msgErr := (types.ExecError)(nil); errors.As(err, &msgErr) {
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

func (e *ExecutionResult) SetDebugInfo(debugInfo *vm.DebugInfo) *ExecutionResult {
	e.DebugInfo = debugInfo
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
		return e.Error
	}
	return nil
}

type revision struct {
	id           int
	journalIndex int
}

var _ vm.StateDB = new(ExecutionState)

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(es *ExecutionState) (*vm.BlockContext, error) {
	data, err := es.shardAccessor.GetBlock().ByHash(es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	lastBlockId := uint64(0)
	var header *types.Block
	if err == nil {
		header = data.Block()
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
	stateAccessor := NewStateAccessor()
	shardAccessor := stateAccessor.Access(tx, shardId)
	mainAccessor := stateAccessor.Access(tx, types.MainShardId)

	var gasPrice types.Value
	if blockHash != common.EmptyHash {
		block, err := shardAccessor.GetBlock().ByHash(blockHash)
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
		DebugLogs:        map[common.Hash][]*types.DebugLog{},
		Errors:           map[common.Hash]error{},
		GasPrice:         gasPrice,

		journal:          newJournal(),
		transientStorage: newTransientStorage(),

		shardAccessor: shardAccessor,
		mainAccessor:  mainAccessor,

		gasPriceScale: gasPriceScale,
	}

	return res, res.initTries()
}

func (es *ExecutionState) initTries() error {
	data, err := es.shardAccessor.GetBlock().ByHash(es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	es.ContractTree = NewDbContractTrie(es.tx, es.ShardId)
	es.InMessageTree = NewDbMessageTrie(es.tx, es.ShardId)
	es.OutMessageTree = NewDbMessageTrie(es.tx, es.ShardId)
	es.ReceiptTree = NewDbReceiptTrie(es.tx, es.ShardId)
	if err == nil {
		es.ContractTree.SetRootHash(data.Block().SmartContractsRoot)
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

func (es *ExecutionState) GetConfigAccessor() *config.ConfigAccessor {
	if es.configAccessor == nil {
		var err error
		es.configAccessor, err = config.NewConfigAccessor(es.tx, es.ShardId, &es.MainChainHash)
		check.PanicIfErr(err)
	}
	return es.configAccessor
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
		return nil, fmt.Errorf("GetAccount failed: %w", err)
	}

	acc, err = NewAccountState(es, addr, data)
	if err != nil {
		return nil, fmt.Errorf("NewAccountState failed: %w", err)
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

func (es *ExecutionState) AddLog(log *types.Log) error {
	es.journal.append(addLogChange{txhash: es.InMessageHash})
	if len(es.Logs[es.InMessageHash]) >= types.ReceiptMaxLogsSize {
		return errors.New("too many logs")
	}
	es.Logs[es.InMessageHash] = append(es.Logs[es.InMessageHash], log)
	return nil
}

func (es *ExecutionState) AddDebugLog(log *types.DebugLog) error {
	if len(es.DebugLogs[es.InMessageHash]) >= types.ReceiptMaxDebugLogsSize {
		return errors.New("too many debug logs")
	}
	es.DebugLogs[es.InMessageHash] = append(es.DebugLogs[es.InMessageHash], log)
	return nil
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

	if err := es.newVm(message.IsInternal(), message.From, nil); err != nil {
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

func (es *ExecutionState) AddInMessage(message *types.Message) common.Hash {
	// We store a copy of the message, because the original message will be modified.
	es.InMessages = append(es.InMessages, common.CopyPtr(message))
	es.InMessageHash = message.Hash()
	es.InMessageHashes = append(es.InMessageHashes, es.InMessageHash)
	return es.InMessageHash
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

func (es *ExecutionState) AddOutRequestMessage(
	caller types.Address,
	payload *types.InternalMessagePayload,
	responseProcessingGas types.Gas,
	isAwait bool,
) (*types.Message, error) {
	msg, err := es.AddOutMessage(caller, payload)
	if err != nil {
		return nil, err
	}

	acc, err := es.GetAccount(caller)
	check.PanicIfErr(err)

	msg.RequestId = acc.FetchRequestId()

	// If inbound message is also a request, we need to add a new record to the request chain.
	inMsg := es.GetInMessage()
	if inMsg.IsRequest() {
		msg.RequestChain = make([]*types.AsyncRequestInfo, len(es.GetInMessage().RequestChain)+1)
		copy(msg.RequestChain, inMsg.RequestChain)
		msg.RequestChain[len(inMsg.RequestChain)] = &types.AsyncRequestInfo{
			Id:     inMsg.RequestId,
			Caller: inMsg.From,
		}
	} else if len(inMsg.RequestChain) != 0 {
		// If inbound message is a response, we need to copy the request chain from it.
		check.PanicIfNot(inMsg.IsResponse())
		msg.RequestChain = inMsg.RequestChain
	}

	if isAwait {
		es.wasSentRequest = true
		// Stop vm execution and save its state after the current instruction (call of precompile) is finished.
		es.evm.StopAndDumpState(responseProcessingGas)
	} else {
		context := &types.AsyncContext{
			IsAwait:               false,
			Data:                  payload.RequestContext,
			ResponseProcessingGas: responseProcessingGas,
		}
		acc.SetAsyncContext(types.MessageIndex(msg.RequestId), context)
	}

	return msg, nil
}

func (es *ExecutionState) AddOutMessage(caller types.Address, payload *types.InternalMessagePayload) (*types.Message, error) {
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
			if balance == nil {
				balance = &types.Value{}
			}
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
	if _, err = es.AddOutMessage(msg.To, bounceMsg); err != nil {
		return false, err
	}
	logger.Debug().Stringer(logging.FieldMessageFrom, msg.To).Stringer(logging.FieldMessageTo, msg.BounceTo).Msg("Bounce message sent")
	return true, nil
}

func (es *ExecutionState) SendResponseMessage(msg *types.Message, res *ExecutionResult) error {
	asyncResponsePayload := types.AsyncResponsePayload{
		Success:    !res.Failed(),
		ReturnData: res.ReturnData,
	}
	data, err := asyncResponsePayload.MarshalSSZ()
	if err != nil {
		return err
	}

	responsePayload := &types.InternalMessagePayload{
		Kind:        types.ResponseMessageKind,
		ForwardKind: types.ForwardKindRemaining,
		Data:        data,
	}

	// TODO: need to pay for response here
	// we pay for mem during VM execution, so likely big response isn't a problem
	responseMsg, err := es.AddOutMessage(msg.To, responsePayload)
	if err != nil {
		return err
	}

	// Send back value in case of failed message, thereby we don't need in a separate bounce message.
	if res.Failed() {
		responseMsg.Value = msg.Value
	}

	if msg.IsRequest() {
		responseMsg.To = msg.From
		responseMsg.RequestId = msg.RequestId
		responseMsg.RequestChain = msg.RequestChain
	} else {
		// We are processing a response message with requests chain. So get pending request from the chain and send
		// response to it.
		check.PanicIfNotf(msg.IsResponse(), "Message should be a response")
		responseMsg.To = msg.RequestChain[len(msg.RequestChain)-1].Caller
		responseMsg.RequestId = msg.RequestChain[len(msg.RequestChain)-1].Id
		responseMsg.RequestChain = msg.RequestChain[:len(msg.RequestChain)-1]
	}
	return nil
}

func (es *ExecutionState) HandleMessage(ctx context.Context, msg *types.Message, payer Payer) *ExecutionResult {
	if err := buyGas(payer, msg); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorBuyGas, err))
	}
	if err := msg.VerifyFlags(); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorValidation, err))
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
	responseWasSent := false
	bounced := false
	if msg.IsRequest() {
		if !es.wasSentRequest {
			if err := es.SendResponseMessage(msg, res); err != nil {
				return NewExecutionResult().SetFatal(fmt.Errorf("SendResponseMessage failed: %w\n", err))
			}
			bounced = true
			responseWasSent = true
		}
	} else if msg.IsResponse() && !es.wasSentRequest && len(msg.RequestChain) > 0 {
		// There is pending requests in the chain, so we need to send response to them.
		// But we don't send response if a new request was sent during the execution.
		if err := es.SendResponseMessage(msg, res); err != nil {
			return NewExecutionResult().SetFatal(fmt.Errorf("SendResponseMessage failed: %w\n", err))
		}
		responseWasSent = true
	}
	// We don't need bounce message for request, because it will be sent within the response message.
	if res.Error != nil && !responseWasSent {
		revString := decodeRevertMessage(res.ReturnData)
		if revString != "" {
			if types.IsVmError(res.Error) {
				res.Error = types.NewVmVerboseError(res.Error.Code(), revString)
			} else {
				res.Error = types.NewVerboseError(res.Error.Code(), revString)
			}
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
			res.Error = types.KeepOrWrapError(types.ErrorForwardingFailed, err)
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

	if err := es.newVm(message.IsInternal(), message.From, nil); err != nil {
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
		SetMsgErrorOrFatal(err).
		SetUsed((gas - types.Gas(leftOver)).ToValue(es.GasPrice)).
		SetReturnData(ret).SetDebugInfo(es.evm.DebugInfo)
}

func (es *ExecutionState) TryProcessResponse(message *types.Message) ([]byte, *vm.EvmRestoreData, *ExecutionResult) {
	if !message.IsResponse() {
		return message.Data, nil, nil
	}
	var restoreState *vm.EvmRestoreData
	var callData []byte

	check.PanicIfNot(message.RequestId != 0)
	acc, err := es.GetAccount(message.To)
	if err != nil {
		return nil, nil, NewExecutionResult().SetFatal(err)
	}
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, message.RequestId)
	context, err := acc.GetAndRemoveAsyncContext(types.MessageIndex(message.RequestId))
	if err != nil {
		return nil, nil, NewExecutionResult().SetFatal(fmt.Errorf("failed to get async context: %w", err))
	}

	responsePayload := new(types.AsyncResponsePayload)
	if err := responsePayload.UnmarshalSSZ(message.Data); err != nil {
		return nil, nil, NewExecutionResult().SetFatal(fmt.Errorf("AsyncResponsePayload unmarshal failed: %w", err))
	}

	message.FeeCredit = message.FeeCredit.Add(context.ResponseProcessingGas.ToValue(es.GasPrice))

	if context.IsAwait {
		// Restore VM state from the context
		restoreState = new(vm.EvmRestoreData)
		if err = restoreState.EvmState.UnmarshalSSZ(context.Data); err != nil {
			return nil, nil, NewExecutionResult().SetFatal(fmt.Errorf("context unmarshal failed: %w", err))
		}

		restoreState.ReturnData = responsePayload.ReturnData
		restoreState.Result = responsePayload.Success
	} else {
		if len(context.Data) < 4 {
			return nil, nil, NewExecutionResult().SetError(types.NewError(types.ErrorAwaitCallTooShortContextData))
		}
		contextData := context.Data[4:]
		bytesTy, _ := abi.NewType("bytes", "", nil)
		boolTy, _ := abi.NewType("bool", "", nil)
		args := abi.Arguments{
			abi.Argument{Name: "success", Type: boolTy},
			abi.Argument{Name: "returnData", Type: bytesTy},
			abi.Argument{Name: "context", Type: bytesTy},
		}
		if callData, err = args.Pack(responsePayload.Success, responsePayload.ReturnData, contextData); err != nil {
			return nil, nil, NewExecutionResult().SetFatal(err)
		}
		callData = append(context.Data[:4], callData...)
	}

	return callData, restoreState, nil
}

func (es *ExecutionState) HandleExecutionMessage(_ context.Context, message *types.Message) *ExecutionResult {
	check.PanicIfNot(message.IsExecution())
	addr := message.To
	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Stringer(logging.FieldMessageFlags, message.Flags).
		Msg("Handling execution message...")

	caller := (vm.AccountRef)(message.From)

	callData, restoreState, res := es.TryProcessResponse(message)
	if res != nil && res.Failed() {
		return res
	}

	if err := es.newVm(message.IsInternal(), message.From, restoreState); err != nil {
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
	ret, leftOver, err := es.evm.Call(caller, addr, callData, gas.Uint64(), message.Value.Int())

	return NewExecutionResult().
		SetMsgErrorOrFatal(err).
		SetUsed((gas - types.Gas(leftOver)).ToValue(es.GasPrice)).
		SetReturnData(ret).SetDebugInfo(es.evm.DebugInfo)
}

// decodeRevertMessage decodes the revert message from the EVM revert data
func decodeRevertMessage(data []byte) string {
	if len(data) <= 68 {
		return ""
	}

	data = data[68:]
	var revString string
	if index := bytes.IndexByte(data, 0); index > 0 {
		revString = string(data[:index])
	}
	return revString
}

func (es *ExecutionState) HandleRefundMessage(_ context.Context, message *types.Message) error {
	err := es.AddBalance(message.To, message.Value, tracing.BalanceIncreaseRefund)
	logger.Debug().Err(err).Msgf("Refunded %s to %v", message.Value, message.To)
	return err
}

func (es *ExecutionState) AddReceipt(execResult *ExecutionResult) {
	status := types.ErrorSuccess
	if execResult.Failed() {
		status = execResult.Error.Code()
	}

	r := &types.Receipt{
		Success:         !execResult.Failed(),
		Status:          status,
		GasUsed:         execResult.CoinsUsed.ToGas(es.GasPrice),
		Forwarded:       execResult.CoinsForwarded,
		MsgHash:         es.InMessageHash,
		Logs:            es.Logs[es.InMessageHash],
		DebugLogs:       es.DebugLogs[es.InMessageHash],
		ContractAddress: es.GetInMessage().To,
	}
	r.Bloom = types.CreateBloom(types.Receipts{r})

	if execResult.Failed() {
		es.Errors[es.InMessageHash] = execResult.Error
		if execResult.DebugInfo != nil {
			check.PanicIfNot(execResult.DebugInfo.Pc <= math.MaxUint32)
			r.FailedPc = uint32(execResult.DebugInfo.Pc)
		}
	}
	es.Receipts = append(es.Receipts, r)
}

func GetOutMessages(es *ExecutionState) []*types.Message {
	outMsgValues := make([]*types.Message, 0, len(es.InMessages))
	for i := range es.InMessages {
		// Put all outbound messages into the trie
		for _, m := range es.OutMessages[es.InMessageHashes[i]] {
			outMsgValues = append(outMsgValues, m.Message)
		}
	}
	// Put all outbound messages transmitted over the topology into the trie
	for _, m := range es.OutMessages[common.EmptyHash] {
		outMsgValues = append(outMsgValues, m.Message)
	}
	return outMsgValues
}

func (es *ExecutionState) Commit(blockId types.BlockNumber) (common.Hash, []*types.Message, error) {
	keys := make([]common.Hash, 0, len(es.Accounts))
	values := make([]*types.SmartContract, 0, len(es.Accounts))
	for k, acc := range es.Accounts {
		v, err := acc.Commit()
		if err != nil {
			return common.EmptyHash, nil, err
		}

		keys = append(keys, k.Hash())
		values = append(values, v)
	}
	if err := es.ContractTree.UpdateBatch(keys, values); err != nil {
		return common.EmptyHash, nil, err
	}

	treeShardsRootHash := common.EmptyHash
	if len(es.ChildChainBlocks) > 0 {
		treeShards := NewDbShardBlocksTrie(es.tx, es.ShardId, blockId)
		if err := UpdateFromMap(treeShards, es.ChildChainBlocks, func(v common.Hash) *common.Hash { return &v }); err != nil {
			return common.EmptyHash, nil, err
		}
		treeShardsRootHash = treeShards.RootHash()
	}

	inMsgKeys := make([]types.MessageIndex, 0, len(es.InMessages))
	inMsgValues := make([]*types.Message, 0, len(es.InMessages))
	for i, msg := range es.InMessages {
		inMsgKeys = append(inMsgKeys, types.MessageIndex(i))
		inMsgValues = append(inMsgValues, msg)
	}

	outMsgValues := GetOutMessages(es)
	outMsgKeys := make([]types.MessageIndex, 0, len(es.InMessages))
	for i := range outMsgValues {
		outMsgKeys = append(outMsgKeys, types.MessageIndex(i))
	}

	if err := es.InMessageTree.UpdateBatch(inMsgKeys, inMsgValues); err != nil {
		return common.EmptyHash, nil, err
	}
	if err := es.OutMessageTree.UpdateBatch(outMsgKeys, outMsgValues); err != nil {
		return common.EmptyHash, nil, err
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
				return common.EmptyHash, nil, fmt.Errorf("outbound message %v does not belong to any inbound message", outMsgHash)
			}
		}
	}
	if len(es.InMessages) != len(es.Receipts) {
		return common.EmptyHash, nil, fmt.Errorf("number of messages does not match number of receipts: %d != %d", len(es.InMessages), len(es.Receipts))
	}
	for i, msgHash := range es.InMessageHashes {
		if msgHash != es.Receipts[i].MsgHash {
			return common.EmptyHash, nil, fmt.Errorf("receipt hash doesn't match its message #%d", i)
		}
	}

	// Update receipts trie
	receiptKeys := make([]types.MessageIndex, 0, len(es.Receipts))
	receiptValues := make([]*types.Receipt, 0, len(es.Receipts))
	msgStart := 0
	for i, r := range es.Receipts {
		msgHash := es.InMessageHashes[i]
		r.OutMsgIndex = uint32(msgStart)
		r.OutMsgNum = uint32(len(es.OutMessages[msgHash]))

		receiptKeys = append(receiptKeys, types.MessageIndex(i))
		receiptValues = append(receiptValues, r)
		msgStart += len(es.OutMessages[msgHash])
	}
	if err := es.ReceiptTree.UpdateBatch(receiptKeys, receiptValues); err != nil {
		return common.EmptyHash, nil, err
	}

	configRoot := common.EmptyHash
	if es.ShardId.IsMainShard() {
		var err error
		prevBlock, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return common.EmptyHash, nil, fmt.Errorf("failed to read previous block: %w", err)
		}
		if prevBlock != nil {
			configRoot = prevBlock.ConfigRoot
		}
		if configRoot, err = es.GetConfigAccessor().UpdateConfigTrie(es.tx, configRoot); err != nil {
			return common.EmptyHash, nil, fmt.Errorf("failed to update config trie: %w", err)
		}
	}

	block := types.Block{
		Id:                  blockId,
		PrevBlock:           es.PrevBlock,
		SmartContractsRoot:  es.ContractTree.RootHash(),
		InMessagesRoot:      es.InMessageTree.RootHash(),
		OutMessagesRoot:     es.OutMessageTree.RootHash(),
		ConfigRoot:          configRoot,
		OutMessagesNum:      types.MessageIndex(len(outMsgKeys)),
		ReceiptsRoot:        es.ReceiptTree.RootHash(),
		ChildBlocksRootHash: treeShardsRootHash,
		MainChainHash:       es.MainChainHash,
		GasPrice:            es.GasPrice,
		LogsBloom:           types.CreateBloom(es.Receipts),
		// TODO(@klonD90): remove this field after changing explorer
		Timestamp: 0,
	}

	if TraceBlocksEnabled {
		blocksTracer.Trace(es, &block)
	}

	for k, v := range es.Errors {
		if err := db.WriteError(es.tx, k, v.Error()); err != nil {
			return common.EmptyHash, nil, err
		}
	}

	blockHash := block.Hash(es.ShardId)
	err := db.WriteBlock(es.tx, es.ShardId, blockHash, &block)
	if err != nil {
		return common.EmptyHash, nil, err
	}

	logger.Trace().
		Stringer(logging.FieldShardId, es.ShardId).
		Stringer(logging.FieldBlockNumber, blockId).
		Stringer(logging.FieldBlockHash, blockHash).
		Msgf("Committed new block with %d in-msgs and %d out-msgs", len(es.InMessages), block.OutMessagesNum)

	return blockHash, outMsgValues, nil
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
				err := fmt.Errorf("not enough credit for ForwardKindValue: %v < %v", availValue, msg.Message.FeeCredit)
				return types.NewZeroValue(), err
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
				return types.NewZeroValue(), fmt.Errorf("invalid percentage value %v", msg.FeeCredit)
			}
			msg.FeeCredit = availValue0.Mul(msg.FeeCredit).Div(types.NewValueFromUint64(100))

			availValue, overflow = availValue.SubOverflow(msg.Message.FeeCredit)
			if overflow {
				return types.NewZeroValue(), errors.New("sum of percentage is more than 100")
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

func (es *ExecutionState) GetMessageFlags() types.MessageFlags {
	return es.GetInMessage().Flags
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

	if err := es.newVm(message.IsInternal(), message.From, nil); err != nil {
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
		msgErr := types.KeepOrWrapError(types.ErrorExternalVerificationFailed, err)
		return NewExecutionResult().SetError(msgErr)
	}
	if !bytes.Equal(ret, common.LeftPadBytes([]byte{1}, 32)) {
		return NewExecutionResult().SetError(types.NewError(types.ErrorExternalVerificationFailed))
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
		Stringer("id", currencyId).
		Msg("Add currency")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetCurrencyBalance(currencyId)
	if balance == nil {
		balance = &types.Value{}
	}
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
		Stringer("id", currencyId).
		Msg("Sub currency")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetCurrencyBalance(currencyId)
	if balance == nil {
		balance = &types.Value{}
	}
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
	for k, v := range acc.CurrencyTrieReader.Iterate() {
		var c types.CurrencyBalance
		c.Currency = types.CurrencyId(k)
		if err := c.Balance.UnmarshalSSZ(v); err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal currency balance")
			continue
		}
		res[c.Currency] = c.Balance
	}
	// If some currency was changed during execution, we need to set it to the result. It will probably rewrite values
	// fetched from the storage above.
	for id, balance := range *acc.Currencies {
		res[id] = balance
	}

	return res
}

func (es *ExecutionState) GetGasPrice(shardId types.ShardId) (types.Value, error) {
	prices, err := es.GetConfigAccessor().GetParamGasPrice()
	if err != nil {
		return types.Value{}, err
	}
	return types.Value{Uint256: &prices.Shards[shardId]}, nil
}

func (es *ExecutionState) SetCurrencyTransfer(currencies []types.CurrencyBalance) {
	es.evm.SetCurrencyTransfer(currencies)
}

func (es *ExecutionState) SaveVmState(state *types.EvmState, continuationGasCredit types.Gas) error {
	outMessages := es.OutMessages[es.InMessageHash]
	check.PanicIfNot(len(outMessages) > 0)

	outMsg := outMessages[len(outMessages)-1]
	check.PanicIfNot(outMsg.RequestId != 0)

	data, err := state.MarshalSSZ()
	if err != nil {
		return err
	}

	acc, err := es.GetAccount(es.GetInMessage().To)
	check.PanicIfErr(err)

	logger.Debug().Int("size", len(data)).Msg("Save vm state")

	context := &types.AsyncContext{IsAwait: true, Data: data, ResponseProcessingGas: continuationGasCredit}
	acc.SetAsyncContext(types.MessageIndex(outMsg.RequestId), context)
	return nil
}

func (es *ExecutionState) newVm(internal bool, origin types.Address, state *vm.EvmRestoreData) error {
	blockContext, err := NewEVMBlockContext(es)
	if err != nil {
		return err
	}
	es.evm = vm.NewEVM(blockContext, es, origin, state)
	es.evm.IsAsyncCall = internal
	return nil
}

func (es *ExecutionState) SetVm(evm *vm.EVM) {
	es.evm = evm
}

func (es *ExecutionState) resetVm() {
	es.evm = nil
}

func (es *ExecutionState) UpdateGasPrice() {
	data, err := es.shardAccessor.GetBlock().ByHash(es.PrevBlock)
	if err != nil {
		// If we can't read the previous block, we don't change the gas price
		es.GasPrice = types.DefaultGasPrice
		logger.Error().Err(err).Msg("failed to read previous block, gas price won't be changed")
		return
	}

	es.GasPrice = data.Block().GasPrice

	decreasePerBlock := types.NewValueFromUint64(1)
	maxGasPrice := types.NewValueFromUint64(100)

	gasIncrease := uint64(math.Ceil(float64(data.Block().OutMessagesNum) * es.gasPriceScale))
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

func (es ExecutionState) MarshalJSON() ([]byte, error) {
	data := struct {
		ContractTreeRoot   common.Hash                              `json:"contractTreeRoot"`
		InMessageTreeRoot  common.Hash                              `json:"inMessageTreeRoot"`
		OutMessageTreeRoot common.Hash                              `json:"outMessageTreeRoot"`
		ReceiptTreeRoot    common.Hash                              `json:"receiptTreeRoot"`
		PrevBlock          common.Hash                              `json:"prevBlock"`
		MainChainHash      common.Hash                              `json:"mainChainHash"`
		ShardId            types.ShardId                            `json:"shardId"`
		ChildChainBlocks   map[types.ShardId]common.Hash            `json:"childChainBlocks"`
		GasPrice           types.Value                              `json:"gasPrice"`
		InMessages         []*types.Message                         `json:"inMessages"`
		InMessageHashes    []common.Hash                            `json:"inMessageHashes"`
		OutMessages        map[common.Hash][]*types.OutboundMessage `json:"outMessages"`
		Receipts           []*types.Receipt                         `json:"receipts"`
		Errors             map[common.Hash]error                    `json:"errors"`
		GasPriceScale      float64                                  `json:"gasPriceScale"`
	}{
		ContractTreeRoot:   es.ContractTree.RootHash(),
		InMessageTreeRoot:  es.InMessageTree.RootHash(),
		OutMessageTreeRoot: es.OutMessageTree.RootHash(),
		ReceiptTreeRoot:    es.ReceiptTree.RootHash(),
		PrevBlock:          es.PrevBlock,
		MainChainHash:      es.MainChainHash,
		ShardId:            es.ShardId,
		ChildChainBlocks:   es.ChildChainBlocks,
		GasPrice:           es.GasPrice,
		InMessages:         es.InMessages,
		InMessageHashes:    es.InMessageHashes,
		OutMessages:        es.OutMessages,
		Receipts:           es.Receipts,
		Errors:             es.Errors,
		GasPriceScale:      es.gasPriceScale,
	}

	return json.Marshal(data)
}
