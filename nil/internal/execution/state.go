package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"unicode/utf8"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

const (
	TraceBlocksEnabled                    = false
	ExternalTransactionVerificationMaxGas = types.Gas(100_000)

	ModeReadOnly     = "read-only"
	ModeProposal     = "proposal"
	ModeSyncReplay   = "syncer-replay"
	ModeManualReplay = "manual-replay"
	ModeVerify       = "verify"

	// TODO: align with protocol – currently using a dummy address to match Ethereum block structure
	CoinbaseShardIndependentAddr = "0x0"
)

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

type IContractMPTRepository interface {
	SetRootHash(root common.Hash) error
	GetAccountState(addr types.Address, createIfNotExists bool) (AccountState, error)
	UpdateContracts(contracts map[types.Address]AccountState) error
	RootHash() common.Hash
	Commit() (common.Hash, error)
}

type TxCounts map[types.ShardId]types.TransactionIndex

type ExecutionState struct {
	tx               db.RwTx
	ContractTree     IContractMPTRepository
	ReceiptTree      *ReceiptTrie
	PrevBlock        common.Hash
	MainShardHash    common.Hash
	ShardId          types.ShardId
	ChildShardBlocks map[types.ShardId]common.Hash
	GasPrice         types.Value // Current gas price including priority fee
	BaseFee          types.Value
	GasLimit         types.Gas

	// Those fields are just copied from the proposal into the block
	// and are not used in the state
	PatchLevel      uint32
	RollbackCounter uint32

	InTransactionHash common.Hash
	Logs              map[common.Hash][]*types.Log
	DebugLogs         map[common.Hash][]*types.DebugLog

	Accounts            map[types.Address]JournaledAccountState
	InTransactions      []*types.Transaction
	InTxCounts          TxCounts
	InTransactionHashes []common.Hash

	// OutTransactions holds outbound transactions for every transaction in the executed block, where key is hash of
	// Transaction that sends the transaction
	OutTransactions map[common.Hash][]*types.OutboundTransaction
	OutTxCounts     TxCounts

	Receipts []*types.Receipt
	Errors   map[common.Hash]error

	GasUsed types.Gas

	CoinbaseAddress types.Address

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

	// Tracing hooks set for every EVM created during execution
	EvmTracingHooks *tracing.Hooks

	shardAccessor *shardAccessor

	// Pointer to currently executed VM
	evm *vm.EVM

	configAccessor config.ConfigAccessor

	// txnFeeCredit holds the total fee credit for the inbound transaction. It can be changed during execution, thus we
	// use this separate variable instead of the one in the transaction.
	txnFeeCredit types.Value

	// isReadOnly is true if the state is in read-only mode. This mode is used for eth_call and eth_estimateGas.
	isReadOnly bool

	FeeCalculator FeeCalculator

	// filled in if a rollback was requested by a transaction
	rollback *RollbackParams

	logger logging.Logger
}

var (
	_ vm.StateDB                = new(ExecutionState)
	_ IRevertableExecutionState = new(ExecutionState)
)

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

func (e *ExecutionResult) String() string {
	if e.Error != nil {
		return fmt.Errorf("error: %w", e.Error).Error()
	}
	if e.FatalError != nil {
		return fmt.Errorf("fatal: %w", e.FatalError).Error()
	}
	return "success"
}

type revision struct {
	id           int
	journalIndex int
}

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
		GasLimit:    es.GasLimit.Uint64(),
		Time:        time,
		Coinbase:    es.CoinbaseAddress,

		RollbackCounter: rollbackCounter,
	}, nil
}

type StateParams struct {
	// Block must be set for non-genesis block.
	Block *types.Block

	// Required parameters
	ConfigAccessor config.ConfigAccessor
	StateAccessor  *StateAccessor

	// Optional parameters
	FeeCalculator         FeeCalculator
	Mode                  string
	GasLimit              types.Gas
	ContractMptRepository IContractMPTRepository
}

func NewExecutionState(tx db.RoTx, shardId types.ShardId, params StateParams) (*ExecutionState, error) {
	var resTx db.RwTx
	isReadOnly := false
	if rwTx, ok := tx.(db.RwTx); ok {
		resTx = rwTx
	} else {
		isReadOnly = true
		resTx = &db.RwWrapper{RoTx: tx}
	}

	l := logging.NewLogger("execution").
		With().
		Stringer(logging.FieldShardId, shardId)
	if params.Mode != "" {
		l = l.Str("mode", params.Mode)
	}
	logger := l.Logger()

	feeCalculator := params.FeeCalculator
	if feeCalculator == nil {
		feeCalculator = &MainFeeCalculator{}
	}

	var baseFeePerGas types.Value
	var prevBlockHash common.Hash
	if params.Block != nil {
		baseFeePerGas = feeCalculator.CalculateBaseFee(params.Block)
		if baseFeePerGas.Cmp(params.Block.BaseFee) != 0 {
			logger.Debug().
				Stringer("Old", params.Block.BaseFee).
				Stringer("New", baseFeePerGas).
				Msg("BaseFee changed")
		}
		prevBlockHash = params.Block.Hash(shardId)
	}
	if params.GasLimit == 0 {
		params.GasLimit = types.DefaultMaxGasInBlock
	}

	res := &ExecutionState{
		tx:               resTx,
		PrevBlock:        prevBlockHash,
		ShardId:          shardId,
		ChildShardBlocks: map[types.ShardId]common.Hash{},
		Accounts:         map[types.Address]JournaledAccountState{},
		OutTransactions:  map[common.Hash][]*types.OutboundTransaction{},
		OutTxCounts:      TxCounts{},
		InTxCounts:       TxCounts{},
		Logs:             map[common.Hash][]*types.Log{},
		DebugLogs:        map[common.Hash][]*types.DebugLog{},
		Errors:           map[common.Hash]error{},
		CoinbaseAddress:  types.ShardAndHexToAddress(shardId, CoinbaseShardIndependentAddr),

		journal:          newJournal(),
		transientStorage: newTransientStorage(),

		shardAccessor:  params.StateAccessor.Access(tx, shardId),
		configAccessor: params.ConfigAccessor,

		BaseFee:  baseFeePerGas,
		GasPrice: types.NewZeroValue(),
		GasLimit: params.GasLimit,

		isReadOnly: isReadOnly,

		FeeCalculator: feeCalculator,

		logger: logger,
	}

	return res, res.initTries(&params)
}

