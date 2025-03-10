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

const ExternalTransactionVerificationMaxGas = types.Gas(100_000)

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

type RollbackParams struct {
	Version     uint32
	Counter     uint32
	PatchLevel  uint32
	MainBlockId uint64
	ReplayDepth uint32
	SearchDepth uint32
}

type ExecutionState struct {
	tx                 db.RwTx
	ContractTree       *ContractTrie
	InTransactionTree  *TransactionTrie
	OutTransactionTree *TransactionTrie
	ReceiptTree        *ReceiptTrie
	PrevBlock          common.Hash
	MainChainHash      common.Hash
	ShardId            types.ShardId
	ChildChainBlocks   map[types.ShardId]common.Hash
	GasPrice           types.Value // Current gas price including priority fee
	BaseFee            types.Value

	// Those fields are just copied from the proposal into the block
	// and are not used in the state
	PatchLevel      uint32
	RollbackCounter uint32

	InTransactionHash common.Hash
	Logs              map[common.Hash][]*types.Log
	DebugLogs         map[common.Hash][]*types.DebugLog

	Accounts            map[types.Address]*AccountState
	InTransactions      []*types.Transaction
	InTransactionHashes []common.Hash

	// OutTransactions holds outbound transactions for every transaction in the executed block, where key is hash of Transaction that sends the transaction
	OutTransactions map[common.Hash][]*types.OutboundTransaction

	Receipts []*types.Receipt
	Errors   map[common.Hash]error

	GasUsed types.Gas

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

	// Pointer to currently executed VM
	evm *vm.EVM

	// wasAwaitCall is true if the VM execution ended with sending a awaitCall transaction
	wasAwaitCall bool

	configAccessor config.ConfigAccessor

	// txnFeeCredit holds the total fee credit for the inbound transaction. It can be changed during execution, thus we
	// use this separate variable instead of the one in the transaction.
	txnFeeCredit types.Value

	// isReadOnly is true if the state is in read-only mode. This mode is used for eth_call and eth_estimateGas.
	isReadOnly bool

	FeeCalculator FeeCalculator

	// filled in if a rollback was requested by a transaction
	rollback *RollbackParams
}

