package tracer

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

type ExecutionTraces struct {
	// Stack/Memory/State Ops are handled for entire block, they share the same counter (rw_circuit)
	StackOps    []StackOp
	MemoryOps   []MemoryOp
	StorageOps  []StorageOp
	ZKEVMStates []ZKEVMState
	CopyEvents  []CopyEvent

	ContractsBytecode map[types.Address][]byte
	// For each message, for each touched contract
	SlotsChanges []map[types.Address][]SlotChangeTrace
}

type Stats struct {
	ProcessedInMsgsN uint
	OpsN             uint // should be the same as StackOpsN, since every op is a stack op
	StackOpsN        uint
	MemoryOpsN       uint
	StateOpsN        uint
	CopyOpsN         uint
}

type TracerStateDB struct {
	client           *rpc.Client
	rwTx             db.RwTx
	shardId          types.ShardId
	shardBlockNumber types.BlockNumber
	accountsCache    map[types.Address]*Account
	InMessages       []*types.Message
	blkContext       *vm.BlockContext
	Traces           ExecutionTraces
	RwCounter        RwCounter
	Stats            Stats

	// gas price for current block
	GasPrice types.Value

	// Reinited for each message
	// Pointer to currently executed VM
	evm               *vm.EVM
	code              []byte // currently executed code
	codeHash          common.Hash
	stackTracer       *StackOpTracer
	memoryTracer      *MemoryOpTracer
	storageInteractor *MessageStorageInteractor
	zkevmTracer       *ZKEVMStateTracer
	copyTracer        *CopyTracer

	// Current program counter, used only for storage operations trace. Incremetned inside OnOpcode
	curPC uint64
}

var _ vm.StateDB = new(TracerStateDB)

// TODO: refactor, move Account into other file
type Account struct {
	Address     types.Address
	StorageTrie *execution.StorageTrie
	Balance     types.Value
	Code        types.Code
	Seqno       types.Seqno
	ExtSeqno    types.Seqno
}

func NewTracerStateDB(ctx context.Context, client *rpc.Client, shardId types.ShardId, shardBlockNumber types.BlockNumber, blkContext *vm.BlockContext, db db.DB) (TracerStateDB, error) {
	rwTx, err := db.CreateRwTx(ctx)
	if err != nil {
		return TracerStateDB{}, err
	}

	return TracerStateDB{
		client:           client,
		rwTx:             rwTx,
		shardId:          shardId,
		shardBlockNumber: shardBlockNumber,
		accountsCache:    make(map[types.Address]*Account),
		blkContext:       blkContext,
		Traces: ExecutionTraces{
			ContractsBytecode: make(map[types.Address][]byte),
		},
	}, nil
}

func (tsdb *TracerStateDB) getAccount(addr types.Address) (*Account, error) {
	smartContract, exists := tsdb.accountsCache[addr]
	if exists {
		return smartContract, nil
	}

	// Since we don't always need entire contract storage, we could add StorageRoot to RPC response and
	// fetch nodes on demand. For now whole storage and code is included into debug contract.
	debugContract, err := tsdb.client.GetDebugContract(addr, transport.BlockNumber(tsdb.shardBlockNumber))
	if err != nil {
		return nil, err
	}

	if debugContract == nil {
		// No need to fetch the absent account next time
		tsdb.accountsCache[addr] = nil
		return nil, nil
	}

	storageTrie := execution.NewDbStorageTrie(tsdb.rwTx, tsdb.shardId)
	storageKeys := make([]common.Hash, len(debugContract.StorageEntries))
	storageValues := make([]*types.Uint256, len(debugContract.StorageEntries))
	for key, val := range debugContract.StorageEntries {
		storageKeys = append(storageKeys, key)
		storageValues = append(storageValues, &val)
	}
	err = storageTrie.UpdateBatch(storageKeys, storageValues)
	if err != nil {
		return nil, err
	}

	// Currencies will be fetched on demand
	smartContract = &Account{
		Address:     addr,
		StorageTrie: storageTrie,
		Balance:     debugContract.Contract.Balance,
		Code:        debugContract.Code,
		Seqno:       debugContract.Contract.Seqno,
		ExtSeqno:    debugContract.Contract.ExtSeqno,
	}

	tsdb.accountsCache[addr] = smartContract

	return smartContract, nil
}