type DbContractAccessor struct {
	*ContractTrie
	rwTxProvider DbRwTxProvider
	logger       logging.Logger
}

var _ IContractMPTRepository = (*DbContractAccessor)(nil)

func (ca *DbContractAccessor) GetAccountState(addr types.Address, createIfNotExists bool) (AccountState, error) {
	smartContract, err := ca.Fetch(addr.Hash())
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("GetAccount failed: %w", err)
	}
	if errors.Is(err, db.ErrKeyNotFound) && !createIfNotExists {
		return nil, nil
	}

	return NewAccountState(ca.rwTxProvider, addr, smartContract, ca.logger)
}

func (ca *DbContractAccessor) UpdateContracts(contracts map[types.Address]AccountState) error {
	keys := make([]common.Hash, 0, len(contracts))
	values := make([]*types.SmartContract, 0, len(contracts))
	for addr, acc := range contracts {
		smartContract, err := acc.Commit()
		if err != nil {
			return err
		}

		keys = append(keys, addr.Hash())
		values = append(values, smartContract)
	}
	return ca.UpdateBatch(keys, values)
}

func (es *ExecutionState) initTries(params *StateParams) error {
	es.ReceiptTree = NewDbReceiptTrie(es.tx, es.ShardId)
	smartContractsRoot := mpt.EmptyRootHash
	outTransactionsRoot := mpt.EmptyRootHash
	inTransactionsRoot := mpt.EmptyRootHash
	if params.Block != nil {
		smartContractsRoot = params.Block.SmartContractsRoot
		outTransactionsRoot = params.Block.OutTransactionsRoot
		inTransactionsRoot = params.Block.InTransactionsRoot
	}
	if err := es.fetchTxCounts(outTransactionsRoot, es.OutTxCounts); err != nil {
		return err
	}
	if err := es.fetchTxCounts(inTransactionsRoot, es.InTxCounts); err != nil {
		return err
	}

	if params.ContractMptRepository != nil {
		es.ContractTree = params.ContractMptRepository
	} else {
		es.ContractTree = &DbContractAccessor{
			ContractTrie: NewDbContractTrie(es.tx, es.ShardId),
			rwTxProvider: es,
			logger:       es.logger,
		}
		if err := es.ContractTree.SetRootHash(smartContractsRoot); err != nil {
			return err
		}
	}
	return nil
}

func (es *ExecutionState) GetConfigAccessor() config.ConfigAccessor {
	return es.configAccessor
}

func (es *ExecutionState) fetchTxCounts(root common.Hash, counts TxCounts) error {
	reader := NewDbTxCountTrieReader(es.tx, es.ShardId)
	if err := reader.SetRootHash(root); err != nil {
		return err
	}
	for shardId, count := range reader.Items() {
		counts[shardId] = count
	}
	return nil
}

func (es *ExecutionState) GetReceipt(txnIndex types.TransactionIndex) (*types.Receipt, error) {
	return es.ReceiptTree.Fetch(txnIndex)
}

func (es *ExecutionState) GetAccountReader(addr types.Address) (AccountReader, error) {
	return es.GetAccount(addr)
}

func (es *ExecutionState) GetAccount(addr types.Address) (JournaledAccountState, error) {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc, nil
	}

	accountState, err := es.ContractTree.GetAccountState(addr, false)
	// if errors.Is(err, db.ErrKeyNotFound) {
	// 	return nil, nil
	// }
	if err != nil {
		return nil, fmt.Errorf("GetAccount failed: %w", err)
	}

	// acc, err = NewAccountState(es, addr, data, es.logger)
	// if err != nil {
	// 	return nil, fmt.Errorf("NewAccountState failed: %w", err)
	// }
	// cache accont state even if nil
	if accountState == nil {
		return nil, nil
	}

	journaledAccountState := NewJournaledAccountStateFromRaw(es, accountState, es.logger)
	es.Accounts[addr] = journaledAccountState
	return journaledAccountState, nil
}

func (es *ExecutionState) setAccountObject(acc JournaledAccountState) {
	// TODO: why unwrap here
	es.Accounts[*acc.GetAddress()] = acc
}

func (es *ExecutionState) AddAddressToAccessList(types.Address) {
}

// AddBalance adds amount to the account associated with addr.
func (es *ExecutionState) AddBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	stateObject, err := es.getOrNewAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	return stateObject.JournaledAddBalance(amount, reason)
}

// SubBalance subtracts amount from the account associated with addr.
func (es *ExecutionState) SubBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	stateObject, err := es.getOrNewAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	return stateObject.JournaledSubBalance(amount, reason)
}

func (es *ExecutionState) AddLog(log *types.Log) error {
	es.AppendToJournal(addLogChange{txhash: es.InTransactionHash})
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
	es.AppendToJournal(refundChange{prev: es.refund})
	es.refund += gas
}

// GetRefund returns the current value of the refund counter.
func (es *ExecutionState) GetRefund() uint64 {
	return es.refund
}

func (es *ExecutionState) AddSlotToAccessList(types.Address, common.Hash) {
}

func (es *ExecutionState) AddressInAccessList(types.Address) bool {
	return true // FIXME
}

func (es *ExecutionState) Empty(addr types.Address) (bool, error) {
	acc, err := es.GetAccount(addr)
	return acc == nil || acc.IsEmpty(), err
}

func (es *ExecutionState) Exists(addr types.Address) (bool, error) {
	acc, err := es.GetAccount(addr)
	return acc != nil, err
}

func (es *ExecutionState) GetCode(addr types.Address) ([]byte, common.Hash, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return nil, types.EmptyCodeHash, err
	}
	return acc.GetCode(), acc.GetCodeHash(), nil
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
		return common.EmptyHash, err
	}
	storageRootHash := acc.GetStorageRoot()
	return storageRootHash, nil
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (es *ExecutionState) SetTransientState(addr types.Address, key, value common.Hash) {
	prev := es.GetTransientState(addr, key)
	if prev == value {
		return
	}
	es.AppendToJournal(transientStorageChange{
		account:  &addr,
		key:      key,
		prevalue: prev,
	})
	es.SetTransientNoJournal(addr, key, value)
}