type ExecutionResult struct {
	ReturnData     []byte
	Error          types.ExecError
	FatalError     error
	GasUsed        types.Gas
	GasPrice       types.Value
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

func (e *ExecutionResult) SetTxnErrorOrFatal(err error) *ExecutionResult {
	if txnErr := (types.ExecError)(nil); errors.As(err, &txnErr) {
		e.SetError(txnErr)
	} else {
		e.SetFatal(err)
	}
	return e
}

func (e *ExecutionResult) SetUsed(gas types.Gas, gasPrice types.Value) *ExecutionResult {
	e.GasUsed = gas
	e.GasPrice = gasPrice
	return e
}

func (e *ExecutionResult) AddUsed(gas types.Gas) *ExecutionResult {
	e.GasUsed += gas
	return e
}

func (e *ExecutionResult) CoinsUsed() types.Value {
	return e.GasUsed.ToValue(e.GasPrice)
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
	return value.Sub(e.CoinsUsed()).Sub(e.CoinsForwarded)
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

	currentBlockId := uint64(0)
	var header *types.Block
	time := uint64(0)
	rollbackCounter := uint32(0)
	if err == nil {
		header = data.Block()
		currentBlockId = header.Id.Uint64() + 1
		// TODO: we need to use header.Timestamp instead of but it's always zero for now.
		// Let's return some kind of logical timestamp (monotonic increasing block number).
		time = header.Id.Uint64()
		rollbackCounter = header.RollbackCounter
	}
	return &vm.BlockContext{
		GetHash:     getHashFn(es, header),
		BlockNumber: currentBlockId,
		Random:      &common.EmptyHash,
		BaseFee:     big.NewInt(10),
		BlobBaseFee: big.NewInt(10),
		Time:        time,

		RollbackCounter: rollbackCounter,
	}, nil
}

type StateParams struct {
	Block          *types.Block
	ConfigAccessor config.ConfigAccessor
	FeeCalculator  FeeCalculator
}

func NewExecutionState(tx any, shardId types.ShardId, params StateParams) (*ExecutionState, error) {
	var resTx db.RwTx
	isReadOnly := false
	if rwTx, ok := tx.(db.RwTx); ok {
		resTx = rwTx
	} else if roTx, ok := tx.(db.RoTx); ok {
		isReadOnly = true
		resTx = &db.RwWrapper{RoTx: roTx}
	} else {
		return nil, errors.New("invalid tx type")
	}

	feeCalculator := params.FeeCalculator
	if feeCalculator == nil {
		feeCalculator = &MainFeeCalculator{}
	}

	var baseFeePerGas types.Value
	var prevBlockHash common.Hash
	if params.Block != nil {
		baseFeePerGas = feeCalculator.CalculateBaseFee(params.Block)
		prevBlockHash = params.Block.Hash(shardId)
	}

	res := &ExecutionState{
		tx:               resTx,
		PrevBlock:        prevBlockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[types.ShardId]common.Hash{},
		Accounts:         map[types.Address]*AccountState{},
		OutTransactions:  map[common.Hash][]*types.OutboundTransaction{},
		Logs:             map[common.Hash][]*types.Log{},
		DebugLogs:        map[common.Hash][]*types.DebugLog{},
		Errors:           map[common.Hash]error{},

		journal:          newJournal(),
		transientStorage: newTransientStorage(),

		shardAccessor:  NewStateAccessor().Access(resTx, shardId),
		configAccessor: params.ConfigAccessor,

		BaseFee:  baseFeePerGas,
		GasPrice: types.NewZeroValue(),

		isReadOnly: isReadOnly,

		FeeCalculator: feeCalculator,
	}

	return res, res.initTries()
}

func (es *ExecutionState) initTries() error {
	data, err := es.shardAccessor.GetBlock().ByHash(es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	es.ContractTree = NewDbContractTrie(es.tx, es.ShardId)
	es.InTransactionTree = NewDbTransactionTrie(es.tx, es.ShardId)
	es.OutTransactionTree = NewDbTransactionTrie(es.tx, es.ShardId)
	es.ReceiptTree = NewDbReceiptTrie(es.tx, es.ShardId)
	if err == nil {
		es.ContractTree.SetRootHash(data.Block().SmartContractsRoot)
	}

	return nil
}

func (es *ExecutionState) GetConfigAccessor() config.ConfigAccessor {
	return es.configAccessor
}

func (es *ExecutionState) GetReceipt(txnIndex types.TransactionIndex) (*types.Receipt, error) {
	return es.ReceiptTree.Fetch(txnIndex)
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
	es.journal.append(addLogChange{txhash: es.InTransactionHash})
	if len(es.Logs[es.InTransactionHash]) >= types.ReceiptMaxLogsSize {
		return errors.New("too many logs")
	}
	es.Logs[es.InTransactionHash] = append(es.Logs[es.InTransactionHash], log)
	return nil
}

func (es *ExecutionState) AddDebugLog(log *types.DebugLog) error {
	if len(es.DebugLogs[es.InTransactionHash]) >= types.ReceiptMaxDebugLogsSize {
		return errors.New("too many debug logs")
	}
	es.DebugLogs[es.InTransactionHash] = append(es.DebugLogs[es.InTransactionHash], log)
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
		return nil, common.EmptyHash, err
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
	es.journal.revert(&ExecutionStateRevertableWrapper{es}, snapshot)
	es.validRevisions = es.validRevisions[:idx]
}

func (es *ExecutionState) GetStorageRoot(addr types.Address) (common.Hash, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return common.EmptyHash, err
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
	if stateObject.NewContract {
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

func (es *ExecutionState) SetInitState(addr types.Address, transaction *types.Transaction) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.Seqno = transaction.Seqno

	if err := es.newVm(transaction.IsInternal(), transaction.From, nil); err != nil {
		return err
	}
	defer es.resetVm()

	_, deployAddr, _, err := es.evm.Deploy(addr, vm.AccountRef{}, transaction.Data, uint64(100_000_000) /* gas */, uint256.NewInt(0))
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
// This operation sets the 'NewContract'-flag, which is required in order to
// correctly handle EIP-6780 'delete-in-same-transaction' logic.
func (es *ExecutionState) CreateContract(addr types.Address) error {
	obj, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if !obj.NewContract {
		obj.NewContract = true
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

func (es *ExecutionState) AddInTransactionWithHash(transaction *types.Transaction, hash common.Hash) {
	// Refund transactions can be identical (see comment to AddOutTransaction).
	// Otherwise, adding the same transaction twice is an error in the code.
	check.PanicIfNot(hash != es.InTransactionHash || transaction.IsRefund())

	// We store a copy of the transaction, because the original transaction will be modified.
	es.InTransactions = append(es.InTransactions, common.CopyPtr(transaction))
	es.InTransactionHash = hash
	es.InTransactionHashes = append(es.InTransactionHashes, hash)
}

func (es *ExecutionState) AddInTransaction(transaction *types.Transaction) common.Hash {
	hash := transaction.Hash()
	es.AddInTransactionWithHash(transaction, hash)
	return hash
}

func (es *ExecutionState) DropInTransaction() {
	check.PanicIfNot(len(es.InTransactions) == len(es.InTransactionHashes))

	es.InTransactions = es.InTransactions[:len(es.InTransactions)-1]
	es.InTransactionHashes = es.InTransactionHashes[:len(es.InTransactionHashes)-1]

	if len(es.InTransactionHashes) > 0 {
		es.InTransactionHash = es.InTransactionHashes[len(es.InTransactions)-1]
	} else {
		es.InTransactionHash = common.EmptyHash
	}
}

func (es *ExecutionState) updateGasPrice(txn *types.Transaction) error {
	es.GasPrice = es.BaseFee.Add(txn.MaxPriorityFeePerGas)

	// For read-only execution, there is no need to validate MaxFeePerGas.
	if !es.isReadOnly && es.GasPrice.Cmp(txn.MaxFeePerGas) > 0 {
		if es.BaseFee.Cmp(txn.MaxFeePerGas) > 0 {
			es.GasPrice = types.Value0
			logger.Error().
				Stringer("MaxFeePerGas", txn.MaxFeePerGas).
				Stringer("BaseFee", es.BaseFee).
				Msg("MaxFeePerGas is less than BaseFee")
			return types.NewError(types.ErrorBaseFeeTooHigh)
		}
		es.GasPrice = txn.MaxFeePerGas
	}
	return nil
}

func (es *ExecutionState) AppendForwardTransaction(txn *types.Transaction) {
	// setting all forward txns to the same empty hash preserves ordering
	parentHash := common.EmptyHash
	outTxn := &types.OutboundTransaction{Transaction: txn, TxnHash: txn.Hash(), ForwardKind: types.ForwardKindNone}
	es.OutTransactions[parentHash] = append(es.OutTransactions[parentHash], outTxn)
}

func (es *ExecutionState) AddOutRequestTransaction(
	caller types.Address,
	payload *types.InternalTransactionPayload,
	responseProcessingGas types.Gas,
	isAwait bool,
) (*types.Transaction, error) {
	txn, err := es.AddOutTransaction(caller, payload)
	if err != nil {
		return nil, err
	}

	acc, err := es.GetAccount(caller)
	check.PanicIfErr(err)

	txn.RequestId = acc.FetchRequestId()

	// Only await calls should inherit the request chain from the inbound transaction.
	if isAwait {
		// If an inbound transaction is also a request, we need to add a new record to the request chain.
		inTxn := es.GetInTransaction()
		if inTxn.IsRequest() {
			txn.RequestChain = make([]*types.AsyncRequestInfo, len(es.GetInTransaction().RequestChain)+1)
			copy(txn.RequestChain, inTxn.RequestChain)
			txn.RequestChain[len(inTxn.RequestChain)] = &types.AsyncRequestInfo{
				Id:     inTxn.RequestId,
				Caller: inTxn.From,
			}
		} else if len(inTxn.RequestChain) != 0 {
			// If inbound transaction is a response, we need to copy the request chain from it.
			check.PanicIfNot(inTxn.IsResponse())
			txn.RequestChain = inTxn.RequestChain
		}

		es.wasAwaitCall = true
		// Stop vm execution and save its state after the current instruction (call of precompile) is finished.
		es.evm.StopAndDumpState(responseProcessingGas)
	} else {
		acc.SetAsyncContext(types.TransactionIndex(txn.RequestId), &types.AsyncContext{
			IsAwait:               false,
			Data:                  payload.RequestContext,
			ResponseProcessingGas: responseProcessingGas,
		})
	}

	return txn, nil
}

func (es *ExecutionState) AddOutTransaction(caller types.Address, payload *types.InternalTransactionPayload) (*types.Transaction, error) {
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

	txn := payload.ToTransaction(caller, seqno)

	// Propagate fee settings from the inbound message
	txn.MaxPriorityFeePerGas = es.GetInTransaction().MaxPriorityFeePerGas
	txn.MaxFeePerGas = es.GetInTransaction().MaxFeePerGas

	txnHash := txn.Hash()

	// In case of bounce transaction, we don't debit token from account
	// In case of refund transaction, we don't transfer tokens
	if !txn.IsBounce() && !txn.IsRefund() {
		acc, err := es.GetAccount(txn.From)
		if err != nil {
			return nil, err
		}
		for _, token := range txn.Token {
			balance := acc.GetTokenBalance(token.Token)
			if balance == nil {
				balance = &types.Value{}
			}
			if balance.Cmp(token.Balance) < 0 {
				return nil, fmt.Errorf("%w: %s < %s, token %s",
					vm.ErrInsufficientBalance, balance, token.Balance, token.Token)
			}
			if err := es.SubToken(txn.From, token.Token, token.Balance); err != nil {
				return nil, err
			}
		}
	}

	logger.Trace().
		Stringer(logging.FieldTransactionHash, txnHash).
		Stringer(logging.FieldTransactionFrom, txn.From).
		Stringer(logging.FieldTransactionTo, txn.To).
		Msg("Outbound transaction added")

	es.journal.append(outTransactionsChange{
		txnHash: es.InTransactionHash,
		index:   len(es.OutTransactions[es.InTransactionHash]),
	})

	outTxn := &types.OutboundTransaction{Transaction: txn, TxnHash: txnHash, ForwardKind: payload.ForwardKind}
	es.OutTransactions[es.InTransactionHash] = append(es.OutTransactions[es.InTransactionHash], outTxn)

	return txn, nil
}

func (es *ExecutionState) sendBounceTransaction(txn *types.Transaction, execResult *ExecutionResult) (bool, error) {
	if txn.Value.IsZero() && len(txn.Token) == 0 {
		return false, nil
	}
	if txn.BounceTo == types.EmptyAddress {
		logger.Debug().Msg("Bounce transaction not sent, no bounce address")
		return false, nil
	}

	data, err := contracts.NewCallData(contracts.NameNilBounceable, "bounce", execResult.Error.Error())
	if err != nil {
		return false, err
	}

	check.PanicIfNotf(execResult.CoinsForwarded.IsZero(), "CoinsForwarded should be zero when sending bounce transaction")
	toReturn := es.txnFeeCredit.Sub(execResult.CoinsUsed())

	bounceTxn := &types.InternalTransactionPayload{
		Bounce:    true,
		To:        txn.BounceTo,
		RefundTo:  txn.RefundTo,
		Value:     txn.Value,
		Token:     txn.Token,
		Data:      data,
		FeeCredit: toReturn,
	}
	if _, err = es.AddOutTransaction(txn.To, bounceTxn); err != nil {
		return false, err
	}
	logger.Debug().Stringer(logging.FieldTransactionFrom, txn.To).Stringer(logging.FieldTransactionTo, txn.BounceTo).Msg("Bounce transaction sent")
	return true, nil
}

func (es *ExecutionState) SendResponseTransaction(txn *types.Transaction, res *ExecutionResult) error {
	asyncResponsePayload := types.AsyncResponsePayload{
		Success:    !res.Failed(),
		ReturnData: res.ReturnData,
	}
	data, err := asyncResponsePayload.MarshalSSZ()
	if err != nil {
		return err
	}

	responsePayload := &types.InternalTransactionPayload{
		Kind:        types.ResponseTransactionKind,
		ForwardKind: types.ForwardKindRemaining,
		Data:        data,
	}

	// TODO: need to pay for response here
	// we pay for mem during VM execution, so likely big response isn't a problem
	responseTxn, err := es.AddOutTransaction(txn.To, responsePayload)
	if err != nil {
		return err
	}

	// Send back value in case of failed transaction, thereby we don't need in a separate bounce transaction.
	if res.Failed() {
		responseTxn.Value = txn.Value
	}

	if txn.IsRequest() {
		responseTxn.To = txn.From
		responseTxn.RequestId = txn.RequestId
		responseTxn.RequestChain = txn.RequestChain
	} else {
		// We are processing a response transaction with requests chain. So get pending request from the chain and send
		// response to it.
		check.PanicIfNotf(txn.IsResponse(), "Transaction should be a response")
		responseTxn.To = txn.RequestChain[len(txn.RequestChain)-1].Caller
		responseTxn.RequestId = txn.RequestChain[len(txn.RequestChain)-1].Id
		responseTxn.RequestChain = txn.RequestChain[:len(txn.RequestChain)-1]
	}
	return nil
}

func (es *ExecutionState) HandleTransaction(ctx context.Context, txn *types.Transaction, payer Payer) (retError *ExecutionResult) {
	// Catch panic during execution and return it as an error
	defer func() {
		if recResult := recover(); recResult != nil {
			if err, ok := recResult.(error); ok {
				retError = NewExecutionResult().SetError(types.NewWrapError(types.ErrorPanicDuringExecution, err))
			} else {
				retError = NewExecutionResult().SetError(
					types.NewVerboseError(types.ErrorPanicDuringExecution, fmt.Sprintf("panic transaction: %v", recResult)))
			}
		}
	}()

	es.txnFeeCredit = txn.FeeCredit

	if err := es.updateGasPrice(txn); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorBaseFeeTooHigh, err))
	}

	if es.GasPrice.IsZero() {
		return NewExecutionResult().SetError(types.NewError(types.ErrorMaxFeePerGasIsZero))
	}

	if err := buyGas(payer, txn); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorBuyGas, err))
	}
	if err := txn.VerifyFlags(); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorValidation, err))
	}

	var res *ExecutionResult
	switch {
	case txn.IsRefund():
		return NewExecutionResult().SetFatal(es.handleRefundTransaction(ctx, txn))
	case txn.IsDeploy():
		res = es.handleDeployTransaction(ctx, txn)
	default:
		res = es.handleExecutionTransaction(ctx, txn)
	}
	responseWasSent := false
	bounced := false
	if txn.IsRequest() {
		if !es.wasAwaitCall {
			if err := es.SendResponseTransaction(txn, res); err != nil {
				return NewExecutionResult().SetFatal(fmt.Errorf("SendResponseTransaction failed: %w\n", err))
			}
			bounced = true
			responseWasSent = true
		}
	} else if txn.IsResponse() && !es.wasAwaitCall && len(txn.RequestChain) > 0 {
		// There is pending requests in the chain, so we need to send response to them.
		// But we don't send response if a new request was sent during the execution.
		if err := es.SendResponseTransaction(txn, res); err != nil {
			return NewExecutionResult().SetFatal(fmt.Errorf("SendResponseTransaction failed: %w\n", err))
		}
		responseWasSent = true
	}
	// We don't need bounce transaction for request, because it will be sent within the response transaction.
	if res.Error != nil && !responseWasSent {
		revString := decodeRevertTransaction(res.ReturnData)
		if revString != "" {
			if types.IsVmError(res.Error) {
				res.Error = types.NewVmVerboseError(res.Error.Code(), revString)
			} else {
				res.Error = types.NewVerboseError(res.Error.Code(), revString)
			}
		}
		if txn.IsBounce() {
			logger.Error().Err(res.Error).Msg("VM returns error during bounce transaction processing")
		} else {
			logger.Debug().Err(res.Error).Msg("execution txn failed")
			if txn.IsInternal() {
				var bounceErr error
				if bounced, bounceErr = es.sendBounceTransaction(txn, res); bounceErr != nil {
					logger.Error().Err(bounceErr).Msg("Bounce transaction sent failed")
					return res.SetFatal(bounceErr)
				}
			}
		}
	} else {
		availableGas := es.txnFeeCredit.Sub(res.CoinsUsed())
		var err error
		if res.CoinsForwarded, err = es.CalculateGasForwarding(availableGas); err != nil {
			es.RevertToSnapshot(es.revertId)
			res.Error = types.KeepOrWrapError(types.ErrorForwardingFailed, err)
		}
	}
	// Gas is already refunded with the bounce transaction
	if !bounced {
		leftOverCredit := res.GetLeftOverValue(es.txnFeeCredit)
		if txn.RefundTo == txn.To {
			acc, err := es.GetAccount(txn.To)
			check.PanicIfErr(err)
			check.PanicIfErr(acc.AddBalance(leftOverCredit, tracing.BalanceIncreaseRefund))
		} else {
			if err := refundGas(payer, leftOverCredit); err != nil {
				res.Error = types.KeepOrWrapError(types.ErrorGasRefundFailed, err)
			}
		}
	}

	es.GasUsed += res.GasUsed

	return res
}

