package tracer

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/mpttracer"
	"github.com/rs/zerolog"
)

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
	logger           zerolog.Logger
	mptTracer        *mpttracer.MPTTracer // unlike others MPT tracer keeps its state between transactions

	// gas price for current block
	GasPrice types.Value

	// Reinited for each message
	msgTraceCtx *messageTraceContext
}

var _ vm.StateDB = (*TracerStateDB)(nil)

type messageTraceContext struct {
	evm       *vm.EVM     // EVM instance re-running current transaction
	code      []byte      // currently executed code
	codeHash  common.Hash // hash of this.code
	rwCounter *RwCounter  // inherited from TracerStateDB, sequential RW operations counter

	// tracers recording different events
	stackTracer   *StackOpTracer
	memoryTracer  *MemoryOpTracer
	storageTracer *StorageOpTracer
	zkevmTracer   *ZKEVMStateTracer
	copyTracer    *CopyTracer
	expTracer     *ExpOpTracer

	// Current program counter, used only for storage operations trace. Incremetned inside OnOpcode
	curPC uint64
}

func (mtc *messageTraceContext) processOpcode(
	stats *Stats,
	pc uint64,
	op byte,
	gas uint64,
	scope tracing.OpContext,
	returnData []byte,
) error {
	opCode := vm.OpCode(op)
	stats.OpsN++

	// Finish in reverse order to keep rw_counter sequential.
	// Each operation consists of read stack -> read memory -> write memory -> write stack (we
	// ignore specific memory parts like returndata, etc for now). Stages could be omitted, but
	// not reordered.
	mtc.memoryTracer.FinishPrevOpcodeTracing()
	mtc.stackTracer.FinishPrevOpcodeTracing()
	mtc.expTracer.FinishPrevOpcodeTracing()
	if err := mtc.copyTracer.FinishPrevOpcodeTracing(); err != nil {
		return err
	}

	ranges, hasMemOps := mtc.memoryTracer.GetUsedMemoryRanges(opCode, scope)

	// Store zkevmState before counting rw operations
	numRequiredStackItems := mtc.evm.Interpreter().GetNumRequiredStackItems(opCode)
	additionalInput := types.NewUint256(0) // data for pushX opcodes
	if len(mtc.code) != 0 && opCode.IsPush() {
		bytesToPush := uint64(opCode) - uint64(vm.PUSH0)
		if bytesToPush > 0 {
			additionalInput = types.NewUint256FromBytes(mtc.code[pc+1 : pc+bytesToPush+1])
		}
	}
	if err := mtc.zkevmTracer.TraceOp(opCode, pc, gas, numRequiredStackItems, additionalInput, ranges, scope); err != nil {
		return err
	}

	// Stack tracing is splitted between current opcode (before change read operations)
	// and the next opcode (after change write operations)
	if err := mtc.stackTracer.TraceOp(opCode, pc, scope); err != nil {
		return err
	}
	stats.StackOpsN++

	if hasMemOps {
		copyOccured, err := mtc.copyTracer.TraceOp(opCode, mtc.rwCounter.ctr, scope, returnData)
		if err != nil {
			return err
		}
		if copyOccured {
			stats.CopyOpsN++
		}

		if err := mtc.memoryTracer.TraceOp(opCode, pc, ranges, scope); err != nil {
			return err
		}
		stats.MemoryOpsN++
	}

	expTraced, err := mtc.expTracer.TraceOp(opCode, pc, scope)
	if err != nil {
		return err
	}
	if expTraced {
		stats.ExpOpsN++
	}

	// Storage tracing is done inside Get/SetState methods
	if opCode == vm.SLOAD || opCode == vm.SSTORE {
		stats.StateOpsN++
	}

	return nil
}

func (mtc *messageTraceContext) saveMessageTraces(dst ExecutionTraces) error {
	dst.AddMemoryOps(mtc.memoryTracer.Finalize())
	dst.AddStackOps(mtc.stackTracer.Finalize())
	dst.AddZKEVMStates(mtc.zkevmTracer.Finalize())
	dst.AddStorageOps(mtc.storageTracer.GetStorageOps())

	copies, err := mtc.copyTracer.Finalize()
	if err != nil {
		return err
	}
	dst.AddCopyEvents(copies)

	return nil
}