// SetTransientNoJournal is a lower level setter for transient storage. It
// is called during a revert to prevent modifications to the journal.
func (es *ExecutionState) SetTransientNoJournal(addr types.Address, key, value common.Hash) {
	es.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (es *ExecutionState) GetTransientState(addr types.Address, key common.Hash) common.Hash {
	return es.transientStorage.Get(addr, key)
}

func (es *ExecutionState) Selfdestruct6780(addr types.Address) error {
	stateObject, err := es.GetAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	if stateObject.IsNew() {
		stateObject.JournaledSetIsSelfDestructed(true)
	}
	return nil
}

func (es *ExecutionState) HasSelfDestructed(addr types.Address) (bool, error) {
	stateObject, err := es.GetAccount(addr)
	if err != nil || stateObject == nil {
		return false, err
	}
	return stateObject.IsSelfDestructed(), nil
}

func (es *ExecutionState) SetCode(addr types.Address, code []byte) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.JournaledSetCode(types.Code(code).Hash(), code)
	return nil
}

func (es *ExecutionState) SetInitState(addr types.Address, transaction *types.Transaction) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.SetSeqno(transaction.Seqno)

	if err := es.newVm(transaction.IsInternal(), transaction.From); err != nil {
		return err
	}
	defer es.resetVm()

	_, deployAddr, _, err := es.evm.Deploy(
		addr, vm.AccountRef{}, transaction.Data, uint64(10_000_000) /* gas */, uint256.NewInt(0))
	if err != nil {
		return err
	}
	if addr != deployAddr {
		return errors.New("deploy address is not correct")
	}
	return nil
}

func (es *ExecutionState) SlotInAccessList(types.Address, common.Hash) (addressOk bool, slotOk bool) {
	return true, true // FIXME
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (es *ExecutionState) SubRefund(gas uint64) {
	es.AppendToJournal(refundChange{prev: es.refund})
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
	return acc.JournaledSetState(key, val)
}

func (es *ExecutionState) SetAsyncContext(
	addr types.Address, index types.TransactionIndex, val *types.AsyncContext,
) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.JournaledSetAsyncContext(index, val)
	return nil
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
	return acc.GetBalance(), nil
}

func (es *ExecutionState) GetSeqno(addr types.Address) (types.Seqno, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return 0, err
	}
	return acc.GetSeqno(), nil
}

func (es *ExecutionState) GetExtSeqno(addr types.Address) (types.Seqno, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return 0, err
	}
	return acc.GetExtSeqno(), nil
}

func (es *ExecutionState) getOrNewAccount(addr types.Address) (JournaledAccountState, error) {
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
	acc.JournaledSetBalance(balance)
	return nil
}

func (es *ExecutionState) SetSeqno(addr types.Address, seqno types.Seqno) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.JournaledSetSeqno(seqno)
	return nil
}

func (es *ExecutionState) SetExtSeqno(addr types.Address, seqno types.Seqno) error {
	acc, err := es.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.JournaledSetExtSeqno(seqno)
	return nil
}

func (es *ExecutionState) CreateAccount(addr types.Address) error {
	_, err := es.createAccount(addr)
	return err
}

func (es *ExecutionState) createAccount(addr types.Address) (JournaledAccountState, error) {
	if addr.ShardId() != es.ShardId {
		return nil, fmt.Errorf(
			"attempt to create account %v from %v shard on %v shard", addr, addr.ShardId(), es.ShardId)
	}
	acc, err := es.GetAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return nil, errors.New("account already exists")
	}

	es.AppendToJournal(createAccountChange{account: &addr})

	accountState, err := es.ContractTree.GetAccountState(addr, true)
	if err != nil {
		return nil, err
	}

	check.PanicIfNot(accountState != nil)

	journaledAccountState := NewJournaledAccountStateFromRaw(es, accountState, es.logger)

	es.Accounts[addr] = journaledAccountState
	return journaledAccountState, nil
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
	if !obj.IsNew() {
		obj.JournaledSetIsNew(true)
	}
	return nil
}

// Contract is regarded as existent if any of these three conditions is met:
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
	return (contractHash != types.EmptyCodeHash) || // non-empty code
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
			es.logger.Error().
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

func (es *ExecutionState) AddOutTransaction(
	caller types.Address,
	payload *types.InternalTransactionPayload,
	responseProcessingGas types.Gas,
) (*types.Transaction, error) {
	seqno, err := es.GetSeqno(caller)
	if err != nil {
		return nil, err
	}

	if seqno+1 < seqno {
		return nil, vm.ErrNonceUintOverflow
	}
	if err := es.SetSeqno(caller, seqno+1); err != nil {
		return nil, err
	}

	txn := payload.ToTransaction(caller, seqno)

	// Propagate fee settings from the inbound message
	txn.MaxPriorityFeePerGas = es.GetInTransaction().MaxPriorityFeePerGas
	txn.MaxFeePerGas = es.GetInTransaction().MaxFeePerGas

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

	// Use next TxId
	txn.TxId = es.OutTxCounts[txn.To.ShardId()]
	es.OutTxCounts[txn.To.ShardId()] = txn.TxId + 1

	txnHash := txn.Hash()

	es.logger.Trace().
		Stringer(logging.FieldTransactionHash, txnHash).
		Stringer(logging.FieldTransactionFrom, txn.From).
		Stringer(logging.FieldTransactionTo, txn.To).
		Msg("Outbound transaction added")

	es.AppendToJournal(outTransactionsChange{
		txnHash: es.InTransactionHash,
		index:   len(es.OutTransactions[es.InTransactionHash]),
	})

	outTxn := &types.OutboundTransaction{Transaction: txn, TxnHash: txnHash, ForwardKind: payload.ForwardKind}
	es.OutTransactions[es.InTransactionHash] = append(es.OutTransactions[es.InTransactionHash], outTxn)

	if txn.RequestId != 0 {
		acc, err := es.GetAccount(caller)
		check.PanicIfErr(err)

		acc.JournaledSetAsyncContext(types.TransactionIndex(txn.RequestId), &types.AsyncContext{
			ResponseProcessingGas: responseProcessingGas,
		})
	}

	return txn, nil
}

