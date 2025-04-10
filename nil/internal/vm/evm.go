package vm

import (
	"encoding/binary"
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/params"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/holiman/uint256"
)

type (
	// GetHashFunc returns the n'th block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc func(uint64) (common.Hash, error)
)

func (evm *EVM) precompile(addr types.Address) (PrecompiledContract, bool) {
	precompiles := PrecompiledContractsPrague
	p, ok := precompiles[addr]
	return p, ok
}

// BlockContext provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type BlockContext struct {
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Block information
	Coinbase    types.Address // Provides information for COINBASE
	GasLimit    uint64        // Provides information for GASLIMIT
	BlockNumber uint64        // Provides information for NUMBER
	Time        uint64        // Provides information for TIME
	BaseFee     *big.Int      // Provides information for BASEFEE (0 if vm runs with NoBaseFee flag and 0 gas price)
	BlobBaseFee *big.Int      // Provides information for BLOBBASEFEE
	//                           (0 if vm runs with NoBaseFee flag and 0 blob gas price)
	Random *common.Hash // Provides information for PREVRANDAO

	RollbackCounter uint32 // Provides information for rollback handling
}

// TxContext provides the EVM with information about a transaction.
// All fields can change between transactions.
type TxContext struct {
	// Transaction information
	Origin     types.Address // Provides information for ORIGIN
	GasPrice   *big.Int      // Provides information for GASPRICE (and is used to zero the basefee if NoBaseFee is set)
	BlobHashes []common.Hash // Provides information for BLOBHASH
}

// EVM is the Ethereum Virtual Machine base object and provides
// the necessary tools to run a contract on the given state with
// the provided context. It should be noted that any error
// generated through any of the calls should be considered a
// revert-state-and-consume-all-gas operation, no checks on
// specific errors should ever be performed. The interpreter makes
// sure that any errors generated are to be considered faulty code.
//
// The EVM should never be reused and is not thread safe.
type EVM struct {
	// Context provides auxiliary blockchain related information
	Context *BlockContext
	TxContext
	// StateDB gives access to the underlying state
	StateDB StateDB
	// Depth is the current call stack
	depth int
	// Indicates whether this is async call
	IsAsyncCall bool

	// chainConfig contains information about the current chain
	chainConfig *params.ChainConfig
	// virtual machine configuration options used to initialise the
	// evm.
	Config Config

	// global (to this context) ethereum virtual machine
	// used throughout the execution of the tx.
	interpreter *EVMInterpreter
	// abort is used to abort the EVM calling operations
	abort atomic.Bool
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64

	// tokenTransfer holds the tokens that will be transferred in next Call opcode.
	// Main usage is a transfer token through regular EVM Call opcode in Nil Solidity library(syncCall function).
	tokenTransfer []types.TokenBalance

	RevertReason error

	DebugInfo *DebugInfo
}

type DebugInfo struct {
	Pc uint64
}

// NewEVM returns a new EVM. The returned EVM is not thread safe and should
// only ever be used *once*.
func NewEVM(
	blockContext *BlockContext,
	statedb StateDB,
	origin types.Address,
	gasPrice types.Value,
	state *EvmRestoreData,
) *EVM {
	evm := &EVM{
		Context: blockContext,
		StateDB: statedb,
		TxContext: TxContext{
			Origin:   origin,
			GasPrice: gasPrice.ToBig(),
		},
		chainConfig: &params.ChainConfig{ChainID: big.NewInt(1)},
	}
	evm.interpreter = NewEVMInterpreter(evm, state)
	return evm
}