func NewTracerStateDB(
	ctx context.Context,
	aggTraces ExecutionTraces,
	client client.Client,
	shardId types.ShardId,
	shardBlockNumber types.BlockNumber,
	blkContext *vm.BlockContext,
	db db.DB,
	logger zerolog.Logger,
) (*TracerStateDB, error) {
	rwTx, err := db.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}

	return &TracerStateDB{
		client:           client,
		mptTracer:        mpttracer.New(client, shardBlockNumber, rwTx, shardId),
		shardId:          shardId,
		shardBlockNumber: shardBlockNumber,
		blkContext:       blkContext,
		Traces:           aggTraces,
		logger:           logger,
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
func (tsdb *TracerStateDB) HandleInMessage(message *types.Message) (err error) {
	tsdb.logger.Trace().
		Int64("seqno", int64(message.Seqno)).
		Str("flags", message.Flags.String()).
		Msg("tracing in_message")

	// handlers below initialize EVM instance with tracer
	// since tracer is not designed to return an error we just make it panic in case of failure and catch result here
	// it will help us to analyze logical errors in tracer impl down by the callstack
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if caughtErr, ok := r.(error); ok {
			var managed managedTracerFailureError
			if errors.As(caughtErr, &managed) {
				err = managed.Unwrap()
				return
			}
		}
		tsdb.logger.Error().Err(err).Str("stacktrace", string(debug.Stack())).Msg("trace collection failed")
		panic(r) // all unmanaged errors (runtime or explicit panic() calls) are rethrown from tracer with stack logging
	}()

	switch {
	case message.IsRefund():
		err = tsdb.HandleRefundMessage(message)
	case message.IsDeploy():
		err = tsdb.HandleDeployMessage(message)
	case message.IsExecution():
		err = tsdb.HandleExecutionMessage(message)
	default:
		err = fmt.Errorf("unknown message type: %+v", message)
	}

	tsdb.Stats.ProcessedInMsgsN++
	return //nolint:nakedret
}

func (tsdb *TracerStateDB) HandleRefundMessage(message *types.Message) error {
	return tsdb.AddBalance(message.To, message.Value, tracing.BalanceIncreaseRefund)
}

func (tsdb *TracerStateDB) HandleExecutionMessage(message *types.Message) error {
	if message.IsResponse() {
		return errors.New("Can't handle response yet")
	}

	caller := (vm.AccountRef)(message.From)
	callData := message.Data

	code, _, err := tsdb.GetCode(message.To)
	if err != nil {
		return err
	}

	tsdb.msgTraceCtx = tsdb.initMessageTraceContext(
		message.IsInternal(),
		message.From,
		message.Currency,
		code,
		nil, // vm reset state
	)
	defer tsdb.resetMsgTrace()

	gas := message.FeeCredit.ToGas(tsdb.GasPrice) // mb previous block, not current one?
	ret, gasLeft, err := tsdb.msgTraceCtx.evm.Call(caller, message.To, callData, gas.Uint64(), message.Value.Int())
	_, _ = ret, gasLeft

	if err != nil {
		return err
	}

	return tsdb.msgTraceCtx.saveMessageTraces(tsdb.Traces)
}

func (tsdb *TracerStateDB) HandleDeployMessage(message *types.Message) error {
	addr := message.To
	deployMsg := types.ParseDeployPayload(message.Data)

	tsdb.msgTraceCtx = tsdb.initMessageTraceContext(
		message.IsInternal(),
		message.From,
		nil, // currency transfer
		deployMsg.Code(),
		nil, // vm reset state
	)
	defer tsdb.resetMsgTrace()

	gas := message.FeeCredit.ToGas(tsdb.GasPrice)
	ret, addr, leftOver, err := tsdb.msgTraceCtx.evm.Deploy(addr, (vm.AccountRef)(message.From), deployMsg.Code(), gas.Uint64(), message.Value.Int())
	if err != nil {
		return err
	}
	// `_, _, _, err` doesn't satisfy linter
	_ = ret
	_ = addr
	_ = leftOver

	return tsdb.msgTraceCtx.saveMessageTraces(tsdb.Traces)
}

