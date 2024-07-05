package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/contracts"
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

	// If true, log every instruction execution.
	TraceVm bool

	Accessor *StateAccessor

	// Pointer to currently executed VM
	evm *vm.EVM
}

type revision struct {
	id           int
	journalIndex int
}

var _ vm.StateDB = new(ExecutionState)

func NewAccountState(es *ExecutionState, addr types.Address, tx db.RwTx, account *types.SmartContract) (*AccountState, error) {
	shardId := addr.ShardId()

	// TODO: store storage of each contract in separate table
	root := NewStorageTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.StorageTrieTable, account.StorageRoot))

	currencyRoot := NewCurrencyTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.CurrencyTrieTable, account.CurrencyRoot))

	code, err := db.ReadCode(tx, shardId, account.CodeHash)
	if err != nil {
		return nil, err
	}

	return &AccountState{
		db:      es,
		address: addr,

		Tx:           tx,
		Balance:      account.Balance,
		CurrencyTree: currencyRoot,
		StorageTree:  root,
		CodeHash:     account.CodeHash,
		Code:         code,
		ExtSeqno:     account.ExtSeqno,
		Seqno:        account.Seqno,
		State:        make(Storage),
	}, nil
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

func NewROExecutionState(tx db.RoTx, shardId types.ShardId, blockHash common.Hash, timer common.Timer) (*ExecutionState, error) {
	return NewExecutionState(&db.RwWrapper{RoTx: tx}, shardId, blockHash, timer)
}

func NewROExecutionStateForShard(tx db.RoTx, shardId types.ShardId, timer common.Timer) (*ExecutionState, error) {
	return NewExecutionStateForShard(&db.RwWrapper{RoTx: tx}, shardId, timer)
}

func NewExecutionState(tx db.RwTx, shardId types.ShardId, blockHash common.Hash, timer common.Timer) (*ExecutionState, error) {
	accessor, err := NewStateAccessor()
	if err != nil {
		return nil, err
	}

	res := &ExecutionState{
		tx:               tx,
		Timer:            timer,
		PrevBlock:        blockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[types.ShardId]common.Hash{},
		Accounts:         map[types.Address]*AccountState{},
		OutMessages:      map[common.Hash][]*types.Message{},
		Logs:             map[common.Hash][]*types.Log{},
		Errors:           map[common.Hash]error{},

		journal:          newJournal(),
		transientStorage: newTransientStorage(),

		Accessor: accessor,
	}
	return res, res.initTries()
}

func (es *ExecutionState) initTries() error {
	block, err := db.ReadBlock(es.tx, es.ShardId, es.PrevBlock)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	if block != nil {
		es.ContractTree = NewContractTrie(mpt.NewMerklePatriciaTrieWithRoot(es.tx, es.ShardId, db.ContractTrieTable, block.SmartContractsRoot))
	} else {
		es.ContractTree = NewContractTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.ContractTrieTable))
	}
	es.InMessageTree = NewMessageTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.MessageTrieTable))
	es.OutMessageTree = NewMessageTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.MessageTrieTable))
	es.ReceiptTree = NewReceiptTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.ReceiptTrieTable))

	return nil
}

func NewExecutionStateForShard(tx db.RwTx, shardId types.ShardId, timer common.Timer) (*ExecutionState, error) {
	hash, err := db.ReadLastBlockHash(tx, shardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed getting last block: %w", err)
	}
	return NewExecutionState(tx, shardId, hash, timer)
}

func (es *ExecutionState) GetReceipt(msgIndex types.MessageIndex) (*types.Receipt, error) {
	return es.ReceiptTree.Fetch(msgIndex)
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

	acc, err = NewAccountState(es, addr, es.tx, data)
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
	stateObject.AddBalance(amount, reason)
	return nil
}

// SubBalance subtracts amount from the account associated with addr.
func (es *ExecutionState) SubBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	stateObject, err := es.getOrNewAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	stateObject.SubBalance(amount, reason)
	return nil
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

	if err := es.newVm(message.IsInternal()); err != nil {
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

func (es *ExecutionState) SetMasterchainHash(masterChainHash common.Hash) {
	es.MasterChain = masterChainHash
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

	// TODO: store storage of each contract in separate table
	root := NewStorageTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.StorageTrieTable))
	currencyRoot := NewCurrencyTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.CurrencyTrieTable))

	res := &AccountState{
		db:      es,
		address: addr,

		Tx:           es.tx,
		StorageTree:  root,
		CurrencyTree: currencyRoot,
		State:        map[common.Hash]common.Hash{},
	}
	es.Accounts[addr] = res
	return res, nil
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
}

func (es *ExecutionState) DropInMessage() {
	es.InMessages = es.InMessages[:len(es.InMessages)-1]
	if len(es.InMessages) > 0 {
		es.InMessageHash = es.InMessages[len(es.InMessages)-1].Hash()
	} else {
		es.InMessageHash = common.EmptyHash
	}
}