func (tsdb *TracerStateDB) getOrNewAccount(addr types.Address) (*Account, error) {
	acc, err := tsdb.getAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return acc, nil
	}

	return tsdb.createAccount(addr)
}

func (tsdb *TracerStateDB) createAccount(addr types.Address) (*Account, error) {
	// FIXME: it will probably fail now, since trie is not initialized
	acc := &Account{
		Address: addr,
	}

	tsdb.accountsCache[addr] = acc
	return acc, nil
}

// OutMessages don't requre handling, they are just included into block
func (tsdb *TracerStateDB) HandleInMessage(message *types.Message) error {
	var err error
	switch {
	case message.IsRefund():
		err = tsdb.HandleRefundMessage(message)
	case message.IsDeploy():
		err = tsdb.HandleDeployMessage(message)
	case message.IsExecution():
		err = tsdb.HandleExecutionMessage(message)
	default:
		panic(fmt.Sprintf("unknown message type: %+v", message))
	}
	tsdb.Stats.ProcessedInMsgsN++
	return err
}

func (tsdb *TracerStateDB) processOpcodeWithTracers(
	pc uint64,
	op byte,
	gas uint64,
	_ uint64,
	scope tracing.OpContext,
	returnData []byte,
	_ int,
	err error,
) {
	if err != nil {
		panic("prover execution should not raise errors")
	}

	opCode := vm.OpCode(op)
	tsdb.Stats.OpsN++

	// Finish in reverse order to keep rw_counter sequential.
	// Each operation consists of read stack -> read memory -> write memory -> write stack (we
	// ignore specific memory parts like returndata, etc for now). Stages could be omitted, but
	// not reordered.
	tsdb.memoryTracer.FinishPrevOpcodeTracing()
	tsdb.stackTracer.FinishPrevOpcodeTracing()

	ranges, hasMemOps := tsdb.memoryTracer.GetUsedMemoryRanges(opCode, scope)

	// Store zkevmState before counting rw operations
	numRequiredStackItems := tsdb.evm.Interpreter().GetNumRequiredStackItems(opCode)
	additionalInput := types.NewUint256(0) // data for pushX opcodes
	if len(tsdb.code) != 0 && opCode.IsPush() {
		bytesToPush := uint64(opCode) - uint64(vm.PUSH0)
		if bytesToPush > 0 {
			additionalInput = types.NewUint256FromBytes(tsdb.code[pc+1 : pc+bytesToPush+1])
		}
	}
	tsdb.zkevmTracer.TraceOp(opCode, pc, gas, numRequiredStackItems, additionalInput, ranges, scope)

	// Stack tracing is splitted between current opcode (before change read operations)
	// and the next opcode (after change write operations)
	tsdb.stackTracer.TraceOp(opCode, pc, scope)
	tsdb.Stats.StackOpsN++

	// Memory tracing is hanled in one go. Mb split into two as for stack
	if hasMemOps {
		copyOccured := tsdb.copyTracer.TraceOp(opCode, tsdb.RwCounter.ctr, scope, returnData)
		if copyOccured {
			tsdb.Stats.CopyOpsN++
		}

		tsdb.memoryTracer.TraceOp(opCode, pc, ranges, scope)
		tsdb.Stats.MemoryOpsN++
	}

	// Storage tracing is done inside Get/SetState methods
	if opCode == vm.SLOAD || opCode == vm.SSTORE {
		tsdb.Stats.StateOpsN++
	}
}

