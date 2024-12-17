package tracer

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/mpttracer"
)

type ExecutionTraces interface {
	AddMemoryOps(ops []MemoryOp)
	AddStackOps(ops []StackOp)
	AddStorageOps(ops []StorageOp)
	AddExpOps(ops []ExpOp)
	AddZKEVMStates(states []ZKEVMState)
	AddCopyEvents(events []CopyEvent)
	AddContractBytecode(addr types.Address, code []byte)
	SetMptTraces(mptTraces *mpttracer.MPTTraces)
}

type executionTracesImpl struct {
	// Stack/Memory/State Ops are handled for entire block, they share the same counter (rw_circuit)
	StackOps    []StackOp
	MemoryOps   []MemoryOp
	StorageOps  []StorageOp
	ExpOps      []ExpOp
	ZKEVMStates []ZKEVMState
	CopyEvents  []CopyEvent
	MPTTraces   *mpttracer.MPTTraces

	ContractsBytecode map[types.Address][]byte
}

var _ ExecutionTraces = new(executionTracesImpl)

func NewExecutionTraces() ExecutionTraces {
	return &executionTracesImpl{
		ContractsBytecode: make(map[types.Address][]byte),
	}
}

func (tr *executionTracesImpl) AddMemoryOps(ops []MemoryOp) {
	tr.MemoryOps = append(tr.MemoryOps, ops...)
}

func (tr *executionTracesImpl) AddStackOps(ops []StackOp) {
	tr.StackOps = append(tr.StackOps, ops...)
}

func (tr *executionTracesImpl) AddStorageOps(ops []StorageOp) {
	tr.StorageOps = append(tr.StorageOps, ops...)
}

func (tr *executionTracesImpl) AddExpOps(ops []ExpOp) {
	tr.ExpOps = append(tr.ExpOps, ops...)
}

func (tr *executionTracesImpl) AddZKEVMStates(states []ZKEVMState) {
	tr.ZKEVMStates = append(tr.ZKEVMStates, states...)
}

func (tr *executionTracesImpl) AddCopyEvents(events []CopyEvent) {
	tr.CopyEvents = append(tr.CopyEvents, events...)
}

func (tr *executionTracesImpl) AddContractBytecode(addr types.Address, code []byte) {
	tr.ContractsBytecode[addr] = code
}

func (tr *executionTracesImpl) SetMptTraces(mptTraces *mpttracer.MPTTraces) {
	tr.MPTTraces = mptTraces
}

type Stats struct {
	ProcessedInMsgsN   uint
	OpsN               uint // should be the same as StackOpsN, since every op is a stack op
	StackOpsN          uint
	MemoryOpsN         uint
	StateOpsN          uint
	CopyOpsN           uint
	ExpOpsN            uint
	AffectedContractsN uint
}

type TracerStateDB struct {
	client           client.Client
	shardId          types.ShardId
	shardBlockNumber types.BlockNumber
	InMessages       []*types.Message
	blkContext       *vm.BlockContext
	Traces           ExecutionTraces
	RwCounter        RwCounter
	Stats            Stats
	AccountSparseMpt mpt.MerklePatriciaTrie

	// gas price for current block
	GasPrice types.Value

	// Reinited for each message
	// Pointer to currently executed VM
	evm           *vm.EVM
	code          []byte // currently executed code
	codeHash      common.Hash
	stackTracer   *StackOpTracer
	memoryTracer  *MemoryOpTracer
	storageTracer *StorageOpTracer
	expTracer     *ExpOpTracer
	mptTracer     *mpttracer.MPTTracer
	zkevmTracer   *ZKEVMStateTracer
	copyTracer    *CopyTracer

	// Current program counter, used only for storage operations trace. Incremetned inside OnOpcode
	curPC uint64
}

var _ vm.StateDB = new(TracerStateDB)

func NewTracerStateDB(
	ctx context.Context,
	aggTraces ExecutionTraces,
	client client.Client,
	shardId types.ShardId,
	shardBlockNumber types.BlockNumber,
	blkContext *vm.BlockContext,
	db db.DB,
) (TracerStateDB, error) {
	rwTx, err := db.CreateRwTx(ctx)
	if err != nil {
		return TracerStateDB{}, err
	}

	return TracerStateDB{
		client:           client,
		mptTracer:        mpttracer.New(client, shardBlockNumber, rwTx, shardId),
		shardId:          shardId,
		shardBlockNumber: shardBlockNumber,
		blkContext:       blkContext,
		Traces:           aggTraces,
	}, nil
}