func (es *ExecutionState) sendBounceTransaction(txn *types.Transaction, execResult *ExecutionResult) (bool, error) {
	if txn.Value.IsZero() && len(txn.Token) == 0 {
		return false, nil
	}
	if txn.BounceTo == types.EmptyAddress {
		es.logger.Debug().Msg("Bounce transaction not sent, no bounce address")
		return false, nil
	}

	data, err := contracts.NewCallData(contracts.NameNilBounceable, "bounce", execResult.Error.Error())
	if err != nil {
		return false, err
	}

	check.PanicIfNotf(
		execResult.CoinsForwarded.IsZero(),
		"CoinsForwarded should be zero when sending bounce transaction")
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
	if _, err = es.AddOutTransaction(txn.To, bounceTxn, 0); err != nil {
		return false, err
	}
	es.logger.Debug().
		Stringer(logging.FieldTransactionFrom, txn.To).
		Stringer(logging.FieldTransactionTo, txn.BounceTo).
		Msg("Bounce transaction sent")
	return true, nil
}

func (es *ExecutionState) SendResponseTransaction(txn *types.Transaction, res *ExecutionResult) error {
	asyncResponsePayload := types.AsyncResponsePayload{
		Success:    !res.Failed(),
		ReturnData: res.ReturnData,
	}
	data, err := asyncResponsePayload.MarshalNil()
	if err != nil {
		return err
	}

	responsePayload := &types.InternalTransactionPayload{
		Kind:        types.ResponseTransactionKind,
		ForwardKind: types.ForwardKindRemaining,
		Data:        data,
	}

	// Send back value in case of failed transaction, so that we don't need a separate bounce transaction
	if res.Failed() {
		responsePayload.Value = txn.Value
	}

	requestChain := txn.RequestChain
	if txn.IsRequest() {
		responsePayload.To = txn.From
		responsePayload.RequestId = txn.RequestId
	} else {
		// We are processing a response transaction with requests chain. So get pending request from the chain and send
		// response to it.
		check.PanicIfNotf(txn.IsResponse(), "Transaction should be a response")
		responsePayload.To = txn.RequestChain[len(txn.RequestChain)-1].Caller
		responsePayload.RequestId = txn.RequestChain[len(txn.RequestChain)-1].Id
		requestChain = txn.RequestChain[:len(txn.RequestChain)-1]
	}

	// TODO: need to pay for response here
	// we pay for mem during VM execution, so likely big response isn't a problem
	responseTxn, err := es.AddOutTransaction(txn.To, responsePayload, 0)
	if err != nil {
		return err
	}
	responseTxn.RequestChain = requestChain
	return nil
}

func (es *ExecutionState) AcceptInternalTransaction(tx *types.Transaction) error {
	check.PanicIfNot(tx.IsInternal())

	nextTxId := es.InTxCounts[tx.From.ShardId()]
	if tx.TxId != nextTxId {
		return types.NewError(types.ErrorTxIdGap)
	}
	es.InTxCounts[tx.From.ShardId()] = nextTxId + 1

	if tx.IsDeploy() {
		return ValidateDeployTransaction(tx)
	}
	return nil
}