func (tsdb *TracerStateDB) initVm(
	internal bool,
	origin types.Address,
	executingCode types.Code,
	state *vm.EvmRestoreData,
) {
	tsdb.evm = vm.NewEVM(tsdb.blkContext, tsdb, origin, state)
	tsdb.evm.IsAsyncCall = internal

	tsdb.code = executingCode
	tsdb.codeHash = executingCode.Hash()

	msgId := uint(len(tsdb.InMessages) - 1)
	tsdb.stackTracer = &StackOpTracer{rwCtr: &tsdb.RwCounter, msgId: msgId}
	tsdb.memoryTracer = &MemoryOpTracer{rwCtr: &tsdb.RwCounter, msgId: msgId}
	tsdb.storageInteractor = NewMessageStorageInteractor(&tsdb.RwCounter, tsdb.GetCurPC, msgId)
	tsdb.zkevmTracer = &ZKEVMStateTracer{
		rwCtr:        &tsdb.RwCounter,
		msgId:        msgId,
		txHash:       tsdb.GetInMessage().Hash(),
		bytecodeHash: tsdb.codeHash,
	}
	tsdb.copyTracer = &CopyTracer{codeProvider: tsdb, msgId: msgId}

	tsdb.evm.Config.Tracer = &tracing.Hooks{
		OnOpcode: func(pc uint64, op byte, gas uint64, cost uint64, scope tracing.OpContext, rData []byte, depth int, err error) {
			verifyIntegrity := assertEVMStateConsistent(pc, scope, rData)
			defer verifyIntegrity()

			tsdb.curPC = pc
			tsdb.processOpcodeWithTracers(pc, op, gas, cost, scope, rData, depth, err)
		},
	}
}

// Tracers finalizations happen here
func (tsdb *TracerStateDB) saveMessageTraces() error {
	tsdb.Traces.MemoryOps = append(tsdb.Traces.MemoryOps, tsdb.memoryTracer.Finalize()...)
	tsdb.Traces.StackOps = append(tsdb.Traces.StackOps, tsdb.stackTracer.Finalize()...)
	tsdb.Traces.ZKEVMStates = append(tsdb.Traces.ZKEVMStates, tsdb.zkevmTracer.Finalize()...)
	tsdb.Traces.StorageOps = append(tsdb.Traces.StorageOps, tsdb.storageInteractor.GetStorageOps()...)
	tsdb.Traces.CopyEvents = append(tsdb.Traces.CopyEvents, tsdb.copyTracer.Finalize()...)

	curMessageSlotChanges := make(map[types.Address][]SlotChangeTrace)
	for _, addr := range tsdb.storageInteractor.GetAffectedAccountsAddresses() {
		acc, err := tsdb.getAccount(addr)
		if err != nil {
			return err
		}

		addrSlotChanges, err := tsdb.storageInteractor.GetAccountSlotChangeTraces(acc)
		if err != nil {
			return err
		}
		curMessageSlotChanges[addr] = addrSlotChanges
	}
	tsdb.Traces.SlotsChanges = append(tsdb.Traces.SlotsChanges, curMessageSlotChanges)

	return nil
}

func (tsdb *TracerStateDB) resetVm() {
	tsdb.stackTracer = nil
	tsdb.memoryTracer = nil
	tsdb.zkevmTracer = nil
	tsdb.evm = nil
	tsdb.code = nil
}

func (tsdb *TracerStateDB) HandleRefundMessage(message *types.Message) error {
	return tsdb.AddBalance(message.To, message.Value, tracing.BalanceIncreaseRefund)
}