func (es *ExecutionState) handleDeployTransaction(_ context.Context, transaction *types.Transaction) *ExecutionResult {
	addr := transaction.To
	deployTxn := types.ParseDeployPayload(transaction.Data)

	logger.Debug().
		Stringer(logging.FieldTransactionTo, addr).
		Stringer(logging.FieldShardId, es.ShardId).
		Msg("Handling deploy transaction...")

	if err := es.newVm(transaction.IsInternal(), transaction.From, nil); err != nil {
		return NewExecutionResult().SetFatal(err)
	}
	defer es.resetVm()

	gas := transaction.FeeCredit.ToGas(es.GasPrice)
	ret, addr, leftOver, err := es.evm.Deploy(addr, (vm.AccountRef)(transaction.From), deployTxn.Code(), gas.Uint64(), transaction.Value.Int())

	event := logger.Debug().Stringer(logging.FieldTransactionTo, addr)
	if err != nil {
		event.Err(err).Msg("Contract deployment failed.")
	} else {
		event.Msg("Created new contract.")
	}

	return NewExecutionResult().
		SetTxnErrorOrFatal(err).
		SetUsed(gas-types.Gas(leftOver), es.GasPrice).
		SetReturnData(ret).SetDebugInfo(es.evm.DebugInfo)
}