func (tsdb *TracerStateDB) getOrNewAccount(addr types.Address) (*execution.AccountState, error) {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return &acc.AccountState, nil
	}

	createdAcc, err := tsdb.mptTracer.CreateAccount(addr)
	if err != nil {
		return nil, err
	}

	return &createdAcc.AccountState, nil
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
		panic(fmt.Errorf("prover execution should not raise errors %w", err))
	}

	opCode := vm.OpCode(op)
	tsdb.Stats.OpsN++

	// Finish memory and stack tracers in reverse order to keep rw_counter sequential.
	// Each operation consists of read stack -> read memory -> write memory -> write stack (we
	// ignore specific memory parts like returndata, etc for now). Stages could be omitted, but
	// not reordered.
	tsdb.memoryTracer.FinishPrevOpcodeTracing()
	tsdb.stackTracer.FinishPrevOpcodeTracing()
	tsdb.copyTracer.FinishPrevOpcodeTracing()

	tsdb.expTracer.FinishPrevOpcodeTracing()

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

	if hasMemOps {
		copyOccured := tsdb.copyTracer.TraceOp(opCode, tsdb.RwCounter.ctr, scope, returnData)
		if copyOccured {
			tsdb.Stats.CopyOpsN++
		}

		tsdb.memoryTracer.TraceOp(opCode, pc, ranges, scope)
		tsdb.Stats.MemoryOpsN++
	}

	if tsdb.expTracer.TraceOp(opCode, pc, scope) {
		tsdb.Stats.ExpOpsN++
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
	tsdb.storageTracer = NewStorageOpTracer(tsdb.mptTracer, &tsdb.RwCounter, tsdb.GetCurPC, msgId)
	tsdb.expTracer = &ExpOpTracer{msgId: msgId}
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
func (tsdb *TracerStateDB) saveMessageTraces() {
	tsdb.Traces.AddMemoryOps(tsdb.memoryTracer.Finalize())
	tsdb.Traces.AddStackOps(tsdb.stackTracer.Finalize())
	tsdb.Traces.AddZKEVMStates(tsdb.zkevmTracer.Finalize())
	tsdb.Traces.AddStorageOps(tsdb.storageTracer.GetStorageOps())
	tsdb.Traces.AddExpOps(tsdb.expTracer.Finalize())
	tsdb.Traces.AddCopyEvents(tsdb.copyTracer.Finalize())
}

func (tsdb *TracerStateDB) resetVm() {
	tsdb.stackTracer = nil
	tsdb.memoryTracer = nil
	tsdb.expTracer = nil
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
	tsdb.saveMessageTraces()
	return nil
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

	tsdb.saveMessageTraces()
	return nil
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
	_, err := tsdb.mptTracer.CreateAccount(addr)
	return err
}

func (tsdb *TracerStateDB) CreateContract(addr types.Address) error {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil {
		return err
	}

	acc.NewContract = true

	return nil
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
	acc, err := tsdb.mptTracer.GetAccount(addr)
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
	acc, err := tsdb.mptTracer.GetAccount(addr)
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
		return nil, common.EmptyHash, errors.New("no code is currently executed")
	}
	return tsdb.code, tsdb.codeHash, nil
}

func (tsdb *TracerStateDB) GetCode(addr types.Address) ([]byte, common.Hash, error) {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil || acc == nil {
		return nil, common.EmptyHash, err
	}

	// if contract code was requested, we dump it into traces
	tsdb.Traces.AddContractBytecode(addr, acc.Code)

	return acc.Code, acc.Code.Hash(), nil
}

func (tsdb *TracerStateDB) SetCode(addr types.Address, code []byte) error {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.SetCode(types.Code(code).Hash(), code)
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

func (tsdb *TracerStateDB) GetCommittedState(addr types.Address, key common.Hash) common.Hash {
	// copied from state.go
	return common.EmptyHash
}

func (tsdb *TracerStateDB) GetState(addr types.Address, key common.Hash) (common.Hash, error) {
	val, err := tsdb.storageTracer.GetSlot(addr, key)
	if err != nil {
		return common.EmptyHash, err
	}
	// `storageTracer.GetSlot` returns `nil, nil` in case of no such addr exists.
	// Such read operation will be also included into traces.
	// Pass slot data to zkevm_state
	tsdb.zkevmTracer.SetLastStateStorage((types.Uint256)(*key.Uint256()), (types.Uint256)(*val.Uint256()))
	return val, nil
}

func (tsdb *TracerStateDB) SetState(addr types.Address, key common.Hash, val common.Hash) error {
	_, err := tsdb.getOrNewAccount(addr)
	if err != nil {
		return err
	}

	prev, err := tsdb.storageTracer.SetSlot(addr, key, val)
	// Pass slote data before setting to zkevm_state
	tsdb.zkevmTracer.SetLastStateStorage((types.Uint256)(*key.Uint256()), types.Uint256(*prev.Uint256()))
	return err
}

func (tsdb *TracerStateDB) GetStorageRoot(addr types.Address) (common.Hash, error) {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil || acc == nil {
		return common.Hash{}, err
	}

	return acc.AccountState.StorageTree.RootHash(), nil
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
	account, err := tsdb.mptTracer.GetAccount(address)
	if err != nil {
		return false, err
	}
	return account != nil, nil
}

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (tsdb *TracerStateDB) Empty(addr types.Address) (bool, error) {
	acc, err := tsdb.mptTracer.GetAccount(addr)
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

func (tsdb *TracerStateDB) AddLog(*types.Log) error {
	return nil
}

func (tsdb *TracerStateDB) AddDebugLog(*types.DebugLog) error {
	return nil
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

func (tsdb *TracerStateDB) FinalizeTraces() error {
	mptTraces, err := tsdb.mptTracer.GetMPTTraces()
	if err != nil {
		return err
	}
	tsdb.Traces.SetMptTraces(&mptTraces)
	tsdb.Stats.AffectedContractsN = uint(len(mptTraces.ContractTrieTraces))
	return nil
}