func (tsdb *TracerStateDB) HandleExecutionMessage(message *types.Message) error {
	check.PanicIfNotf(!message.IsResponse(), "Can't handle response yet")
	caller := (vm.AccountRef)(message.From)
	callData := message.Data

	code, _, err := tsdb.GetCode(message.To)
	check.PanicIfErr(err)

	tsdb.initVm(message.IsInternal(), message.From, code, nil)
	defer tsdb.resetVm()

	tsdb.evm.SetCurrencyTransfer(message.Currency)
	gas := message.FeeCredit.ToGas(tsdb.GasPrice) // mb previous block, not current one?
	ret, gasLeft, err := tsdb.evm.Call(caller, message.To, callData, gas.Uint64(), message.Value.Int())
	_, _ = ret, gasLeft
	if err != nil {
		panic(fmt.Sprintf("EVM call returned error: %v", err))
	}
	return tsdb.saveMessageTraces()
}

func (tsdb *TracerStateDB) HandleDeployMessage(message *types.Message) error {
	addr := message.To
	deployMsg := types.ParseDeployPayload(message.Data)

	tsdb.initVm(message.IsInternal(), message.From, deployMsg.Code(), nil)
	defer tsdb.resetVm()

	gas := message.FeeCredit.ToGas(tsdb.GasPrice)
	ret, addr, leftOver, err := tsdb.evm.Deploy(addr, (vm.AccountRef)(message.From), deployMsg.Code(), gas.Uint64(), message.Value.Int())
	if err != nil {
		panic("deploy must not throw")
	}
	// `_, _, _, err` doesn't satisfy linter
	_ = ret
	_ = addr
	_ = leftOver

	return tsdb.saveMessageTraces()
}

// The only way to add InMessage to state
func (tsdb *TracerStateDB) AddInMessage(message *types.Message) {
	// We store a copy of the message, because the original message will be modified.
	tsdb.InMessages = append(tsdb.InMessages, common.CopyPtr(message))
}

// Read-only methods
func (tsdb *TracerStateDB) IsInternalMessage() bool {
	// If contract calls another contract using EVM's call(depth > 1), we treat it as an internal message.
	return tsdb.GetInMessage().IsInternal() || tsdb.evm.GetDepth() > 1
}