func (es *ExecutionState) TryProcessResponse(transaction *types.Transaction) ([]byte, *vm.EvmRestoreData, *ExecutionResult) {
	if !transaction.IsResponse() {
		return transaction.Data, nil, nil
	}
	var restoreState *vm.EvmRestoreData
	var callData []byte

	check.PanicIfNot(transaction.RequestId != 0)
	acc, err := es.GetAccount(transaction.To)
	if err != nil {
		return nil, nil, NewExecutionResult().SetFatal(err)
	}
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, transaction.RequestId)
	asyncContext, err := acc.GetAndRemoveAsyncContext(types.TransactionIndex(transaction.RequestId))
	if err != nil {
		return nil, nil, NewExecutionResult().SetFatal(fmt.Errorf("failed to get async context: %w", err))
	}

	responsePayload := new(types.AsyncResponsePayload)
	if err := responsePayload.UnmarshalSSZ(transaction.Data); err != nil {
		return nil, nil, NewExecutionResult().SetFatal(fmt.Errorf("AsyncResponsePayload unmarshal failed: %w", err))
	}

	es.txnFeeCredit = es.txnFeeCredit.Add(asyncContext.ResponseProcessingGas.ToValue(es.GasPrice))

	if asyncContext.IsAwait {
		// Restore VM state from the context
		restoreState = new(vm.EvmRestoreData)
		if err = restoreState.EvmState.UnmarshalSSZ(asyncContext.Data); err != nil {
			return nil, nil, NewExecutionResult().SetFatal(fmt.Errorf("context unmarshal failed: %w", err))
		}

		restoreState.ReturnData = responsePayload.ReturnData
		restoreState.Result = responsePayload.Success
	} else {
		if len(asyncContext.Data) < 4 {
			return nil, nil, NewExecutionResult().SetError(types.NewError(types.ErrorAwaitCallTooShortContextData))
		}
		contextData := asyncContext.Data[4:]
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
		callData = append(asyncContext.Data[:4], callData...)
	}

	return callData, restoreState, nil
}