func (es *ExecutionState) HandleTransaction(
	ctx context.Context, txn *types.Transaction, payer Payer,
) (retError *ExecutionResult) {
	defer func() {
		var ev *logging.Event
		if retError.Failed() {
			ev = es.logger.Info()
		} else {
			if es.logger.GetLevel() > zerolog.DebugLevel {
				return
			}
			ev = es.logger.Debug()
		}
		ev.Stringer(logging.FieldTransactionHash, es.InTransactionHash)
		ev.Stringer("result", retError).Int(logging.FieldTransactionSeqno, int(txn.Seqno))
		if !txn.IsRefund() && !txn.IsBounce() {
			ev.Stringer("gasUsed", retError.GasUsed).
				Stringer("gasPrice", retError.GasPrice)
		}
		if retError.Failed() {
			failedPc := uint64(0)
			if retError.DebugInfo != nil {
				failedPc = retError.DebugInfo.Pc
			}
			ev.Int("failedPc", int(failedPc))
		}
		if retError.Failed() {
			ev.Msg("Transaction completed with error")
		} else {
			ev.Msg("Transaction completed successfully")
		}
	}()

	// Catch panic during execution and return it as an error
	defer func() {
		if recResult := recover(); recResult != nil {
			if err, ok := recResult.(error); ok {
				retError = NewExecutionResult().SetError(types.NewWrapError(types.ErrorPanicDuringExecution, err))
			} else {
				retError = NewExecutionResult().SetError(
					types.NewVerboseError(
						types.ErrorPanicDuringExecution,
						fmt.Sprintf("panic transaction: %v", recResult)))
			}
		}
	}()

	if txn.IsExternal() {
		addr := txn.To
		seqno, err := es.GetExtSeqno(addr)
		if err != nil {
			return NewExecutionResult().SetFatal(err)
		}
		if err := es.SetExtSeqno(addr, seqno+1); err != nil {
			return NewExecutionResult().SetFatal(err)
		}

		defer func() {
			// Execution message pays for verifyExternal.
			// We need to revert ExtSeqno only for Deploy messages that doesn't spend gas.
			if txn.IsDeploy() && retError.GasUsed == 0 {
				check.PanicIfErr(es.SetExtSeqno(txn.To, seqno))
			}
		}()
	}

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

	if err := es.transferPriorityFee(res.GasUsed.ToValue(txn.MaxPriorityFeePerGas)); err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("transferPriorityFee failed: %w", err))
	}

	responseWasSent := false
	bounced := false
	if txn.IsRequest() {
		if err := es.SendResponseTransaction(txn, res); err != nil {
			return NewExecutionResult().SetFatal(fmt.Errorf("SendResponseTransaction failed: %w", err))
		}
		bounced = true
		responseWasSent = true
	} else if txn.IsResponse() && len(txn.RequestChain) > 0 {
		// There is pending requests in the chain, so we need to send response to them.
		// But we don't send response if a new request was sent during the execution.
		if err := es.SendResponseTransaction(txn, res); err != nil {
			return NewExecutionResult().SetFatal(fmt.Errorf("SendResponseTransaction failed: %w", err))
		}
		responseWasSent = true
	}
	// We don't need bounce transaction for request, because it will be sent within the response transaction.
	if res.Error != nil && !responseWasSent {
		if res.Error.Code() == types.ErrorExecutionReverted {
			revString := decodeRevertTransaction(res.ReturnData)
			if revString != "" {
				if types.IsVmError(res.Error) {
					res.Error = types.NewVmVerboseError(res.Error.Code(), revString)
				} else {
					res.Error = types.NewVerboseError(res.Error.Code(), revString)
				}
			}
		}
		if txn.IsBounce() {
			es.logger.Error().Err(res.Error).Msg("VM returns error during bounce transaction processing")
		} else {
			es.logger.Debug().Err(res.Error).Msg("execution txn failed")
			if txn.IsInternal() {
				var bounceErr error
				if bounced, bounceErr = es.sendBounceTransaction(txn, res); bounceErr != nil {
					es.logger.Error().Err(bounceErr).Msg("Bounce transaction sent failed")
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

func (es *ExecutionState) handleDeployTransaction(_ context.Context, transaction *types.Transaction) (
	result *ExecutionResult,
) {
	addr := transaction.To
	deployTxn := types.ParseDeployPayload(transaction.Data)

	es.logger.Debug().
		Stringer(logging.FieldTransactionTo, addr).
		Msg("Handling deploy transaction...")

	if err := es.newVm(transaction.IsInternal(), transaction.From); err != nil {
		return NewExecutionResult().SetFatal(err)
	}
	defer es.resetVm()

	es.preTxHookCall(transaction)
	defer func() { es.postTxHookCall(transaction, result) }()

	gas, exceedBlockLimit := es.calcGasLimit(es.txnFeeCredit.ToGas(es.GasPrice))
	ret, addr, leftOver, err := es.evm.Deploy(
		addr, (vm.AccountRef)(transaction.From), deployTxn.Code(), gas.Uint64(), transaction.Value.Int())
	if exceedBlockLimit && types.IsOutOfGasError(err) {
		err = types.NewError(types.ErrorTransactionExceedsBlockGasLimit)
	}

	event := es.logger.Debug().Stringer(logging.FieldTransactionTo, addr)
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

func (es *ExecutionState) TryProcessResponse(
	transaction *types.Transaction,
) ([]byte, *ExecutionResult) {
	if !transaction.IsResponse() {
		return transaction.Data, nil
	}
	var callData []byte

	check.PanicIfNot(transaction.RequestId != 0)
	acc, err := es.GetAccount(transaction.To)
	if err != nil {
		return nil, NewExecutionResult().SetFatal(err)
	}
	asyncContext, err := acc.GetAndRemoveAsyncContext(types.TransactionIndex(transaction.RequestId))
	if err != nil {
		return nil, NewExecutionResult().SetFatal(fmt.Errorf("failed to get async context %s (%d): %w",
			transaction.To, transaction.RequestId, err))
	}

	responsePayload := new(types.AsyncResponsePayload)
	if err := responsePayload.UnmarshalNil(transaction.Data); err != nil {
		return nil, NewExecutionResult().SetFatal(
			fmt.Errorf("AsyncResponsePayload unmarshal failed: %w", err))
	}

	es.txnFeeCredit = es.txnFeeCredit.Add(asyncContext.ResponseProcessingGas.ToValue(es.GasPrice))

	methodSignature := "onFallback(uint256,bool,bytes)"
	methodSelector := crypto.Keccak256([]byte(methodSignature))[:4]

	uint256Ty, _ := abi.NewType("uint256", "", nil)
	boolTy, _ := abi.NewType("bool", "", nil)
	bytesTy, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		abi.Argument{Name: "answer_id", Type: uint256Ty},
		abi.Argument{Name: "success", Type: boolTy},
		abi.Argument{Name: "response", Type: bytesTy},
	}

	if callData, err = args.Pack(
		types.NewUint256(transaction.RequestId),
		responsePayload.Success,
		responsePayload.ReturnData,
	); err != nil {
		return nil, NewExecutionResult().SetFatal(err)
	}
	return append(methodSelector, callData...), nil
}

func (es *ExecutionState) handleExecutionTransaction(
	_ context.Context,
	transaction *types.Transaction,
) (res *ExecutionResult) {
	if assert.Enable {
		check.PanicIfNot(transaction.Hash() == es.InTransactionHash)
	}

	check.PanicIfNot(transaction.IsExecution())
	addr := transaction.To
	es.logger.Debug().
		Stringer(logging.FieldTransactionFrom, transaction.From).
		Stringer(logging.FieldTransactionTo, addr).
		Stringer(logging.FieldTransactionFlags, transaction.Flags).
		Stringer(logging.FieldTransactionHash, es.InTransactionHash).
		Stringer("value", transaction.Value).
		Stringer("feeCredit", transaction.FeeCredit).
		Msg("Handling execution transaction...")

	caller := (vm.AccountRef)(transaction.From)

	callData, res := es.TryProcessResponse(transaction)
	if res != nil && res.Failed() {
		return res
	}

	if err := es.newVm(transaction.IsInternal(), transaction.From); err != nil {
		return NewExecutionResult().SetFatal(err)
	}
	defer es.resetVm()

	es.preTxHookCall(transaction)
	defer func() { es.postTxHookCall(transaction, res) }()

	es.revertId = es.Snapshot()

	gas, exceedBlockLimit := es.calcGasLimit(es.txnFeeCredit.ToGas(es.GasPrice))
	es.evm.SetTokenTransfer(transaction.Token)
	ret, leftOver, err := es.evm.Call(caller, addr, callData, gas.Uint64(), transaction.Value.Int())

	if exceedBlockLimit && types.IsOutOfGasError(err) {
		err = types.NewError(types.ErrorTransactionExceedsBlockGasLimit)
	}

	return NewExecutionResult().
		SetTxnErrorOrFatal(err).
		SetUsed(gas-types.Gas(leftOver), es.GasPrice).
		SetReturnData(ret).SetDebugInfo(es.evm.DebugInfo)
}

func (es *ExecutionState) calcGasLimit(gas types.Gas) (types.Gas, bool) {
	if gas > es.GasLimit {
		return es.GasLimit, true
	}
	return gas, false
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
		if !utf8.ValidString(revString) {
			return "Not a UTF-8 string: " + hexutil.Encode(data[:index])
		}
	}
	return revString
}

func (es *ExecutionState) handleRefundTransaction(_ context.Context, transaction *types.Transaction) error {
	err := es.AddBalance(transaction.To, transaction.Value, tracing.BalanceIncreaseRefund)
	es.logger.Debug().Err(err).Msgf("Refunded %s to %v", transaction.Value, transaction.To)
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

func getOutTransactions(es *ExecutionState) ([]*types.Transaction, []common.Hash) {
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

func (es *ExecutionState) writeTxCounts(root common.Hash, counts TxCounts) (common.Hash, error) {
	if len(counts) == 0 {
		return root, nil
	}
	keys := make([]types.ShardId, 0, len(counts))
	values := make([]*types.TransactionIndex, 0, len(counts))
	for shard, count := range counts {
		if count > 0 {
			keys = append(keys, shard)
			cnt := count
			values = append(values, &cnt)
		}
	}
	trie := NewDbTxCountTrie(es.tx, es.ShardId)
	if err := trie.SetRootHash(root); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to set tx count trie root hash: %w", err)
	}
	if err := trie.UpdateBatch(keys, values); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to update tx count trie: %w", err)
	}
	return trie.Commit()
}

func (es *ExecutionState) BuildBlock(blockId types.BlockNumber) (*BlockGenerationResult, error) {
	// Update contract tree with current account states
	if err := es.updateContractTree(); err != nil {
		return nil, err
	}

	// Build child shard blocks tree
	treeShardsRootHash, err := es.buildChildShardBlocksTree(blockId)
	if err != nil {
		return nil, err
	}

	// Prepare output transactions
	outTxnValues, outTxnHashes := getOutTransactions(es)

	// Build transaction trees
	inTxRoot, outTxRoot, err := es.buildTransactionTrees(outTxnValues)
	if err != nil {
		return nil, err
	}

	// Validate transactions and receipts
	if err := es.validateTransactionsAndReceipts(); err != nil {
		return nil, err
	}

	// Update receipts tree
	if err := es.updateReceiptsTree(); err != nil {
		return nil, err
	}

	// Handle configuration (main shard only)
	configRoot, configParams, err := es.handleConfiguration()
	if err != nil {
		return nil, err
	}

	// Commit all trees
	contractTree, receiptTree, err := es.commitTrees()
	if err != nil {
		return nil, err
	}

	// Build the final block
	block := es.buildFinalBlock(
		blockId, contractTree, inTxRoot, outTxRoot, configRoot, receiptTree, treeShardsRootHash, outTxnValues,
	)

	return &BlockGenerationResult{
		Block:        block,
		BlockHash:    block.Hash(es.ShardId),
		InTxns:       es.InTransactions,
		InTxnHashes:  es.InTransactionHashes,
		OutTxns:      outTxnValues,
		OutTxnHashes: outTxnHashes,
		Receipts:     es.Receipts,
		ConfigParams: configParams,
	}, nil
}

func (es *ExecutionState) updateContractTree() error {
	accounts := make(map[types.Address]AccountState)
	for addr, journaledState := range es.Accounts {
		accounts[addr] = journaledState
	}
	return es.ContractTree.UpdateContracts(accounts)
}

func (es *ExecutionState) buildChildShardBlocksTree(blockId types.BlockNumber) (common.Hash, error) {
	treeShardsRootHash := mpt.EmptyRootHash
	if len(es.ChildShardBlocks) > 0 {
		treeShards := NewDbShardBlocksTrie(es.tx, es.ShardId, blockId)
		if err := UpdateFromMap(
			treeShards, es.ChildShardBlocks, func(v common.Hash) *common.Hash { return &v },
		); err != nil {
			return common.Hash{}, err
		}
		var err error
		if treeShardsRootHash, err = treeShards.Commit(); err != nil {
			return common.Hash{}, err
		}
	}
	return treeShardsRootHash, nil
}

func (es *ExecutionState) buildTransactionTrees(outTxnValues []*types.Transaction) (common.Hash, common.Hash, error) {
	// Build inbound transaction tree
	inTxRoot, err := es.buildInboundTransactionTree()
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}

	// Build outbound transaction tree
	outTxRoot, err := es.buildOutboundTransactionTree(outTxnValues)
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}

	return inTxRoot, outTxRoot, nil
}

func (es *ExecutionState) buildInboundTransactionTree() (common.Hash, error) {
	inTxnKeys := make([]types.TransactionIndex, 0, len(es.InTransactions))
	inTxnValues := make([]*types.Transaction, 0, len(es.InTransactions))
	for i, txn := range es.InTransactions {
		inTxnKeys = append(inTxnKeys, types.TransactionIndex(i))
		inTxnValues = append(inTxnValues, txn)
	}

	inTransactionTree := NewDbTransactionTrie(es.tx, es.ShardId)
	if err := inTransactionTree.UpdateBatch(inTxnKeys, inTxnValues); err != nil {
		return common.Hash{}, err
	}

	inTransactionTreeHash, err := inTransactionTree.Commit()
	if err != nil {
		return common.Hash{}, err
	}

	return es.writeTxCounts(inTransactionTreeHash, es.InTxCounts)
}

func (es *ExecutionState) buildOutboundTransactionTree(outTxnValues []*types.Transaction) (common.Hash, error) {
	outTxnKeys := make([]types.TransactionIndex, 0, len(outTxnValues))
	for i := range outTxnValues {
		outTxnKeys = append(outTxnKeys, types.TransactionIndex(i))
	}

	outTransactionTree := NewDbTransactionTrie(es.tx, es.ShardId)
	if err := outTransactionTree.UpdateBatch(outTxnKeys, outTxnValues); err != nil {
		return common.Hash{}, err
	}

	outTransactionTreeHash, err := outTransactionTree.Commit()
	if err != nil {
		return common.Hash{}, err
	}

	return es.writeTxCounts(outTransactionTreeHash, es.OutTxCounts)
}

func (es *ExecutionState) validateTransactionsAndReceipts() error {
	if assert.Enable {
		if err := es.validateOutboundTransactions(); err != nil {
			return err
		}
		if err := es.validateReceiptHashes(); err != nil {
			return err
		}
	}

	if len(es.InTransactions) != len(es.Receipts) {
		return fmt.Errorf(
			"number of transactions does not match number of receipts: %d != %d",
			len(es.InTransactions), len(es.Receipts))
	}

	return nil
}

func (es *ExecutionState) validateOutboundTransactions() error {
	for outTxnHash := range es.OutTransactions {
		if outTxnHash == common.EmptyHash {
			continue // Skip transactions transmitted over the topology
		}
		found := false
		for _, txnHash := range es.InTransactionHashes {
			if txnHash == outTxnHash {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("outbound transaction %v does not belong to any inbound transaction", outTxnHash)
		}
	}
	return nil
}

func (es *ExecutionState) validateReceiptHashes() error {
	for i, txnHash := range es.InTransactionHashes {
		if txnHash != es.Receipts[i].TxnHash {
			return fmt.Errorf("receipt hash doesn't match its transaction #%d", i)
		}
	}
	return nil
}

func (es *ExecutionState) updateReceiptsTree() error {
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

	return es.ReceiptTree.UpdateBatch(receiptKeys, receiptValues)
}

func (es *ExecutionState) handleConfiguration() (common.Hash, map[string][]byte, error) {
	configRoot := mpt.EmptyRootHash
	var configParams map[string][]byte

	if !es.ShardId.IsMainShard() {
		return configRoot, configParams, nil
	}

	// Handle config root
	prevBlock, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return common.Hash{}, nil, fmt.Errorf("failed to read previous block: %w", err)
	}
	if prevBlock != nil {
		configRoot = prevBlock.ConfigRoot
	}

	if configRoot, err = es.GetConfigAccessor().Commit(es.tx, configRoot); err != nil {
		return common.Hash{}, nil, fmt.Errorf("failed to update config trie: %w", err)
	}

	if configParams, err = es.GetConfigAccessor().GetParams(); err != nil {
		return common.Hash{}, nil, fmt.Errorf("failed to read config params: %w", err)
	}

	return configRoot, configParams, nil
}

func (es *ExecutionState) getL1BlockNumber() uint64 {
	if !es.ShardId.IsMainShard() {
		return 0
	}

	if l1Block, err := config.GetParamL1Block(es.configAccessor); err == nil {
		return l1Block.Number
	}
	return 0
}

func (es *ExecutionState) commitTrees() (common.Hash, common.Hash, error) {
	contractTree, err := es.ContractTree.Commit()
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}

	receiptTree, err := es.ReceiptTree.Commit()
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}

	return contractTree, receiptTree, nil
}