// Interpreter returns the current interpreter
func (evm *EVM) Interpreter() *EVMInterpreter {
	return evm.interpreter
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (evm *EVM) Call(
	caller ContractRef,
	addr types.Address,
	input []byte,
	gas uint64,
	value *uint256.Int,
) ([]byte, uint64, error) {
	const readOnly = false

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	// Fail if we're trying to transfer more than the available balance
	if !value.IsZero() {
		if can, err := evm.canTransfer(caller.Address(), value); err != nil {
			return nil, gas, err
		} else if !can {
			return nil, gas, ErrInsufficientBalance
		}
	}
	snapshot := evm.StateDB.Snapshot()
	p, isPrecompile := evm.precompile(addr)

	var ret []byte
	var runErr error
	if isPrecompile {
		ret, gas, runErr = RunPrecompiledContract(p, evm, input, gas, evm.Config.Tracer, value, caller, readOnly)
	} else {
		if exists, err := evm.StateDB.Exists(addr); err != nil {
			return nil, gas, err
		} else if !exists {
			if value.IsZero() {
				// Calling a non-existing account, don't do anything.
				return nil, gas, nil
			}
			if err := evm.StateDB.CreateAccount(addr); err != nil {
				return nil, gas, err
			}
		}

		tokenTransfer := evm.tokenTransfer
		if err := evm.transfer(caller.Address(), addr, value); err != nil {
			return nil, gas, err
		}

		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		code, codeHash, err := evm.StateDB.GetCode(addr)
		if err != nil {
			return nil, gas, err
		}
		if len(code) == 0 {
			return nil, gas, nil // gas is unchanged
		}

		// If the account has no code, we can abort here
		// The depth-check is already done, and precompiles handled above
		contract := NewContract(caller, AccountRef(addr), value, gas, tokenTransfer)
		contract.SetCallCode(addr, codeHash, code)
		ret, runErr = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}

	// When an error was returned by the EVM or when setting the creation code.
	// Above, we revert to the snapshot and consume any gas remaining.
	// Additionally, when we're in homestead, this also counts for code storage gas errors.
	if runErr != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if errors.Is(runErr, ErrExecutionReverted) {
			if evm.RevertReason != nil {
				runErr = evm.RevertReason
			}
		} else {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
		transaction := evm.StateDB.GetInTransaction()
		if transaction != nil && transaction.IsBounce() {
			// Re-transfer value and token in case of bounce transaction.
			evm.tokenTransfer = transaction.Token
			if err := evm.transfer(caller.Address(), addr, value); err != nil {
				return nil, gas, err
			}
		}
		// TODO: consider clearing up unused snapshots:
		// } else {
		//	evm.StateDB.DiscardSnapshot(snapshot)
	}
	return ret, gas, runErr
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (evm *EVM) CallCode(
	caller ContractRef,
	addr types.Address,
	input []byte,
	gas uint64,
	value *uint256.Int,
) ([]byte, uint64, error) {
	const readOnly = false

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	// Note although it's noop to transfer X ether to caller itself. But
	// if caller doesn't have enough balance, it would be an error to allow
	// over-charging itself. So the check here is necessary.

	if can, err := evm.canTransfer(caller.Address(), value); err != nil {
		return nil, gas, err
	} else if !can {
		return nil, gas, ErrInsufficientBalance
	}
	snapshot := evm.StateDB.Snapshot()

	// It is allowed to call precompiles, even via delegatecall
	var ret []byte
	var runErr error
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, runErr = RunPrecompiledContract(p, evm, input, gas, evm.Config.Tracer, value, caller, readOnly)
	} else {
		code, codeHash, err := evm.StateDB.GetCode(addr)
		if err != nil {
			return nil, gas, err
		}

		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(caller.Address()), value, gas, nil)
		contract.SetCallCode(addr, codeHash, code)
		ret, runErr = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}
	if runErr != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(runErr, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, runErr
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (evm *EVM) DelegateCall(caller ContractRef, addr types.Address, input []byte, gas uint64) ([]byte, uint64, error) {
	const readOnly = false

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	snapshot := evm.StateDB.Snapshot()

	// It is allowed to call precompiles, even via delegatecall
	var ret []byte
	var runErr error
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, runErr = RunPrecompiledContract(p, evm, input, gas, evm.Config.Tracer, nil, caller, readOnly)
	} else {
		code, codeHash, err := evm.StateDB.GetCode(addr)
		if err != nil {
			return nil, gas, err
		}

		// Initialise a new contract and make initialise the delegate values
		contract := NewContract(caller, AccountRef(caller.Address()), nil, gas, nil).AsDelegate()
		contract.SetCallCode(addr, codeHash, code)
		ret, runErr = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}

	if runErr != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(runErr, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}
			gas = 0
		}
	}
	return ret, gas, runErr
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (evm *EVM) StaticCall(caller ContractRef, addr types.Address, input []byte, gas uint64) ([]byte, uint64, error) {
	const readOnly = true

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// We take a snapshot here. This is a bit counter-intuitive, and could probably be skipped.
	// However, even a staticcall is considered a 'touch'. On mainnet, static calls were introduced
	// after all empty accounts were deleted, so this is not required. However, if we omit this,
	// then certain tests start failing; stRevertTest/RevertPrecompiledTouchExactOOG.json.
	// We could change this, but for now it's left for legacy reasons
	snapshot := evm.StateDB.Snapshot()

	var ret []byte
	var runErr error
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, runErr = RunPrecompiledContract(p, evm, input, gas, evm.Config.Tracer, nil, caller, readOnly)
	} else {
		code, codeHash, err := evm.StateDB.GetCode(addr)
		if err != nil {
			return nil, gas, err
		}

		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(addr), new(uint256.Int), gas, nil)
		contract.SetCallCode(addr, codeHash, code)
		// When an error was returned by the EVM or when setting the creation code
		// above we revert to the snapshot and consume any gas remaining. Additionally
		// when we're in Homestead this also counts for code storage gas errors.
		ret, runErr = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}
	if runErr != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(runErr, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, runErr
}

