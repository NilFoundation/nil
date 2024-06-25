package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

var logger = logging.NewLogger("execution")

const TraceBlocksEnabled = false

var ExternalMessageVerificationMaxGas = uint256.NewInt(100_000)

// TODO: Make gas price dynamic and use message.GasPrice
var (
	GasPrice   = uint256.NewInt(10)
	ForwardFee = uint256.NewInt(100)
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
		Balance:      account.Balance.Int,
		CurrencyTree: currencyRoot,
		StorageTree:  root,
		CodeHash:     account.CodeHash,
		Code:         code,
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
func (es *ExecutionState) AddBalance(addr types.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) error {
	stateObject, err := es.getOrNewAccount(addr)
	if err != nil || stateObject == nil {
		return err
	}
	stateObject.AddBalance(amount, reason)
	return nil
}

// SubBalance subtracts amount from the account associated with addr.
func (es *ExecutionState) SubBalance(addr types.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) error {
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
	var (
		prev = new(uint256.Int).Set(&stateObject.Balance)
		n    = new(uint256.Int)
	)
	es.journal.append(selfDestructChange{
		account:     &stateObject.address,
		prev:        stateObject.selfDestructed,
		prevbalance: prev,
	})
	stateObject.selfDestructed = true
	stateObject.Balance = *n
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
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.setSeqno(message.Seqno)

	if err := es.newVm(message.Internal); err != nil {
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

func (es *ExecutionState) GetBalance(addr types.Address) (*uint256.Int, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return uint256.NewInt(0), err
	}
	return &acc.Balance, nil
}

func (es *ExecutionState) GetSeqno(addr types.Address) (types.Seqno, error) {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return 0, err
	}
	return acc.Seqno, nil
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

func (es *ExecutionState) SetBalance(addr types.Address, balance uint256.Int) error {
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
	check.PanicIfNotf(addr.ShardId() == es.ShardId, "Attempt to create account %v from %v shard on %v shard", addr, addr.ShardId(), es.ShardId)
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

func (es *ExecutionState) AddOutMessageForTx(txId common.Hash, msg *types.Message) {
	es.OutMessages[txId] = append(es.OutMessages[txId], msg)
}

func (es *ExecutionState) AddOutMessage(msg *types.Message) error {
	acc, err := es.GetAccount(msg.From)
	if err != nil {
		return err
	}
	for _, currency := range msg.Currency {
		balance := acc.GetCurrencyBalance(&currency.Currency)
		if balance.Lt(&currency.Balance.Int) {
			return errors.New("caller does not have enough currency")
		}
		if err := es.SubCurrency(msg.From, &currency.Currency, &currency.Balance.Int); err != nil {
			return err
		}
	}
	es.AddOutMessageForTx(es.InMessageHash, msg)
	return nil
}

func createBounceMethod() abi.Method {
	stringType, err := abi.NewType("string", "", nil)
	check.PanicIfErr(err)
	// Parse the method signature: function bounce(string err) external payable returns ()
	bounceMethod := abi.NewMethod("bounce", "bounce", abi.Function, "external payable", false, true, abi.Arguments{abi.Argument{Name: "err", Type: stringType}}, nil)
	return bounceMethod
}

var bounceMethod = createBounceMethod()

func packBounceCall(bounceErr string) ([]byte, error) {
	// Pack the method call
	data, err := bounceMethod.Inputs.Pack(bounceErr)
	if err != nil {
		return nil, fmt.Errorf("failed to pack method call: %w", err)
	}

	// Combine the method ID and the packed arguments
	methodID := bounceMethod.ID
	return append(methodID, data...), nil
}

var BounceGasLimit = types.NewUint256(100_000)

func (es *ExecutionState) sendBounceMessage(msg *types.Message, bounceErr string) error {
	if msg.BounceTo == types.EmptyAddress {
		logger.Debug().Stringer(logging.FieldMessageHash, msg.Hash()).Msg("Bounce message not sent, no bounce address")
		return nil
	}
	data, err := packBounceCall(bounceErr)
	if err != nil {
		return err
	}
	bounceMsg := &types.InternalMessagePayload{
		To:       msg.BounceTo,
		Value:    msg.Value,
		Data:     data,
		GasLimit: *BounceGasLimit,
	}
	if err = vm.AddOutInternal(es, msg.To, bounceMsg); err != nil {
		return err
	}
	logger.Debug().Stringer(logging.FieldMessageFrom, msg.To).Stringer(logging.FieldMessageTo, msg.BounceTo).Msg("Bounce message sent")
	return nil
}

func (es *ExecutionState) HandleDeployMessage(
	_ context.Context, message *types.Message,
) (uint64, error) {
	addr := message.To
	deployMsg := types.ParseDeployPayload(message.Data)

	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Stringer(logging.FieldShardId, es.ShardId).
		Msg("Handling deploy message...")

	gas := message.GasLimit.Uint64()

	if err := es.newVm(message.Internal); err != nil {
		return gas, err
	}
	defer es.resetVm()

	_, addr, leftOverGas, err := es.evm.Deploy(addr, (vm.AccountRef)(message.From), deployMsg.Code(), gas, &message.Value.Int)
	event := logger.Debug().Stringer(logging.FieldMessageTo, addr)
	if err != nil {
		event.Err(err).Msg("Contract deployment failed.")
		if message.Internal {
			if bounceErr := es.sendBounceMessage(message, err.Error()); bounceErr != nil {
				return leftOverGas, bounceErr
			}
		}
	} else {
		event.Msg("Created new contract.")
	}

	return leftOverGas, err
}

func (es *ExecutionState) HandleExecutionMessage(_ context.Context, message *types.Message) (uint64, []byte, error) {
	addr := message.To
	logger.Debug().
		Stringer(logging.FieldMessageTo, addr).
		Msg("Handling execution message...")

	gas := message.GasLimit.Uint64()

	if err := es.newVm(message.Internal); err != nil {
		return gas, nil, err
	}
	defer es.resetVm()

	if es.TraceVm {
		es.EnableVmTracing(es.evm)
	}

	if !message.Internal {
		seqno, err := es.GetSeqno(addr)
		if err != nil {
			return gas, nil, err
		}
		if err := es.SetSeqno(addr, seqno+1); err != nil {
			return gas, nil, err
		}
	}

	ret, leftOverGas, err := es.evm.Call((vm.AccountRef)(message.From), addr, message.Data, gas, &message.Value.Int)
	if err != nil {
		logger.Error().Err(err).Msg("execution message failed")
		if message.Internal {
			if bounceErr := es.sendBounceMessage(message, err.Error()); bounceErr != nil {
				return leftOverGas, ret, bounceErr
			}
		}
	}
	return leftOverGas, ret, err
}

func (es *ExecutionState) HandleRefundMessage(_ context.Context, message *types.Message) error {
	err := es.AddBalance(message.To, &message.Value.Int, tracing.BalanceIncreaseRefund)
	logger.Debug().Err(err).Msgf("Refunded %v to %v", &message.Value.Int, message.To)
	return err
}

func (es *ExecutionState) AddReceipt(gasUsed uint32, err error) {
	es.Receipts = append(es.Receipts, &types.Receipt{
		Success:         err == nil,
		GasUsed:         gasUsed,
		MsgHash:         es.InMessageHash,
		Logs:            es.Logs[es.InMessageHash],
		ContractAddress: es.GetInMessage().To,
	})
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
			if err := treeShards.Update(k, &types.Uint256{Int: *hash.Uint256()}); err != nil {
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

	logger.Debug().Msgf("Committed block %v on shard %v", blockId, es.ShardId)

	return blockHash, nil
}

func (es *ExecutionState) IsInternalMessage() bool {
	// If contract calls another contract using EVM's call(depth > 1), we treat it as an internal message.
	return es.GetInMessage().Internal || es.evm.GetDepth() > 1
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

func (es *ExecutionState) CallVerifyExternal(message *types.Message, account *AccountState) (bool, error) {
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

	if err := es.newVm(message.Internal); err != nil {
		return false, err
	}
	defer es.resetVm()

	gasCreditLimit := ExternalMessageVerificationMaxGas
	gasAvailable := new(uint256.Int).Div(&account.Balance, GasPrice)

	if gasAvailable.Cmp(gasCreditLimit) < 0 {
		gasCreditLimit = gasAvailable
	}

	ret, leftOverGas, err := es.evm.StaticCall((vm.AccountRef)(account.address), account.address, calldata, gasCreditLimit.Uint64())
	if err != nil || !bytes.Equal(ret, common.LeftPadBytes([]byte{1}, 32)) {
		return false, err
	}
	spentGas := new(uint256.Int).Sub(gasCreditLimit, uint256.NewInt(leftOverGas))
	spentValue := new(uint256.Int).Mul(spentGas, GasPrice)
	account.SubBalance(spentValue, tracing.BalanceDecreaseVerifyExternal)
	return true, nil
}

func (es *ExecutionState) AddCurrency(addr types.Address, currencyId *types.CurrencyId, amount *uint256.Int) error {
	logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", common.Hash(*currencyId)).
		Msg("Add currency")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetCurrencyBalance(currencyId)
	newBalance := new(uint256.Int).Add(balance, amount)
	acc.SetCurrencyBalance(currencyId, newBalance)

	return nil
}

func (es *ExecutionState) SubCurrency(addr types.Address, currencyId *types.CurrencyId, amount *uint256.Int) error {
	logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", common.Hash(*currencyId)).
		Msg("Sub currency")

	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		return fmt.Errorf("destination account %v not found", addr)
	}

	balance := acc.GetCurrencyBalance(currencyId)
	newBalance := new(uint256.Int).Sub(balance, amount)
	acc.SetCurrencyBalance(currencyId, newBalance)

	return nil
}

func (es *ExecutionState) GetCurrencies(addr types.Address) []*types.CurrencyBalance {
	acc, err := es.GetAccount(addr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get account")
		return nil
	}
	if acc == nil {
		return nil
	}

	res := make([]*types.CurrencyBalance, 0)
	for kv := range acc.CurrencyTree.Iterate() {
		var c types.CurrencyBalance
		c.Currency = types.CurrencyId(kv.Key)
		if err := c.Balance.UnmarshalSSZ(kv.Value); err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal currency balance")
			continue
		}
		res = append(res, &c)
	}

	return res
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