func (es *ExecutionState) buildFinalBlock(
	blockId types.BlockNumber,
	contractTree common.Hash,
	inTxRoot common.Hash,
	outTxRoot common.Hash,
	configRoot common.Hash,
	receiptTree common.Hash,
	treeShardsRootHash common.Hash,
	outTxnValues []*types.Transaction,
) *types.Block {
	l1BlockNumber := es.getL1BlockNumber()

	return &types.Block{
		BlockData: types.BlockData{
			Id:                  blockId,
			PrevBlock:           es.PrevBlock,
			SmartContractsRoot:  contractTree,
			InTransactionsRoot:  inTxRoot,
			OutTransactionsRoot: outTxRoot,
			ConfigRoot:          configRoot,
			OutTransactionsNum:  types.TransactionIndex(len(outTxnValues)),
			ReceiptsRoot:        receiptTree,
			ChildBlocksRootHash: treeShardsRootHash,
			MainShardHash:       es.MainShardHash,
			BaseFee:             es.BaseFee,
			GasUsed:             es.GasUsed,
			L1BlockNumber:       l1BlockNumber,
			PatchLevel:          es.PatchLevel,
			RollbackCounter:     es.RollbackCounter,
		},
		LogsBloom: types.CreateBloom(es.Receipts),
	}
}