func (tsdb *TracerStateDB) initMessageTraceContext(
	internal bool,
	origin types.Address,
	currencies []types.CurrencyBalance,
	executingCode types.Code,
	state *vm.EvmRestoreData,
) *messageTraceContext {
	msgId := uint(len(tsdb.InMessages) - 1)
	codeHash := executingCode.Hash()
	msgTraceCtx := &messageTraceContext{
		evm:       vm.NewEVM(tsdb.blkContext, tsdb, origin, state),
		code:      executingCode,
		codeHash:  codeHash,
		rwCounter: &tsdb.RwCounter,

		stackTracer:   NewStackOpTracer(&tsdb.RwCounter, msgId),
		memoryTracer:  NewMemoryOpTracer(&tsdb.RwCounter, msgId),
		expTracer:     NewExpOpTracer(msgId),
		storageTracer: NewStorageOpTracer(tsdb.mptTracer, &tsdb.RwCounter, tsdb.GetCurPC, msgId),

		zkevmTracer: NewZkEVMStateTracer(
			&tsdb.RwCounter,
			tsdb.GetInMessage().Hash(),
			codeHash,
			msgId,
		),

		copyTracer: NewCopyTracer(tsdb, msgId),
	}

	msgTraceCtx.evm.IsAsyncCall = internal
	msgTraceCtx.evm.SetCurrencyTransfer(currencies)
	msgTraceCtx.evm.Config.Tracer = &tracing.Hooks{
		OnOpcode: func(pc uint64, op byte, gas uint64, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
			if err != nil {
				return // this error will be forwarded to the caller as is, no need to trace anything
			}

			// debug-only: ensure that tracer impl did not change any data from the EVM context
			verifyIntegrity := assertEVMStateConsistent(pc, scope, returnData)
			defer verifyIntegrity()

			msgTraceCtx.curPC = pc
			if err := msgTraceCtx.processOpcode(&tsdb.Stats, pc, op, gas, scope, returnData); err != nil {
				err = fmt.Errorf("pc: %d opcode: %X, gas: %d, cost: %d, mem_size: %d bytes, stack: %d items, ret_data_size: %d bytes, depth: %d cause: %w",
					pc, op, gas, cost, len(scope.MemoryData()), len(scope.StackData()), len(returnData), depth, err,
				)

				// tracer by default should not affect the code execution but since we only run code to collect the traces - we should know
				// about any failure as soon as possible instead of continue running
				panic(managedTracerFailureError{underlying: err})
			}
		},
	}

	return msgTraceCtx
}

func (tsdb *TracerStateDB) resetMsgTrace() {
	tsdb.msgTraceCtx = nil
}

// The only way to add InMessage to state
func (tsdb *TracerStateDB) AddInMessage(message *types.Message) {
	// We store a copy of the message, because the original message will be modified.
	tsdb.InMessages = append(tsdb.InMessages, common.CopyPtr(message))
}

// Read-only methods
func (tsdb *TracerStateDB) IsInternalMessage() bool {
	// If contract calls another contract using EVM's call(depth > 1), we treat it as an internal message.
	return tsdb.GetInMessage().IsInternal() || tsdb.msgTraceCtx.evm.GetDepth() > 1
}

func (tsdb *TracerStateDB) GetMessageFlags() types.MessageFlags {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetCurrencies(types.Address) map[types.CurrencyId]types.Value {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetGasPrice(types.ShardId) (types.Value, error) {
	return types.Value{}, errors.New("not implemented")
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
	mctx := tsdb.msgTraceCtx
	if mctx == nil || len(mctx.code) == 0 {
		return nil, common.EmptyHash, errors.New("no code is currently executed")
	}
	return mctx.code, mctx.codeHash, nil
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
	val, err := tsdb.msgTraceCtx.storageTracer.GetSlot(addr, key)
	if err != nil {
		return common.EmptyHash, err
	}
	// `storageTracer.GetSlot` returns `nil, nil` in case of no such addr exists.
	// Such read operation will be also included into traces.
	// Pass slot data to zkevm_state
	if err := tsdb.msgTraceCtx.zkevmTracer.SetLastStateStorage(
		(types.Uint256)(*key.Uint256()), (types.Uint256)(*val.Uint256()),
	); err != nil {
		return common.EmptyHash, err
	}
	return val, nil
}

func (tsdb *TracerStateDB) SetState(addr types.Address, key common.Hash, val common.Hash) error {
	_, err := tsdb.getOrNewAccount(addr)
	if err != nil {
		return err
	}

	prev, err := tsdb.msgTraceCtx.storageTracer.SetSlot(addr, key, val)
	if err != nil {
		return err
	}

	// Pass slote data before setting to zkevm_state
	return tsdb.msgTraceCtx.zkevmTracer.SetLastStateStorage((types.Uint256)(*key.Uint256()), types.Uint256(*prev.Uint256()))
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
	return false, errors.New("not implemented")
}

func (tsdb *TracerStateDB) Selfdestruct6780(types.Address) error {
	return errors.New("not implemented")
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
	if err != nil {
		return false, err
	}

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
	return nil, errors.New("not implemented")
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
	return errors.New("not implemented")
}

func (tsdb *TracerStateDB) GetConfigAccessor() *config.ConfigAccessor {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetCurPC() (uint64, error) {
	// Used by storage tracer
	mctx := tsdb.msgTraceCtx
	if mctx == nil {
		return 0, errors.New("attempt to get pc from unitialized tracer")
	}

	return mctx.curPC, nil
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