func (es *ExecutionState) AppendOutMessageForTx(txId common.Hash, msg *types.Message) {
	es.OutMessages[txId] = append(es.OutMessages[txId], msg)
}

func (es *ExecutionState) AppendOutMessage(msg *types.Message) {
	es.OutMessages[es.InMessageHash] = append(es.OutMessages[es.InMessageHash], msg)
}

func (es *ExecutionState) AddOutMessage(msg *types.Message) error {
	acc, err := es.GetAccount(msg.From)
	if err != nil {
		return err
	}
	// In case of bounce message, we don't debit currency from account
	if !msg.IsBounce() {
		for _, currency := range msg.Currency {
			balance := acc.GetCurrencyBalance(currency.Currency)
			if balance.Cmp(currency.Balance) < 0 {
				return errors.New("caller does not have enough currency")
			}
			if err := es.SubCurrency(msg.From, currency.Currency, currency.Balance); err != nil {
				return err
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
	es.AppendOutMessage(msg)
	return nil
}

var BounceGasLimit types.Gas = 100_000

func (es *ExecutionState) sendBounceMessage(msg *types.Message, bounceErr string) error {
	if msg.Value.IsZero() && len(msg.Currency) == 0 {
		return nil
	}
	if msg.BounceTo == types.EmptyAddress {
		logger.Debug().Stringer(logging.FieldMessageHash, msg.Hash()).Msg("Bounce message not sent, no bounce address")
		return nil
	}

	data, err := contracts.NewCallData(contracts.NameNilBounceable, "bounce", bounceErr)
	if err != nil {
		return err
	}

	bounceMsg := &types.InternalMessagePayload{
		Bounce:   true,
		To:       msg.BounceTo,
		Value:    msg.Value,
		Currency: msg.Currency,
		Data:     data,
		GasLimit: BounceGasLimit,
	}
	if err = vm.AddOutInternal(es, msg.To, bounceMsg); err != nil {
		return err
	}
	logger.Debug().Stringer(logging.FieldMessageFrom, msg.To).Stringer(logging.FieldMessageTo, msg.BounceTo).Msg("Bounce message sent")
	return nil
}

func (es *ExecutionState) HandleDeployMessage(_ context.Context, message *types.Message) (types.Gas, error) {
	addr := message.To
	deployMsg := types.ParseDeployPayload(message.Data)

	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Stringer(logging.FieldShardId, es.ShardId).
		Msg("Handling deploy message...")

	gas := message.GasLimit

	if err := es.newVm(message.IsInternal()); err != nil {
		return gas, err
	}
	defer es.resetVm()

	ret, addr, leftOver, err := es.evm.Deploy(addr, (vm.AccountRef)(message.From), deployMsg.Code(), gas.Uint64(), message.Value.Int())
	leftOverGas := types.Gas(leftOver)
	event := logger.Debug().Stringer(logging.FieldMessageTo, addr)
	if err != nil {
		revString := decodeRevertMessage(ret)
		if revString != "" {
			err = fmt.Errorf("%w: %s", err, revString)
		}
		event.Err(err).Msg("Contract deployment failed.")
		if message.IsInternal() {
			if bounceErr := es.sendBounceMessage(message, err.Error()); bounceErr != nil {
				return leftOverGas, bounceErr
			}
		}
	} else {
		event.Msg("Created new contract.")
	}

	return leftOverGas, err
}

func (es *ExecutionState) HandleExecutionMessage(_ context.Context, message *types.Message) (types.Gas, []byte, error) {
	addr := message.To
	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Msg("Handling execution message...")

	gas := message.GasLimit

	if err := es.newVm(message.IsInternal()); err != nil {
		return gas, nil, err
	}
	defer es.resetVm()

	if es.TraceVm {
		es.EnableVmTracing()
	}

	if message.IsExternal() {
		seqno, err := es.GetExtSeqno(addr)
		if err != nil {
			return gas, nil, err
		}
		if err := es.SetExtSeqno(addr, seqno+1); err != nil {
			return gas, nil, err
		}
	}

	es.evm.SetCurrencyTransfer(message.Currency)
	ret, leftOver, err := es.evm.Call((vm.AccountRef)(message.From), addr, message.Data, gas.Uint64(), message.Value.Int())
	leftOverGas := types.Gas(leftOver)
	if err != nil {
		revString := decodeRevertMessage(ret)
		if revString != "" {
			err = fmt.Errorf("%w: %s", err, revString)
		}
		if message.IsBounce() {
			logger.Warn().Err(err).Msg("VM returns error during bounce message processing")
		} else {
			logger.Error().Err(err).Msg("execution message failed")
			if message.IsInternal() {
				if bounceErr := es.sendBounceMessage(message, err.Error()); bounceErr != nil {
					return leftOverGas, ret, bounceErr
				}
			}
		}
	}
	return leftOverGas, ret, err
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

func (es *ExecutionState) AddReceipt(gasUsed types.Gas, err error) {
	r := &types.Receipt{
		Success:         err == nil,
		GasUsed:         gasUsed,
		MsgHash:         es.InMessageHash,
		Logs:            es.Logs[es.InMessageHash],
		ContractAddress: es.GetInMessage().To,
	}
	if err != nil {
		es.Errors[es.InMessageHash] = err
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
		treeShards := NewShardBlocksTrie(mpt.NewMerklePatriciaTrie(es.tx, es.ShardId, db.ShardBlocksTrieTableName(blockId)))
		for k, hash := range es.ChildChainBlocks {
			if err := treeShards.Update(k, types.CastToUint256(hash.Uint256())); err != nil {
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
		for _, m := range es.OutMessages[msg.Hash()] {
			if err := es.OutMessageTree.Update(outMsgIndex, m); err != nil {
				return common.EmptyHash, err
			}
			outMsgIndex++
		}
	}
	// Put all outbound messages transmitted over the topology into the trie
	for _, m := range es.OutMessages[common.EmptyHash] {
		if err := es.OutMessageTree.Update(outMsgIndex, m); err != nil {
			return common.EmptyHash, err
		}
		outMsgIndex++
	}

	// Check that each outbound message belongs to some inbound message
	for msgHash := range es.OutMessages {
		if msgHash == common.EmptyHash {
			// Skip messages transmitted over the topology
			continue
		}
		found := false
		for _, m := range es.InMessages {
			if m.Hash() == msgHash {
				found = true
				break
			}
		}
		if !found {
			return common.EmptyHash, fmt.Errorf("outbound message %v does not belong to any inbound message", msgHash)
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

	// Update receipts trie
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
		OutMessagesNum:      outMsgIndex,
		ReceiptsRoot:        es.ReceiptTree.RootHash(),
		ChildBlocksRootHash: treeShardsRootHash,
		MasterChainHash:     es.MasterChain,
		Timestamp:           es.Timer.Now(),
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

	logger.Trace().Msgf("Committed block %v on shard %v", blockId, es.ShardId)

	return blockHash, nil
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

func (es *ExecutionState) CallVerifyExternal(message *types.Message, account *AccountState, gasPrice types.Value) (bool, error) {
	methodSignature := "verifyExternal(uint256,bytes)"
	methodSelector := crypto.Keccak256([]byte(methodSignature))[:4]
	argSpec := vm.VerifySignatureArgs()[1:] // skip first arg (pubkey)
	hash, err := message.SigningHash()
	if err != nil {
		return false, err
	}
	argData, err := argSpec.Pack(hash.Big(), ([]byte)(message.Signature))
	if err != nil {
		logger.Error().Err(err).Msg("failed to pack arguments")
		return false, err
	}
	calldata := append(methodSelector, argData...) //nolint:gocritic

	if err := es.newVm(message.IsInternal()); err != nil {
		return false, err
	}
	defer es.resetVm()

	gasCreditLimit := ExternalMessageVerificationMaxGas
	gasAvailable := account.Balance.ToGas(gasPrice)

	if gasAvailable.Lt(gasCreditLimit) {
		gasCreditLimit = gasAvailable
	}

	ret, leftOverGas, err := es.evm.StaticCall((vm.AccountRef)(account.address), account.address, calldata, gasCreditLimit.Uint64())
	if err != nil || !bytes.Equal(ret, common.LeftPadBytes([]byte{1}, 32)) {
		return false, err
	}
	spentGas := gasCreditLimit.Sub(types.Gas(leftOverGas))
	spentValue := spentGas.ToValue(gasPrice)
	account.SubBalance(spentValue, tracing.BalanceDecreaseVerifyExternal)
	return true, nil
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
		return fmt.Errorf("insufficient balance %v for currency %v", balance, currencyId)
	}
	acc.SetCurrencyBalance(currencyId, balance.Sub(amount))

	return nil
}

func (es *ExecutionState) GetCurrencies(addr types.Address) map[types.CurrencyId]types.Value {
	acc, err := es.GetAccount(addr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get account")
		return nil
	}
	if acc == nil {
		return nil
	}

	res := make(map[types.CurrencyId]types.Value)
	for kv := range acc.CurrencyTree.Iterate() {
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

func (es *ExecutionState) SetCurrencyTransfer(currencies []types.CurrencyBalance) {
	es.evm.SetCurrencyTransfer(currencies)
}

func (es *ExecutionState) newVm(internal bool) error {
	blockContext, err := NewEVMBlockContext(es)
	if err != nil {
		return err
	}
	es.evm = vm.NewEVM(blockContext, es)
	es.evm.IsAsyncCall = internal
	return nil
}

func (es *ExecutionState) resetVm() {
	es.evm = nil
}