func (es *ExecutionState) Commit(
	blockId types.BlockNumber,
	params *types.ConsensusParams,
) (*BlockGenerationResult, error) {
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

	es.logger.Trace().
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

func (es *ExecutionState) CallVerifyExternal(
	transaction *types.Transaction,
	account AccountState,
) (res *ExecutionResult) {
	methodSignature := "verifyExternal(uint256,bytes)"
	methodSelector := crypto.Keccak256([]byte(methodSignature))[:4]
	argSpec := vm.VerifySignatureArgs()[1:] // skip first arg (pubkey)
	hash, err := transaction.SigningHash()
	if err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("transaction.SigningHash() failed: %w", err))
	}
	argData, err := argSpec.Pack(hash.Big(), ([]byte)(transaction.Signature))
	if err != nil {
		es.logger.Error().Err(err).Msg("failed to pack arguments")
		return NewExecutionResult().SetFatal(err)
	}

	if err := es.updateGasPrice(transaction); err != nil {
		return NewExecutionResult().SetError(types.KeepOrWrapError(types.ErrorBaseFeeTooHigh, err))
	}

	calldata := append(methodSelector, argData...) //nolint:gocritic

	if err := es.newVm(transaction.IsInternal(), transaction.From); err != nil {
		return NewExecutionResult().SetFatal(fmt.Errorf("newVm failed: %w", err))
	}
	defer es.resetVm()

	es.preTxHookCall(transaction)
	defer func() { es.postTxHookCall(transaction, res) }()

	gasCreditLimit := ExternalTransactionVerificationMaxGas
	gasAvailable := account.GetBalance().ToGas(es.GasPrice)

	if gasAvailable.Lt(gasCreditLimit) {
		gasCreditLimit = gasAvailable
	}

	ret, leftOverGas, err := es.evm.StaticCall(
		(vm.AccountRef)(*account.GetAddress()), *account.GetAddress(), calldata, gasCreditLimit.Uint64())
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
	res = NewExecutionResult()
	spentGas := gasCreditLimit.Sub(types.Gas(leftOverGas))
	res.SetUsed(spentGas, es.GasPrice)
	es.GasUsed += res.GasUsed
	check.PanicIfErr(account.SubBalance(res.CoinsUsed(), tracing.BalanceDecreaseVerifyExternal))
	return res
}