func (tsdb *TracerStateDB) GetMessageFlags() types.MessageFlags {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetCurrencies(types.Address) map[types.CurrencyId]types.Value {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetGasPrice(types.ShardId) (types.Value, error) {
	panic("not implemented")
}

// Write methods
func (tsdb *TracerStateDB) CreateAccount(addr types.Address) error {
	_, err := tsdb.createAccount(addr)
	return err
}

func (tsdb *TracerStateDB) CreateContract(addr types.Address) error {
	_, err := tsdb.getAccount(addr)
	// obj.newContract = true
	return err
}

// SubBalance subtracts amount from the account associated with addr.
func (tsdb *TracerStateDB) SubBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	acc, err := tsdb.getOrNewAccount(addr)
	if err != nil { // in state.go there is also `|| acc == nil`, but seems redundant (acc is always non-nil)
		return err
	}
	acc.Balance.Sub(amount)
	return nil
}

// AddBalance adds amount to the account associated with addr.
func (tsdb *TracerStateDB) AddBalance(addr types.Address, amount types.Value, reason tracing.BalanceChangeReason) error {
	acc, err := tsdb.getOrNewAccount(addr)
	if err != nil { // in state.go there is also `|| acc == nil`, but seems redundant (acc is always non-nil)
		return err
	}
	acc.Balance.Add(amount)
	return nil
}

func (tsdb *TracerStateDB) GetBalance(addr types.Address) (types.Value, error) {
	acc, err := tsdb.getAccount(addr)
	if err != nil || acc == nil {
		return types.Value{}, err
	}
	return acc.Balance, nil
}

func (tsdb *TracerStateDB) AddCurrency(to types.Address, currencyId types.CurrencyId, amount types.Value) error {
	panic("not implemented")
}

func (tsdb *TracerStateDB) SubCurrency(to types.Address, currencyId types.CurrencyId, amount types.Value) error {
	panic("not implemented")
}

func (tsdb *TracerStateDB) SetCurrencyTransfer([]types.CurrencyBalance) {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetSeqno(addr types.Address) (types.Seqno, error) {
	acc, err := tsdb.getAccount(addr)
	if err != nil || acc == nil {
		return 0, err
	}
	return acc.ExtSeqno, nil
}

func (tsdb *TracerStateDB) SetSeqno(addr types.Address, seqno types.Seqno) error {
	acc, err := tsdb.getOrNewAccount(addr)
	if err != nil {
		return err
	}
	acc.Seqno = seqno
	return nil
}

func (tsdb *TracerStateDB) GetCurrentCode() ([]byte, common.Hash, error) {
	if len(tsdb.code) == 0 {
		return []byte{}, common.EmptyHash, errors.New("no code is currently executed")
	}
	return tsdb.code, tsdb.codeHash, nil
}

func (tsdb *TracerStateDB) GetCode(addr types.Address) ([]byte, common.Hash, error) {
	acc, err := tsdb.getAccount(addr)
	if err != nil || acc == nil {
		return []byte{}, common.EmptyHash, err
	}

	// if contract code was requested, we dump it into traces
	_, exists := tsdb.Traces.ContractsBytecode[addr]
	if !exists {
		tsdb.Traces.ContractsBytecode[addr] = acc.Code
	}

	return acc.Code, acc.Code.Hash(), nil
}

func (tsdb *TracerStateDB) SetCode(addr types.Address, code []byte) error {
	acc, err := tsdb.getAccount(addr)
	if err != nil {
		return err
	}
	acc.Code = code
	tsdb.code = acc.Code
	tsdb.codeHash = acc.Code.Hash()
	return nil
}

func (tsdb *TracerStateDB) AddRefund(uint64) {
	panic("not implemented")
}

func (tsdb *TracerStateDB) SubRefund(uint64) {
	panic("not implemented")
}

// GetRefund returns the current value of the refund counter.
func (tsdb *TracerStateDB) GetRefund() uint64 {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetCommittedState(types.Address, common.Hash) common.Hash {
	return common.EmptyHash
}

func (tsdb *TracerStateDB) GetState(addr types.Address, key common.Hash) (common.Hash, error) {
	acc, err := tsdb.getAccount(addr)
	if err != nil || acc == nil {
		return common.EmptyHash, err
	}
	val, err := tsdb.storageInteractor.GetSlot(acc, key)
	// Pass slot data to zkevm_state
	tsdb.zkevmTracer.SetLastStateStorage((types.Uint256)(*key.Uint256()), (types.Uint256)(*val.Uint256()))
	return val, err
}

// proof, err := mpt.BuildProof(acc.StorageTrie.Reader, key.Bytes(), mpt.ReadMPTOperation)
// if err != nil {
// 	return err
// }
// tsdb.addStorageProof(addr, proof)

func (tsdb *TracerStateDB) SetState(addr types.Address, key common.Hash, val common.Hash) error {
	acc, err := tsdb.getOrNewAccount(addr)
	if err != nil {
		return err
	}

	prev, err := tsdb.storageInteractor.SetSlot(acc, key, val)
	// Pass slote data before setting to zkevm_state
	tsdb.zkevmTracer.SetLastStateStorage((types.Uint256)(*key.Uint256()), prev)
	return err
}

func (tsdb *TracerStateDB) GetStorageRoot(addr types.Address) (common.Hash, error) {
	acc, err := tsdb.getAccount(addr)
	if err != nil || acc == nil {
		return common.Hash{}, err
	}
	return acc.StorageTrie.RootHash(), nil
}

func (tsdb *TracerStateDB) GetTransientState(addr types.Address, key common.Hash) common.Hash {
	panic("not implemented")
}

func (tsdb *TracerStateDB) SetTransientState(addr types.Address, key, value common.Hash) {
	panic("not implemented")
}

func (tsdb *TracerStateDB) HasSelfDestructed(types.Address) (bool, error) {
	panic("not implemented")
}

func (tsdb *TracerStateDB) Selfdestruct6780(types.Address) error {
	panic("not implemented")
}

// Exist reports whether the given account exists in state.
// Notably this should also return true for self-destructed accounts.
func (tsdb *TracerStateDB) Exists(address types.Address) (bool, error) {
	account, err := tsdb.getAccount(address)
	if err != nil {
		return false, err
	}
	return account != nil, nil
}

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (tsdb *TracerStateDB) Empty(addr types.Address) (bool, error) {
	acc, err := tsdb.getAccount(addr)
	return acc == nil || (acc.Balance.IsZero() && len(acc.Code) == 0 && acc.Seqno == 0), err
}

// ContractExists is used to check whether we can deploy to an address.
// Contract is regarded as existent if any of these three conditions is met:
// - the nonce is non-zero
// - the code is non-empty
// - the storage is non-empty
func (tsdb *TracerStateDB) ContractExists(addr types.Address) (bool, error) {
	_, contractHash, err := tsdb.GetCode(addr)
	if err != nil {
		return false, err
	}
	storageRoot, err := tsdb.GetStorageRoot(addr)
	if err != nil {
		return false, err
	}
	seqno, err := tsdb.GetSeqno(addr)
	if err != nil {
		return false, err
	}
	return seqno != 0 ||
		(contractHash != common.EmptyHash) || // non-empty code
		(storageRoot != common.EmptyHash), nil // non-empty storage
}

func (tsdb *TracerStateDB) AddressInAccessList(addr types.Address) bool {
	return true // FIXME: not implemented in state.go neither
}

func (tsdb *TracerStateDB) SlotInAccessList(addr types.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return true, true // FIXME: not implemented in state.go neither
}

// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (tsdb *TracerStateDB) AddAddressToAccessList(addr types.Address) {
	panic("not implemented")
}

// AddSlotToAccessList adds the given (address, slot) to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (tsdb *TracerStateDB) AddSlotToAccessList(addr types.Address, slot common.Hash) {
	panic("not implemented")
}

func (tsdb *TracerStateDB) RevertToSnapshot(int) {
	panic("proofprovider execution should not revert")
}

// Snapshot returns an identifier for the current revision of the state.
func (tsdb *TracerStateDB) Snapshot() int {
	// Snapshot is needed for rollback when an error was returned by the EVM.
	// We could just ignore failing transactions in proof provider. In case revert occures, we fail in RevertToSnapshot(int)
	return 0
}

func (tsdb *TracerStateDB) AddLog(*types.Log) {
}

// AddOutMessage adds internal out message for current transaction
func (tsdb *TracerStateDB) AddOutMessage(caller types.Address, payload *types.InternalMessagePayload) (*types.Message, error) {
	// TODO: seems useless now, implement when final hash calculation is needed
	return nil, nil
}

// AddOutRequestMessage adds outbound request message for current transaction
func (tsdb *TracerStateDB) AddOutRequestMessage(
	caller types.Address,
	payload *types.InternalMessagePayload,
	responseProcessingGas types.Gas,
	isAwait bool,
) (*types.Message, error) {
	panic("not implemented")
}

// Get current message
func (tsdb *TracerStateDB) GetInMessage() *types.Message {
	if len(tsdb.InMessages) == 0 {
		return nil
	}
	return tsdb.InMessages[len(tsdb.InMessages)-1]
}

// Get execution context shard id
func (tsdb *TracerStateDB) GetShardID() types.ShardId {
	panic("not implemented")
}

// SaveVmState saves current VM state
func (tsdb *TracerStateDB) SaveVmState(state *types.EvmState, continuationGasCredit types.Gas) error {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetConfigAccessor() *config.ConfigAccessor {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetCurPC() uint64 {
	// Used by storage tracer
	return tsdb.curPC
}