func (es *ExecutionState) handleExecutionTransaction(_ context.Context, transaction *types.Transaction) *ExecutionResult {
	check.PanicIfNot(transaction.IsExecution())
	addr := transaction.To
	logger.Debug().
		Stringer(logging.FieldTransactionTo, addr).
		Stringer(logging.FieldTransactionFlags, transaction.Flags).
		Msg("Handling execution transaction...")

	caller := (vm.AccountRef)(transaction.From)

	callData, restoreState, res := es.TryProcessResponse(transaction)
	if res != nil && res.Failed() {
		return res
	}

	if err := es.newVm(transaction.IsInternal(), transaction.From, restoreState); err != nil {
		return NewExecutionResult().SetFatal(err)
	}
	defer es.resetVm()

	if es.TraceVm {
		es.EnableVmTracing()
	}

	if transaction.IsExternal() {
		seqno, err := es.GetExtSeqno(addr)
		if err != nil {
			return NewExecutionResult().SetFatal(err)
		}
		if err := es.SetExtSeqno(addr, seqno+1); err != nil {
			return NewExecutionResult().SetFatal(err)
		}
	}

	es.revertId = es.Snapshot()

	es.evm.SetTokenTransfer(transaction.Token)
	gas := es.txnFeeCredit.ToGas(es.GasPrice)
	ret, leftOver, err := es.evm.Call(caller, addr, callData, gas.Uint64(), transaction.Value.Int())

	return NewExecutionResult().
		SetTxnErrorOrFatal(err).
		SetUsed(gas-types.Gas(leftOver), es.GasPrice).
		SetReturnData(ret).SetDebugInfo(es.evm.DebugInfo)
}

// decodeRevertTransaction decodes the revert transaction from the EVM revert data
func decodeRevertTransaction(data []byte) string {
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

func (es *ExecutionState) handleRefundTransaction(_ context.Context, transaction *types.Transaction) error {
	err := es.AddBalance(transaction.To, transaction.Value, tracing.BalanceIncreaseRefund)
	logger.Debug().Err(err).Msgf("Refunded %s to %v", transaction.Value, transaction.To)
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
		GasUsed:         execResult.GasUsed,
		Forwarded:       execResult.CoinsForwarded,
		TxnHash:         es.InTransactionHash,
		Logs:            es.Logs[es.InTransactionHash],
		DebugLogs:       es.DebugLogs[es.InTransactionHash],
		ContractAddress: es.GetInTransaction().To,
	}

	if execResult.Failed() {
		es.Errors[es.InTransactionHash] = execResult.Error
		if execResult.DebugInfo != nil {
			check.PanicIfNot(execResult.DebugInfo.Pc <= math.MaxUint32)
			r.FailedPc = uint32(execResult.DebugInfo.Pc)
		}
	}
	es.Receipts = append(es.Receipts, r)
}

func GetOutTransactions(es *ExecutionState) ([]*types.Transaction, []common.Hash) {
	txns := make([]*types.Transaction, 0, len(es.OutTransactions[common.EmptyHash]))
	hashes := make([]common.Hash, 0, len(es.OutTransactions[common.EmptyHash]))

	// First, forwarded txns
	for _, m := range es.OutTransactions[common.EmptyHash] {
		txns = append(txns, m.Transaction)
		hashes = append(hashes, m.TxnHash)
	}

	// Then, outgoing txns in the order of their parent txns
	for _, h := range es.InTransactionHashes {
		for _, m := range es.OutTransactions[h] {
			txns = append(txns, m.Transaction)
			hashes = append(hashes, m.TxnHash)
		}
	}

	return txns, hashes
}