// create creates a new contract using code as deployment code.
func (evm *EVM) create(
	caller ContractRef,
	codeAndHash types.Code,
	gas uint64,
	value *uint256.Int,
	address types.Address,
) ([]byte, types.Address, uint64, error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if evm.depth > int(params.CallCreateDepth) {
		return nil, types.Address{}, gas, ErrDepth
	}

	if can, err := evm.canTransfer(caller.Address(), value); err != nil {
		return nil, types.Address{}, gas, err
	} else if !can {
		return nil, types.Address{}, gas, ErrInsufficientBalance
	}

	// Ensure there's no existing contract already at the designated address.
	if exists, err := evm.StateDB.ContractExists(address); err != nil {
		return nil, types.Address{}, gas, err
	} else if exists {
		if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
			evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
		}
		return nil, types.Address{}, gas, ErrContractAddressCollision
	}

	// bump nonce only for sync calls
	if caller.Address() != types.EmptyAddress && !evm.IsAsyncCall {
		// bump caller's nonce
		nonce, err := evm.StateDB.GetSeqno(caller.Address())
		if err != nil {
			return nil, types.Address{}, gas, err
		}
		if nonce+1 < nonce {
			return nil, types.Address{}, gas, ErrNonceUintOverflow
		}
		if err := evm.StateDB.SetSeqno(caller.Address(), nonce+1); err != nil {
			return nil, types.Address{}, gas, err
		}
	}

	// Create a new account on the state only if the object was not present.
	// It might be possible the contract code is deployed to a pre-existent
	// account with non-zero balance.
	snapshot := evm.StateDB.Snapshot()
	if exists, err := evm.StateDB.Exists(address); err != nil {
		return nil, types.Address{}, gas, err
	} else if !exists {
		if err := evm.StateDB.CreateAccount(address); err != nil {
			return nil, types.Address{}, gas, err
		}
	}

	// CreateContract means that regardless of whether the account previously existed
	// in the state trie or not, it _now_ becomes created as a _contract_ account.
	// This is performed _prior_ to executing the initcode, since the initcode
	// acts inside that account.
	if err := evm.StateDB.CreateContract(address); err != nil {
		return nil, types.Address{}, gas, err
	}

	if err := evm.StateDB.SetSeqno(address, 1); err != nil {
		return nil, types.Address{}, gas, err
	}

	if err := evm.transfer(caller.Address(), address, value); err != nil {
		return nil, types.Address{}, gas, err
	}

	// Initialise a new contract and set the code that is to be used by the EVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, AccountRef(address), value, gas, nil)
	contract.SetCallCode(address, codeAndHash.Hash(), codeAndHash)

	ret, err := evm.interpreter.Run(contract, nil, false)

	// Check whether the max code size has been exceeded (EIP-158)
	if err == nil && len(ret) > params.MaxCodeSize {
		err = ErrMaxCodeSizeExceeded
	}

	// Reject code starting with 0xEF (EIP-3541)
	if err == nil && len(ret) >= 1 && ret[0] == 0xEF {
		err = ErrInvalidCode
	}

	if err == nil {
		createDataGas := uint64(len(ret)) * params.CreateDataGas
		if contract.UseGas(createDataGas, evm.Config.Tracer, tracing.GasChangeCallCodeStorage) {
			err = evm.StateDB.SetCode(address, ret)
		} else {
			err = types.NewError(types.ErrorOutOfGasStorage)
		}
	}

	// When an error was returned by the EVM or when setting the creation code.
	// Above, we revert to the snapshot and consume any gas remaining.
	// Additionally, this also counts for code storage gas errors.
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(err, ErrExecutionReverted) {
			contract.UseGas(contract.Gas, evm.Config.Tracer, tracing.GasChangeCallFailedExecution)
		}
		ret = nil
	}

	return ret, address, contract.Gas, err
}