func (es *ExecutionState) AddToken(addr types.Address, tokenId types.TokenId, amount types.Value) error {
	es.logger.Debug().
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
	acc.JournaledSetTokenBalance(tokenId, newBalance)

	return nil
}

func (es *ExecutionState) SubToken(addr types.Address, tokenId types.TokenId, amount types.Value) error {
	es.logger.Debug().
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
	acc.JournaledSetTokenBalance(tokenId, balance.Sub(amount))

	return nil
}

func (es *ExecutionState) GetTokens(addr types.Address) map[types.TokenId]types.Value {
	acc, err := es.GetAccountReader(addr)
	if err != nil {
		es.logger.Error().Err(err).Msg("failed to get account")
		return nil
	}
	if acc == nil {
		return nil
	}

	return acc.GetTokens()
}

func (es *ExecutionState) GetGasPrice(shardId types.ShardId) (types.Value, error) {
	prices, err := config.GetParamGasPrice(es.GetConfigAccessor())
	if err != nil {
		return types.Value{}, err
	}
	if int(shardId) >= len(prices.Shards) {
		return types.Value{}, fmt.Errorf("shard %d is not found in gas prices", shardId)
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

func (es *ExecutionState) newVm(internal bool, origin types.Address) error {
	blockContext, err := NewEVMBlockContext(es)
	if err != nil {
		return err
	}
	es.evm = vm.NewEVM(blockContext, es, origin, es.GasPrice)
	es.evm.IsAsyncCall = internal

	es.evm.Config.Tracer = es.EvmTracingHooks

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
		ContractTreeRoot    common.Hash                                  `json:"contractTreeRoot"`
		ReceiptTreeRoot     common.Hash                                  `json:"receiptTreeRoot"`
		PrevBlock           *types.Block                                 `json:"prevBlock"`
		PrevBlockHash       common.Hash                                  `json:"prevBlockHash"`
		MainShardHash       common.Hash                                  `json:"mainShardHash"`
		ShardId             types.ShardId                                `json:"shardId"`
		ChildShardBlocks    map[types.ShardId]common.Hash                `json:"childShardBlocks"`
		GasPrice            types.Value                                  `json:"gasPrice"`
		InTransactions      []*types.Transaction                         `json:"inTransactions"`
		InTransactionHashes []common.Hash                                `json:"inTransactionHashes"`
		OutTransactions     map[common.Hash][]*types.OutboundTransaction `json:"outTransactions"`
		Receipts            []*types.Receipt                             `json:"receipts"`
		Errors              map[common.Hash]error                        `json:"errors"`
	}{
		ContractTreeRoot:    es.ContractTree.RootHash(),
		ReceiptTreeRoot:     es.ReceiptTree.RootHash(),
		PrevBlock:           prevBlock,
		PrevBlockHash:       es.PrevBlock,
		MainShardHash:       es.MainShardHash,
		ShardId:             es.ShardId,
		ChildShardBlocks:    es.ChildShardBlocks,
		GasPrice:            es.GasPrice,
		InTransactions:      es.InTransactions,
		InTransactionHashes: es.InTransactionHashes,
		OutTransactions:     es.OutTransactions,
		Receipts:            es.Receipts,
		Errors:              es.Errors,
	}

	return json.Marshal(data)
}

func (es *ExecutionState) AppendToJournal(entry JournalEntry) {
	es.journal.append(entry)
}

func (es *ExecutionState) GetRwTx() db.RwTx {
	return es.tx
}

func (es *ExecutionState) DeleteAccount(addr types.Address) {
	delete(es.Accounts, addr)
}

func (es *ExecutionState) SetRefund(value uint64) {
	es.refund = value
}

func (es *ExecutionState) DeleteLog(txHash common.Hash) {
	logs := es.Logs[txHash]
	if len(logs) == 1 {
		delete(es.Logs, txHash)
	} else {
		es.Logs[txHash] = logs[:len(logs)-1]
	}
}

func (es *ExecutionState) DeleteOutTransaction(index int, txnHash common.Hash) {
	outTransactions, ok := es.OutTransactions[txnHash]
	check.PanicIfNot(ok)

	// Probably it is possible that the transaction is not the last in the list, but let's assume it is for a now.
	// And catch opposite case with this assert.
	check.PanicIfNot(index == len(outTransactions)-1)

	txn := outTransactions[index]
	toShard := txn.To.ShardId()
	check.PanicIfNot(es.OutTxCounts[toShard] == txn.TxId+1)
	es.OutTxCounts[toShard]--
	es.OutTransactions[txnHash] = outTransactions[:index]
}

func (es *ExecutionState) preTxHookCall(txn *types.Transaction) {
	if es.EvmTracingHooks != nil && es.EvmTracingHooks.OnTxEnd != nil {
		es.EvmTracingHooks.OnTxStart(es.evm.GetVMContext(), txn)
	}
}

func (es *ExecutionState) postTxHookCall(txn *types.Transaction, txResult *ExecutionResult) {
	if es.EvmTracingHooks != nil && es.EvmTracingHooks.OnTxEnd != nil {
		es.EvmTracingHooks.OnTxEnd(es.evm.GetVMContext(), txn, txResult.Error)
	}
}

func VerboseTracingHooks(logger logging.Logger) *tracing.Hooks {
	return &tracing.Hooks{
		OnOpcode: func(
			pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error,
		) {
			for i, item := range scope.StackData() {
				logger.Debug().Msgf("     %d: %s", i, item.String())
			}
			logger.Debug().Msgf("%04x: %s", pc, vm.OpCode(op).String())
		},
	}
}

func (es *ExecutionState) transferPriorityFee(priorityFee types.Value) error {
	return es.AddBalance(es.CoinbaseAddress, priorityFee, tracing.BalanceIncreaseRewardTransactionFee)
}