func (es *ExecutionState) BuildBlock(blockId types.BlockNumber) (*BlockGenerationResult, error) {
	keys := make([]common.Hash, 0, len(es.Accounts))
	values := make([]*types.SmartContract, 0, len(es.Accounts))
	for k, acc := range es.Accounts {
		v, err := acc.Commit()
		if err != nil {
			return nil, err
		}

		keys = append(keys, k.Hash())
		values = append(values, v)
	}
	if err := es.ContractTree.UpdateBatch(keys, values); err != nil {
		return nil, err
	}

	treeShardsRootHash := common.EmptyHash
	if len(es.ChildChainBlocks) > 0 {
		treeShards := NewDbShardBlocksTrie(es.tx, es.ShardId, blockId)
		if err := UpdateFromMap(treeShards, es.ChildChainBlocks, func(v common.Hash) *common.Hash { return &v }); err != nil {
			return nil, err
		}
		treeShardsRootHash = treeShards.RootHash()
	}

	inTxnKeys := make([]types.TransactionIndex, 0, len(es.InTransactions))
	inTxnValues := make([]*types.Transaction, 0, len(es.InTransactions))
	for i, txn := range es.InTransactions {
		inTxnKeys = append(inTxnKeys, types.TransactionIndex(i))
		inTxnValues = append(inTxnValues, txn)
	}

	outTxnValues, outTxnHashes := GetOutTransactions(es)
	outTxnKeys := make([]types.TransactionIndex, 0, len(es.InTransactions))
	for i := range outTxnValues {
		outTxnKeys = append(outTxnKeys, types.TransactionIndex(i))
	}

	if err := es.InTransactionTree.UpdateBatch(inTxnKeys, inTxnValues); err != nil {
		return nil, err
	}
	if err := es.OutTransactionTree.UpdateBatch(outTxnKeys, outTxnValues); err != nil {
		return nil, err
	}

	if assert.Enable {
		// Check that each outbound transaction belongs to some inbound transaction
		for outTxnHash := range es.OutTransactions {
			if outTxnHash == common.EmptyHash {
				// Skip transactions transmitted over the topology
				continue
			}
			found := false
			for _, txnHash := range es.InTransactionHashes {
				if txnHash == outTxnHash {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("outbound transaction %v does not belong to any inbound transaction", outTxnHash)
			}
		}
	}
	if len(es.InTransactions) != len(es.Receipts) {
		return nil, fmt.Errorf("number of transactions does not match number of receipts: %d != %d", len(es.InTransactions), len(es.Receipts))
	}
	for i, txnHash := range es.InTransactionHashes {
		if txnHash != es.Receipts[i].TxnHash {
			return nil, fmt.Errorf("receipt hash doesn't match its transaction #%d", i)
		}
	}

	// Update receipts trie
	receiptKeys := make([]types.TransactionIndex, 0, len(es.Receipts))
	receiptValues := make([]*types.Receipt, 0, len(es.Receipts))
	txnStart := 0
	for i, r := range es.Receipts {
		txnHash := es.InTransactionHashes[i]
		r.OutTxnIndex = uint32(txnStart)
		r.OutTxnNum = uint32(len(es.OutTransactions[txnHash]))

		receiptKeys = append(receiptKeys, types.TransactionIndex(i))
		receiptValues = append(receiptValues, r)
		txnStart += len(es.OutTransactions[txnHash])
	}
	if err := es.ReceiptTree.UpdateBatch(receiptKeys, receiptValues); err != nil {
		return nil, err
	}

	l1BlockNumber := uint64(0)
	if es.ShardId.IsMainShard() {
		if l1Block, err := config.GetParamL1Block(es.configAccessor); err == nil {
			l1BlockNumber = l1Block.Number
		}
	}

	configRoot := common.EmptyHash
	var configParams map[string][]byte
	if es.ShardId.IsMainShard() {
		var err error
		prevBlock, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, fmt.Errorf("failed to read previous block: %w", err)
		}
		if prevBlock != nil {
			configRoot = prevBlock.ConfigRoot
		}
		if configRoot, err = es.GetConfigAccessor().Commit(es.tx, configRoot); err != nil {
			return nil, fmt.Errorf("failed to update config trie: %w", err)
		}
		if configParams, err = es.GetConfigAccessor().GetParams(); err != nil {
			return nil, fmt.Errorf("failed to read config params: %w", err)
		}
	}

	block := &types.Block{
		BlockData: types.BlockData{
			Id:                  blockId,
			PrevBlock:           es.PrevBlock,
			SmartContractsRoot:  es.ContractTree.RootHash(),
			InTransactionsRoot:  es.InTransactionTree.RootHash(),
			OutTransactionsRoot: es.OutTransactionTree.RootHash(),
			ConfigRoot:          configRoot,
			OutTransactionsNum:  types.TransactionIndex(len(outTxnKeys)),
			ReceiptsRoot:        es.ReceiptTree.RootHash(),
			ChildBlocksRootHash: treeShardsRootHash,
			MainChainHash:       es.MainChainHash,
			BaseFee:             es.BaseFee,
			GasUsed:             es.GasUsed,
			L1BlockNumber:       l1BlockNumber,
			PatchLevel:          es.PatchLevel,
			RollbackCounter:     es.RollbackCounter,
			// TODO(@klonD90): remove this field after changing explorer
			Timestamp: 0,
		},
		LogsBloom: types.CreateBloom(es.Receipts),
	}

	return &BlockGenerationResult{
		Block:        block,
		BlockHash:    block.Hash(es.ShardId),
		InTxns:       es.InTransactions,
		InTxnHashes:  es.InTransactionHashes,
		OutTxns:      outTxnValues,
		OutTxnHashes: outTxnHashes,
		ConfigParams: configParams,
	}, nil
}

func (es *ExecutionState) Commit(blockId types.BlockNumber, params *types.ConsensusParams) (*BlockGenerationResult, error) {
	blockRes, err := es.BuildBlock(blockId)
	if err != nil {
		return nil, err
	}
	return blockRes, es.CommitBlock(blockRes, params)
}

func (es *ExecutionState) CommitBlock(src *BlockGenerationResult, params *types.ConsensusParams) error {
	block := src.Block
	blockHash := src.BlockHash
	if params != nil {
		block.ConsensusParams = *params
	}

	if TraceBlocksEnabled {
		blocksTracer.Trace(es, block, blockHash)
	}

	for k, v := range es.Errors {
		if err := db.WriteError(es.tx, k, v.Error()); err != nil {
			return err
		}
	}

	if err := db.WriteBlock(es.tx, es.ShardId, blockHash, block); err != nil {
		return err
	}

	logger.Trace().
		Stringer(logging.FieldShardId, es.ShardId).
		Stringer(logging.FieldBlockNumber, block.Id).
		Stringer(logging.FieldBlockHash, blockHash).
		Msgf("Committed new block with %d in-txns and %d out-txns", len(es.InTransactions), block.OutTransactionsNum)

	return nil
}

