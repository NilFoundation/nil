package tracer

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
	"github.com/rs/zerolog"
)

type TracerStateDB struct {
	client           api.RpcClient
	shardId          types.ShardId
	InTransactions   []*types.Transaction
	blkContext       *vm.BlockContext
	Traces           ExecutionTraces
	RwCounter        RwCounter
	Stats            Stats
	AccountSparseMpt mpt.MerklePatriciaTrie
	logger           zerolog.Logger
	mptTracer        *mpttracer.MPTTracer // unlike others MPT tracer keeps its state between transactions
	gasPrice         types.Value
	refund           uint64
	configAccessor   config.ConfigAccessor

	// Reinited for each transaction
	txnTraceCtx *transactionTraceContext
	// txnFeeCredit holds the total fee credit for the inbound transaction. It can be changed during execution in case
	// of Response tx, thus we use this separate variable instead of the one in the transaction.
	txnFeeCredit types.Value
}

var _ vm.StateDB = (*TracerStateDB)(nil)

type transactionTraceContext struct {
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
	keccakTracer  *KeccakTracer

	// Current program counter, used only for storage operations trace. Incremetned inside OnOpcode
	curPC uint64
}

func (mtc *transactionTraceContext) processOpcode(
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
	// Each operation consists of read stack -> read data -> write data -> write stack (we
	// ignore specific memory parts like returndata, etc for now). Intermediate stages could be omitted, but
	// to keep RW ctr correct, stack tracer should be run the first on new opcode, and be finalized the last on previous opcode.
	// TODO: add check that only one of first 3 is run
	mtc.memoryTracer.FinishPrevOpcodeTracing()
	mtc.expTracer.FinishPrevOpcodeTracing()
	mtc.storageTracer.FinishPrevOpcodeTracing()
	mtc.stackTracer.FinishPrevOpcodeTracing()
	mtc.keccakTracer.FinishPrevOpcodeTracing()
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

	if err := mtc.stackTracer.TraceOp(opCode, pc, scope); err != nil {
		return err
	}
	stats.StackOpsN++

	if hasMemOps {
		copyOccurred, err := mtc.copyTracer.TraceOp(opCode, mtc.rwCounter.ctr, scope, returnData)
		if err != nil {
			return err
		}
		if copyOccurred {
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

	keccakTraced, err := mtc.keccakTracer.TraceOp(opCode, scope)
	if err != nil {
		return err
	}
	if keccakTraced {
		stats.KeccakOpsN++
	}

	storageTraced, err := mtc.storageTracer.TraceOp(opCode, pc, scope)
	if err != nil {
		return err
	}
	if storageTraced {
		stats.StateOpsN++
	}

	return nil
}

func (mtc *transactionTraceContext) saveTransactionTraces(dst ExecutionTraces) error {
	dst.AddMemoryOps(mtc.memoryTracer.Finalize())
	dst.AddStackOps(mtc.stackTracer.Finalize())
	dst.AddZKEVMStates(mtc.zkevmTracer.Finalize())
	dst.AddExpOps(mtc.expTracer.Finalize())
	dst.AddKeccakOps(mtc.keccakTracer.Finalize())
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
	client api.RpcClient,
	shardId types.ShardId,
	prevBlockNumber types.BlockNumber,
	blkContext *vm.BlockContext,
	db db.DB,
	chainConfig *jsonrpc.ChainConfig,
	logger zerolog.Logger,
) (*TracerStateDB, error) {
	rwTx, err := db.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}

	if chainConfig == nil {
		return nil, errors.New("chain config must be provided")
	}
	configMap, err := chainConfig.ToMap()
	if err != nil {
		return nil, err
	}
	configAccessor := config.NewConfigAccessorFromMap(configMap)

	return &TracerStateDB{
		client:         client,
		mptTracer:      mpttracer.New(client, prevBlockNumber, rwTx, shardId),
		shardId:        shardId,
		blkContext:     blkContext,
		Traces:         aggTraces,
		logger:         logger,
		configAccessor: configAccessor,

		gasPrice: types.NewZeroValue(),
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

// OutTransactions don't require handling, they are just included into block
func (tsdb *TracerStateDB) HandleInTransaction(transaction *types.Transaction, payer execution.Payer) (err error) {
	tsdb.logger.Trace().
		Int64("seqno", int64(transaction.Seqno)).
		Str("flags", transaction.Flags.String()).
		Msg("tracing in_transaction")

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

	tsdb.txnFeeCredit = transaction.FeeCredit
	tsdb.updateGasPrice(transaction)
	payer.SubBalance(transaction.FeeCredit) // buy gas

	switch {
	case transaction.IsRefund():
		err = tsdb.handleRefundTransaction(transaction)
	case transaction.IsDeploy():
		err = tsdb.handleDeployTransaction(transaction)
	case transaction.IsExecution():
		err = tsdb.handleExecutionTransaction(transaction)
	default:
		err = fmt.Errorf("unknown transaction type: %+v", transaction)
	}

	tsdb.Stats.ProcessedInTxnsN++
	return err
}

func (tsdb *TracerStateDB) handleRefundTransaction(transaction *types.Transaction) error {
	return tsdb.AddBalance(transaction.To, transaction.Value, tracing.BalanceIncreaseRefund)
}

func (tsdb *TracerStateDB) handleExecutionTransaction(transaction *types.Transaction) error {
	if transaction.IsResponse() {
		return errors.New("Can't handle response yet")
	}

	caller := (vm.AccountRef)(transaction.From)
	callData := transaction.Data

	code, _, err := tsdb.GetCode(transaction.To)
	if err != nil {
		return err
	}

	tsdb.txnTraceCtx = tsdb.initTransactionTraceContext(
		transaction.IsInternal(),
		transaction.From,
		transaction.Token,
		code,
		nil, // vm reset state
	)
	defer tsdb.resetTxnTrace()

	gas := transaction.FeeCredit.ToGas(tsdb.gasPrice)
	ret, gasLeft, err := tsdb.txnTraceCtx.evm.Call(caller, transaction.To, callData, gas.Uint64(), transaction.Value.Int())
	_, _ = ret, gasLeft

	if err != nil {
		return err
	}

	return tsdb.txnTraceCtx.saveTransactionTraces(tsdb.Traces)
}

func (tsdb *TracerStateDB) handleDeployTransaction(transaction *types.Transaction) error {
	addr := transaction.To
	deployTxn := types.ParseDeployPayload(transaction.Data)

	tsdb.txnTraceCtx = tsdb.initTransactionTraceContext(
		transaction.IsInternal(),
		transaction.From,
		nil, // token transfer
		deployTxn.Code(),
		nil, // vm reset state
	)
	defer tsdb.resetTxnTrace()

	gas := transaction.FeeCredit.ToGas(tsdb.gasPrice)
	ret, addr, leftOver, err := tsdb.txnTraceCtx.evm.Deploy(addr, (vm.AccountRef)(transaction.From), deployTxn.Code(), gas.Uint64(), transaction.Value.Int())
	if err != nil {
		return err
	}
	// `_, _, _, err` doesn't satisfy linter
	_ = ret
	_ = addr
	_ = leftOver

	return tsdb.txnTraceCtx.saveTransactionTraces(tsdb.Traces)
}

func (tsdb *TracerStateDB) initTransactionTraceContext(
	internal bool,
	origin types.Address,
	tokens []types.TokenBalance,
	executingCode types.Code,
	state *vm.EvmRestoreData,
) *transactionTraceContext {
	txnId := uint(len(tsdb.InTransactions) - 1)
	codeHash := getCodeHash(executingCode)
	txnTraceCtx := &transactionTraceContext{
		evm:       vm.NewEVM(tsdb.blkContext, tsdb, origin, tsdb.gasPrice, state),
		code:      executingCode,
		codeHash:  codeHash,
		rwCounter: &tsdb.RwCounter,

		stackTracer:   NewStackOpTracer(&tsdb.RwCounter, txnId),
		memoryTracer:  NewMemoryOpTracer(&tsdb.RwCounter, txnId),
		expTracer:     NewExpOpTracer(txnId),
		keccakTracer:  NewKeccakTracer(),
		storageTracer: NewStorageOpTracer(&tsdb.RwCounter, txnId, tsdb),

		zkevmTracer: NewZkEVMStateTracer(
			&tsdb.RwCounter,
			tsdb.GetInTransaction().Hash(),
			codeHash,
			txnId,
		),

		copyTracer: NewCopyTracer(tsdb, txnId),
	}

	txnTraceCtx.evm.IsAsyncCall = internal
	txnTraceCtx.evm.SetTokenTransfer(tokens)
	txnTraceCtx.evm.Config.Tracer = &tracing.Hooks{
		OnOpcode: func(pc uint64, op byte, gas uint64, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
			if err != nil {
				return // this error will be forwarded to the caller as is, no need to trace anything
			}

			// debug-only: ensure that tracer impl did not change any data from the EVM context
			verifyIntegrity := assertEVMStateConsistent(pc, scope, returnData)
			defer verifyIntegrity()

			txnTraceCtx.curPC = pc
			if err := txnTraceCtx.processOpcode(&tsdb.Stats, pc, op, gas, scope, returnData); err != nil {
				err = fmt.Errorf("pc: %d opcode: %X, gas: %d, cost: %d, mem_size: %d bytes, stack: %d items, ret_data_size: %d bytes, depth: %d cause: %w",
					pc, op, gas, cost, len(scope.MemoryData()), len(scope.StackData()), len(returnData), depth, err,
				)

				// tracer by default should not affect the code execution but since we only run code to collect the traces - we should know
				// about any failure as soon as possible instead of continue running
				panic(managedTracerFailureError{underlying: err})
			}
		},
	}

	return txnTraceCtx
}

func (tsdb *TracerStateDB) resetTxnTrace() {
	tsdb.txnTraceCtx = nil
}

// The only way to add InTransaction to state
func (tsdb *TracerStateDB) AddInTransaction(transaction *types.Transaction) {
	// We store a copy of the transaction, because the original transaction will be modified.
	tsdb.InTransactions = append(tsdb.InTransactions, common.CopyPtr(transaction))
}

// Read-only methods
func (tsdb *TracerStateDB) IsInternalTransaction() bool {
	// If contract calls another contract using EVM's call(depth > 1), we treat it as an internal transaction.
	return tsdb.GetInTransaction().IsInternal() || tsdb.txnTraceCtx.evm.GetDepth() > 1
}

func (tsdb *TracerStateDB) GetTransactionFlags() types.TransactionFlags {
	panic("not implemented")
}

func (tsdb *TracerStateDB) GetTokens(addr types.Address) map[types.TokenId]types.Value {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	check.PanicIfErr(err)
	if acc == nil {
		return nil
	}

	res := make(map[types.TokenId]types.Value)
	for k, v := range acc.TokenTree.Iterate() {
		var c types.TokenBalance
		c.Token = types.TokenId(k)
		if err := c.Balance.UnmarshalSSZ(v); err != nil {
			tsdb.logger.Error().Err(err).Msg("failed to unmarshal token balance")
			continue
		}
		res[c.Token] = c.Balance
	}

	return res
}

func (tsdb *TracerStateDB) GetGasPrice(types.ShardId) (types.Value, error) {
	return tsdb.gasPrice, nil
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

func (tsdb *TracerStateDB) AddToken(addr types.Address, tokenId types.TokenId, amount types.Value) error {
	tsdb.logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", tokenId).
		Msg("Add token")

	acc, err := tsdb.mptTracer.GetAccount(addr)
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

func (tsdb *TracerStateDB) SubToken(addr types.Address, tokenId types.TokenId, amount types.Value) error {
	tsdb.logger.Debug().
		Stringer("addr", addr).
		Stringer("amount", amount).
		Stringer("id", tokenId).
		Msg("Sub token")

	acc, err := tsdb.mptTracer.GetAccount(addr)
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

func (tsdb *TracerStateDB) SetTokenTransfer(tokens []types.TokenBalance) {
	tsdb.txnTraceCtx.evm.SetTokenTransfer(tokens)
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
	mctx := tsdb.txnTraceCtx
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

	return acc.Code, getCodeHash(acc.Code), nil
}

func (tsdb *TracerStateDB) SetCode(addr types.Address, code []byte) error {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil {
		return err
	}
	acc.SetCode(getCodeHash(types.Code(code)), code)
	return nil
}

func (tsdb *TracerStateDB) AddRefund(gas uint64) {
	tsdb.refund += gas
}

func (tsdb *TracerStateDB) SubRefund(gas uint64) {
	check.PanicIff(gas > tsdb.refund, "Refund counter below zero (gas: %d > refund: %d)", gas, tsdb.refund)
	tsdb.refund -= gas
}

// GetRefund returns the current value of the refund counter.
func (tsdb *TracerStateDB) GetRefund() uint64 {
	return tsdb.refund
}

func (tsdb *TracerStateDB) GetCommittedState(addr types.Address, key common.Hash) common.Hash {
	// copied from state.go
	return common.EmptyHash
}

func (tsdb *TracerStateDB) GetState(addr types.Address, key common.Hash) (common.Hash, error) {
	val, err := tsdb.mptTracer.GetSlot(addr, key)
	if err != nil {
		return common.EmptyHash, err
	}
	// `mptTracer.GetSlot` returns `nil, nil` in case of no such addr exists.
	// Such read operation will be also included into traces.
	// Pass slot data to zkevm_state
	if err := tsdb.txnTraceCtx.zkevmTracer.SetLastStateStorage(
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

	prevValue, err := tsdb.mptTracer.GetSlot(addr, key)
	if err != nil {
		return err
	}

	err = tsdb.mptTracer.SetSlot(addr, key, val)
	if err != nil {
		return err
	}

	// Pass slote data before setting to zkevm_state
	return tsdb.txnTraceCtx.zkevmTracer.SetLastStateStorage((types.Uint256)(*key.Uint256()), types.Uint256(*prevValue.Uint256()))
}

func (tsdb *TracerStateDB) GetStorageRoot(addr types.Address) (common.Hash, error) {
	acc, err := tsdb.mptTracer.GetAccount(addr)
	if err != nil || acc == nil {
		return common.EmptyHash, err
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

func (tsdb *TracerStateDB) Rollback(_, _ uint32, _ uint64) error {
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
	panic("prover execution should not revert")
}

// Snapshot returns an identifier for the current revision of the state.
func (tsdb *TracerStateDB) Snapshot() int {
	// Snapshot is needed for rollback when an error was returned by the EVM.
	// We could just ignore failing transactions in proof provider. In case revert occurs, we fail in RevertToSnapshot(int)
	return 0
}

func (tsdb *TracerStateDB) AddLog(*types.Log) error {
	return nil
}

func (tsdb *TracerStateDB) AddDebugLog(*types.DebugLog) error {
	return nil
}

// AddOutTransaction adds internal out transaction for current transaction
func (tsdb *TracerStateDB) AddOutTransaction(caller types.Address, payload *types.InternalTransactionPayload) (*types.Transaction, error) {
	// TODO: seems useless now, implement when final hash calculation is needed
	return nil, nil
}

// AddOutRequestTransaction adds outbound request transaction for current transaction
func (tsdb *TracerStateDB) AddOutRequestTransaction(
	caller types.Address,
	payload *types.InternalTransactionPayload,
	responseProcessingGas types.Gas,
	isAwait bool,
) (*types.Transaction, error) {
	return nil, errors.New("not implemented")
}

// Get current transaction
func (tsdb *TracerStateDB) GetInTransaction() *types.Transaction {
	if len(tsdb.InTransactions) == 0 {
		return nil
	}
	return tsdb.InTransactions[len(tsdb.InTransactions)-1]
}

// Get execution context shard id
func (tsdb *TracerStateDB) GetShardID() types.ShardId {
	panic("not implemented")
}

// SaveVmState saves current VM state
func (tsdb *TracerStateDB) SaveVmState(state *types.EvmState, continuationGasCredit types.Gas) error {
	return errors.New("not implemented")
}

func (tsdb *TracerStateDB) GetConfigAccessor() config.ConfigAccessor {
	return tsdb.configAccessor
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

func (tsdb *TracerStateDB) updateGasPrice(txn *types.Transaction) {
	tsdb.gasPrice = types.NewValueFromBigMust(tsdb.blkContext.BaseFee).Add(txn.MaxPriorityFeePerGas)

	if tsdb.gasPrice.Cmp(txn.MaxFeePerGas) > 0 {
		tsdb.gasPrice = txn.MaxFeePerGas
	}
}