// Deploy deploys a new contract from a deployment transaction
func (evm *EVM) Deploy(
	addr types.Address,
	caller ContractRef,
	code []byte,
	gas uint64,
	value *uint256.Int,
) (ret []byte, deployAddr types.Address, leftOverGas uint64, err error) {
	return evm.create(caller, code, gas, value, addr)
}

// Create creates a new contract using code as deployment code.
func (evm *EVM) Create(
	caller ContractRef,
	code []byte,
	gas uint64,
	value *uint256.Int,
) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error) {
	addr := caller.Address()
	seqno, err := evm.StateDB.GetSeqno(addr)
	if err != nil {
		return nil, types.Address{}, gas, err
	}
	extSeqno, err := evm.StateDB.GetExtSeqno(addr)
	if err != nil {
		return nil, types.Address{}, gas, err
	}

	var salt common.Hash
	copy(salt[0:16], addr[4:])
	binary.BigEndian.PutUint64(salt[16:24], seqno.Uint64())
	binary.BigEndian.PutUint64(salt[24:32], extSeqno.Uint64())
	payload := types.BuildDeployPayload(code, salt)
	contractAddr = types.CreateAddress(caller.Address().ShardId(), payload)
	return evm.create(caller, code, gas, value, contractAddr)
}

// Create2 creates a new contract using code as deployment code.
//
// The difference between Create2 with Create is Create2 uses hash(0xff ++ msg.sender ++ salt ++ hash(init_code))[12:]
// instead of the usual sender-and-nonce-hash as the address where the contract is initialized at.
func (evm *EVM) Create2(
	caller ContractRef,
	code []byte,
	gas uint64,
	endowment *uint256.Int,
	salt *uint256.Int,
) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error) {
	contractAddr = types.CreateAddressForCreate2(caller.Address(), code, common.BytesToHash(salt.Bytes()))
	return evm.create(caller, code, gas, endowment, contractAddr)
}

// canTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func (evm *EVM) canTransfer(addr types.Address, amount *uint256.Int) (bool, error) {
	// We don't need to check the balance for the async call
	if evm.IsAsyncCall {
		return true, nil
	}
	balance, err := evm.StateDB.GetBalance(addr)
	if err != nil {
		return false, err
	}
	if balance.Cmp(types.NewValue(amount)) < 0 {
		return false, nil
	}

	if len(evm.tokenTransfer) > 0 {
		accTokens := evm.StateDB.GetTokens(addr)
		for _, token := range evm.tokenTransfer {
			balance, ok := accTokens[token.Token]
			if !ok {
				balance = types.Value{}
			}
			if balance.Cmp(token.Balance) < 0 {
				return false, nil
			}
		}
	}

	return true, nil
}

// transfer subtracts amount from sender and adds amount to recipient using the given Db
func (evm *EVM) transfer(sender, recipient types.Address, a *uint256.Int) error {
	amount := types.Value{Uint256: types.CastToUint256(a)}
	// We don't need to subtract balance from async call
	if !evm.IsAsyncCall {
		if err := evm.StateDB.SubBalance(sender, amount, tracing.BalanceChangeTransfer); err != nil {
			return err
		}
	}
	if len(evm.tokenTransfer) > 0 {
		defer func() { evm.tokenTransfer = nil }()

		for _, token := range evm.tokenTransfer {
			if evm.depth > 0 {
				if err := evm.StateDB.SubToken(sender, token.Token, token.Balance); err != nil {
					return err
				}
			}
			if err := evm.StateDB.AddToken(recipient, token.Token, token.Balance); err != nil {
				return err
			}
		}
	}

	return evm.StateDB.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}

func (evm *EVM) GetDepth() int {
	return evm.depth
}

func (evm *EVM) SetTokenTransfer(tokens []types.TokenBalance) {
	evm.tokenTransfer = tokens
}

func (evm *EVM) StopAndDumpState(continuationGasCredit types.Gas) {
	evm.interpreter.stopAndDumpState = true
	evm.interpreter.continuationGasCredit = continuationGasCredit
}

// GetVMContext provides context about the block being executed as well as state
// to the tracers.
func (evm *EVM) GetVMContext() *tracing.VMContext {
	return &tracing.VMContext{
		Coinbase:    evm.Context.Coinbase,
		BlockNumber: big.NewInt(int64(evm.Context.BlockNumber)),
		Time:        evm.Context.Time,
		Random:      evm.Context.Random,
		BaseFee:     evm.Context.BaseFee,
		StateDB:     evm.StateDB,
	}
}