func (es *ExecutionState) CalculateGasForwarding(initialAvailValue types.Value) (types.Value, error) {
	if len(es.OutTransactions) == 0 {
		return types.NewZeroValue(), nil
	}
	var overflow bool

	availValue := initialAvailValue

	remainingFwdTransactions := make([]*types.OutboundTransaction, 0, len(es.OutTransactions[es.InTransactionHash]))
	percentageFwdTransactions := make([]*types.OutboundTransaction, 0, len(es.OutTransactions[es.InTransactionHash]))

	for _, txn := range es.OutTransactions[es.InTransactionHash] {
		switch txn.ForwardKind {
		case types.ForwardKindValue:
			diff, overflow := availValue.SubOverflow(txn.FeeCredit)
			if overflow {
				err := fmt.Errorf("not enough credit for ForwardKindValue: %v < %v", availValue, txn.FeeCredit)
				return types.NewZeroValue(), err
			}
			availValue = diff
		case types.ForwardKindPercentage:
			percentageFwdTransactions = append(percentageFwdTransactions, txn)
		case types.ForwardKindRemaining:
			remainingFwdTransactions = append(remainingFwdTransactions, txn)
		case types.ForwardKindNone:
			// Do nothing for non-forwarding transaction and do not set refundTo
			continue
		}
		if txn.RefundTo.IsEmpty() {
			txn.RefundTo = es.GetInTransaction().RefundTo
		}
	}

	if len(percentageFwdTransactions) != 0 {
		availValue0 := availValue
		for _, txn := range percentageFwdTransactions {
			if !txn.FeeCredit.IsUint64() || txn.FeeCredit.Uint64() > 100 {
				return types.NewZeroValue(), fmt.Errorf("invalid percentage value %v", txn.FeeCredit)
			}
			txn.FeeCredit = availValue0.Mul(txn.FeeCredit).Div(types.NewValueFromUint64(100))

			availValue, overflow = availValue.SubOverflow(txn.FeeCredit)
			if overflow {
				return types.NewZeroValue(), errors.New("sum of percentage is more than 100")
			}
		}
	}

	if len(remainingFwdTransactions) != 0 {
		availValue0 := availValue
		portion := availValue0.Div(types.NewValueFromUint64(uint64(len(remainingFwdTransactions))))
		for _, txn := range remainingFwdTransactions {
			txn.FeeCredit = portion
			availValue = availValue.Sub(portion)
		}
		if !availValue.IsZero() {
			// If there is some remaining value due to division inaccuracy, credit it to the first transaction.
			remainingFwdTransactions[0].FeeCredit = remainingFwdTransactions[0].FeeCredit.Add(availValue)
			availValue = types.NewZeroValue()
		}
	}

	return initialAvailValue.Sub(availValue), nil
}

func (es *ExecutionState) IsInternalTransaction() bool {
	// If contract calls another contract using EVM's call(depth > 1), we treat it as an internal transaction.
	return es.GetInTransaction().IsInternal() || es.evm.GetDepth() > 1
}

func (es *ExecutionState) GetTransactionFlags() types.TransactionFlags {
	return es.GetInTransaction().Flags
}

func (es *ExecutionState) GetInTransaction() *types.Transaction {
	if len(es.InTransactions) == 0 {
		return nil
	}
	return es.InTransactions[len(es.InTransactions)-1]
}

func (es *ExecutionState) GetShardID() types.ShardId {
	return es.ShardId
}

func (es *ExecutionState) CallVerifyExternal(transaction *types.Transaction, account *AccountState) *ExecutionResult {
	methodSignature := "verifyExternal(uint256,bytes)"
	methodSelector := crypto.Keccak256([]byte(methodSignature))[:4]
	argSpec := vm.VerifySignatureArgs()[1:] // skip first arg (pubkey)
	hash, err := transaction.SigningHash()
	if err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("transaction.SigningHash() failed: %w", err))
	}
	argData, err := argSpec.Pack(hash.Big(), ([]byte)(transaction.Signature))
	if err != nil {
		logger.Error().Err(err).Msg("failed to pack arguments")
		return NewExecutionResult().SetFatal(err)
	}

	if err := es.updateGasPrice(transaction); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorBaseFeeTooHigh, err))
	}

	calldata := append(methodSelector, argData...) //nolint:gocritic

	if err := es.newVm(transaction.IsInternal(), transaction.From, nil); err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("newVm failed: %w", err))
	}
	defer es.resetVm()

	gasCreditLimit := ExternalTransactionVerificationMaxGas
	gasAvailable := account.Balance.ToGas(es.GasPrice)

	if gasAvailable.Lt(gasCreditLimit) {
		gasCreditLimit = gasAvailable
	}

	ret, leftOverGas, err := es.evm.StaticCall((vm.AccountRef)(account.address), account.address, calldata, gasCreditLimit.Uint64())
	if err != nil {
		if types.IsOutOfGasError(err) && gasCreditLimit.Lt(ExternalTransactionVerificationMaxGas) {
			// This condition means that account has not enough balance even to execute the verification.
			// So it will be clearer to return `InsufficientBalance` error instead of `OutOfGas`.
			return NewExecutionResult().SetError(types.NewError(types.ErrorInsufficientBalance))
		}
		txnErr := types.KeepOrWrapError(types.ErrorExternalVerificationFailed, err)
		return NewExecutionResult().SetError(txnErr)
	}
	if !bytes.Equal(ret, common.LeftPadBytes([]byte{1}, 32)) {
		return NewExecutionResult().SetError(types.NewError(types.ErrorExternalVerificationFailed))
	}
	res := NewExecutionResult()
	spentGas := gasCreditLimit.Sub(types.Gas(leftOverGas))
	res.SetUsed(spentGas, es.GasPrice)
	es.GasUsed += res.GasUsed
	check.PanicIfErr(account.SubBalance(res.CoinsUsed(), tracing.BalanceDecreaseVerifyExternal))
	return res
}

func (es *ExecutionState) AddToken(addr types.Address, tokenId types.TokenId, amount types.Value) error {
	logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", tokenId).
		Msg("Add token")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetTokenBalance(tokenId)
	if balance == nil {
		balance = &types.Value{}
	}
	newBalance := balance.Add(amount)
	// Amount can be negative(token burning). So, if the new balance is negative, set it to 0
	if newBalance.Cmp(types.Value{}) < 0 {
		newBalance = types.Value{}
	}
	acc.SetTokenBalance(tokenId, newBalance)

	return nil
}

func (es *ExecutionState) SubToken(addr types.Address, tokenId types.TokenId, amount types.Value) error {
	logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", tokenId).
		Msg("Sub token")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetTokenBalance(tokenId)
	if balance == nil {
		balance = &types.Value{}
	}
	if balance.Cmp(amount) < 0 {
		return fmt.Errorf("%w: %s < %s, token %s",
			vm.ErrInsufficientBalance, balance, amount, tokenId)
	}
	acc.SetTokenBalance(tokenId, balance.Sub(amount))

	return nil
}

func (es *ExecutionState) GetTokens(addr types.Address) map[types.TokenId]types.Value {
	acc, err := es.GetAccountReader(addr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get account")
		return nil
	}
	if acc == nil {
		return nil
	}

	res := make(map[types.TokenId]types.Value)
	for k, v := range acc.TokenTrieReader.Iterate() {
		var c types.TokenBalance
		c.Token = types.TokenId(k)
		if err := c.Balance.UnmarshalSSZ(v); err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal token balance")
			continue
		}
		res[c.Token] = c.Balance
	}
	// If some token was changed during execution, we need to set it to the result. It will probably rewrite values
	// fetched from the storage above.
	for id, balance := range *acc.Tokens {
		res[id] = balance
	}

	return res
}

func (es *ExecutionState) GetGasPrice(shardId types.ShardId) (types.Value, error) {
	prices, err := config.GetParamGasPrice(es.GetConfigAccessor())
	if err != nil {
		return types.Value{}, err
	}
	return types.Value{Uint256: &prices.Shards[shardId]}, nil
}

func (es *ExecutionState) Rollback(counter, patchLevel uint32, mainBlock uint64) error {
	es.rollback = &RollbackParams{
		Counter:     counter,
		PatchLevel:  patchLevel,
		MainBlockId: mainBlock,
	}
	return nil
}

func (es *ExecutionState) GetRollback() *RollbackParams {
	return es.rollback
}

func (es *ExecutionState) SetTokenTransfer(tokens []types.TokenBalance) {
	es.evm.SetTokenTransfer(tokens)
}

func (es *ExecutionState) SaveVmState(state *types.EvmState, continuationGasCredit types.Gas) error {
	outTransactions := es.OutTransactions[es.InTransactionHash]
	check.PanicIfNot(len(outTransactions) > 0)

	outTxn := outTransactions[len(outTransactions)-1]
	check.PanicIfNot(outTxn.RequestId != 0)

	data, err := state.MarshalSSZ()
	if err != nil {
		return err
	}

	acc, err := es.GetAccount(es.GetInTransaction().To)
	check.PanicIfErr(err)

	logger.Debug().Int("size", len(data)).Msg("Save vm state")

	acc.SetAsyncContext(types.TransactionIndex(outTxn.RequestId), &types.AsyncContext{
		IsAwait:               true,
		Data:                  data,
		ResponseProcessingGas: continuationGasCredit,
	})
	return nil
}

func (es *ExecutionState) newVm(internal bool, origin types.Address, state *vm.EvmRestoreData) error {
	blockContext, err := NewEVMBlockContext(es)
	if err != nil {
		return err
	}
	es.evm = vm.NewEVM(blockContext, es, origin, es.GasPrice, state)
	es.evm.IsAsyncCall = internal
	return nil
}

func (es *ExecutionState) resetVm() {
	es.evm = nil
}

func (es *ExecutionState) MarshalJSON() ([]byte, error) {
	prevBlockRes, err := es.shardAccessor.GetBlock().ByHash(es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	var prevBlock *types.Block
	if err == nil {
		prevBlock = prevBlockRes.Block()
	}

	data := struct {
		ContractTreeRoot       common.Hash                                  `json:"contractTreeRoot"`
		InTransactionTreeRoot  common.Hash                                  `json:"inTransactionTreeRoot"`
		OutTransactionTreeRoot common.Hash                                  `json:"outTransactionTreeRoot"`
		ReceiptTreeRoot        common.Hash                                  `json:"receiptTreeRoot"`
		PrevBlock              *types.Block                                 `json:"prevBlock"`
		PrevBlockHash          common.Hash                                  `json:"prevBlockHash"`
		MainChainHash          common.Hash                                  `json:"mainChainHash"`
		ShardId                types.ShardId                                `json:"shardId"`
		ChildChainBlocks       map[types.ShardId]common.Hash                `json:"childChainBlocks"`
		GasPrice               types.Value                                  `json:"gasPrice"`
		InTransactions         []*types.Transaction                         `json:"inTransactions"`
		InTransactionHashes    []common.Hash                                `json:"inTransactionHashes"`
		OutTransactions        map[common.Hash][]*types.OutboundTransaction `json:"outTransactions"`
		Receipts               []*types.Receipt                             `json:"receipts"`
		Errors                 map[common.Hash]error                        `json:"errors"`
	}{
		ContractTreeRoot:       es.ContractTree.RootHash(),
		InTransactionTreeRoot:  es.InTransactionTree.RootHash(),
		OutTransactionTreeRoot: es.OutTransactionTree.RootHash(),
		ReceiptTreeRoot:        es.ReceiptTree.RootHash(),
		PrevBlock:              prevBlock,
		PrevBlockHash:          es.PrevBlock,
		MainChainHash:          es.MainChainHash,
		ShardId:                es.ShardId,
		ChildChainBlocks:       es.ChildChainBlocks,
		GasPrice:               es.GasPrice,
		InTransactions:         es.InTransactions,
		InTransactionHashes:    es.InTransactionHashes,
		OutTransactions:        es.OutTransactions,
		Receipts:               es.Receipts,
		Errors:                 es.Errors,
	}

	return json.Marshal(data)
}

func (es *ExecutionState) AppendToJournal(entry JournalEntry) {
	es.journal.append(entry)
}

func (es *ExecutionState) GetRwTx() db.RwTx {
	return es.tx
}
