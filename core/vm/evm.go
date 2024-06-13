package vm

import (
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/params"
	"github.com/holiman/uint256"
)

type (
	// GetHashFunc returns the n'th block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc func(uint64) common.Hash
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
	Difficulty  *big.Int      // Provides information for DIFFICULTY
	BaseFee     *big.Int      // Provides information for BASEFEE (0 if vm runs with NoBaseFee flag and 0 gas price)
	BlobBaseFee *big.Int      // Provides information for BLOBBASEFEE (0 if vm runs with NoBaseFee flag and 0 blob gas price)
	Random      *common.Hash  // Provides information for PREVRANDAO
}

// TxContext provides the EVM with information about a transaction.
// All fields can change between transactions.
type TxContext struct {
	// Message information
	Origin     types.Address // Provides information for ORIGIN
	GasPrice   *big.Int      // Provides information for GASPRICE (and is used to zero the basefee if NoBaseFee is set)
	BlobHashes []common.Hash // Provides information for BLOBHASH
	BlobFeeCap *big.Int      // Is used to zero the blobbasefee if NoBaseFee is set
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
	Context BlockContext
	TxContext
	// StateDB gives access to the underlying state
	StateDB StateDB
	// Depth is the current call stack
	depth int

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
}

// NewEVM returns a new EVM. The returned EVM is not thread safe and should
// only ever be used *once*.
func NewEVM(blockContext BlockContext, statedb StateDB) *EVM {
	evm := &EVM{
		Context: blockContext,
		StateDB: statedb,
	}
	evm.interpreter = NewEVMInterpreter(evm)
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
func (evm *EVM) Call(caller ContractRef, addr types.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	const readOnly = false

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	if !value.IsZero() && !canTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}
	snapshot := evm.StateDB.Snapshot()
	p, isPrecompile := evm.precompile(addr)

	if isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, evm.StateDB, input, gas, evm.Config.Tracer, gas, value, caller, readOnly)
	} else {
		if !evm.StateDB.Exist(addr) {
			if value.IsZero() {
				// Calling a non-existing account, don't do anything.
				return nil, gas, nil
			}
			evm.StateDB.CreateAccount(addr)
		}
		transfer(evm.StateDB, caller.Address(), addr, value)

		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		code := evm.StateDB.GetCode(addr)
		if len(code) == 0 {
			ret, err = nil, nil // gas is unchanged
		} else {
			addrCopy := addr
			// If the account has no code, we can abort here
			// The depth-check is already done, and precompiles handled above
			contract := NewContract(caller, AccountRef(addrCopy), value, gas)
			contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), code)
			ret, err = evm.interpreter.Run(contract, input, readOnly)
			gas = contract.Gas
		}
	}
	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally,
	// when we're in homestead this also counts for code storage gas errors.
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(err, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
		// TODO: consider clearing up unused snapshots:
		// } else {
		//	evm.StateDB.DiscardSnapshot(snapshot)
	}
	return ret, gas, err
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (evm *EVM) CallCode(caller ContractRef, addr types.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	const readOnly = false

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	// Fail if we're trying to transfer more than the available balance
	// Note although it's noop to transfer X ether to caller itself. But
	// if caller doesn't have enough balance, it would be an error to allow
	// over-charging itself. So the check here is necessary.
	if !canTransfer(evm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}
	snapshot := evm.StateDB.Snapshot()

	// It is allowed to call precompiles, even via delegatecall
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, evm.StateDB, input, gas, evm.Config.Tracer, gas, value, caller, readOnly)
	} else {
		addrCopy := addr
		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(caller.Address()), value, gas)
		contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), evm.StateDB.GetCode(addrCopy))
		ret, err = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(err, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, err
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (evm *EVM) DelegateCall(caller ContractRef, addr types.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	const readOnly = false

	// Fail if we're trying to execute above the call depth limit
	if evm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	snapshot := evm.StateDB.Snapshot()

	// It is allowed to call precompiles, even via delegatecall
	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, evm.StateDB, input, gas, evm.Config.Tracer, gas, nil, caller, readOnly)
	} else {
		addrCopy := addr
		// Initialise a new contract and make initialise the delegate values
		contract := NewContract(caller, AccountRef(caller.Address()), nil, gas).AsDelegate()
		contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), evm.StateDB.GetCode(addrCopy))
		ret, err = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(err, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}
			gas = 0
		}
	}
	return ret, gas, err
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (evm *EVM) StaticCall(caller ContractRef, addr types.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
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

	// We do an AddBalance of zero here, just in order to trigger a touch.
	// This doesn't matter on Mainnet, where all empties are gone at the time of Byzantium,
	// but is the correct thing to do and matters on other networks, in tests, and potential
	// future scenarios
	evm.StateDB.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeTouchAccount)

	if p, isPrecompile := evm.precompile(addr); isPrecompile {
		ret, gas, err = RunPrecompiledContract(p, evm.StateDB, input, gas, evm.Config.Tracer, gas, nil, caller, readOnly)
	} else {
		// At this point, we use a copy of address. If we don't, the go compiler will
		// leak the 'contract' to the outer scope, and make allocation for 'contract'
		// even if the actual execution ends on RunPrecompiled above.
		addrCopy := addr
		// Initialise a new contract and set the code that is to be used by the EVM.
		// The contract is a scoped environment for this execution context only.
		contract := NewContract(caller, AccountRef(addrCopy), new(uint256.Int), gas)
		contract.SetCallCode(&addrCopy, evm.StateDB.GetCodeHash(addrCopy), evm.StateDB.GetCode(addrCopy))
		// When an error was returned by the EVM or when setting the creation code
		// above we revert to the snapshot and consume any gas remaining. Additionally
		// when we're in Homestead this also counts for code storage gas errors.
		ret, err = evm.interpreter.Run(contract, input, readOnly)
		gas = contract.Gas
	}
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(err, ErrExecutionReverted) {
			if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
				evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
			}

			gas = 0
		}
	}
	return ret, gas, err
}

// create creates a new contract using code as deployment code.
func (evm *EVM) create(caller ContractRef, codeAndHash types.Code, gas uint64, value *uint256.Int, address types.Address) (ret []byte, createAddress types.Address, leftOverGas uint64, err error) {
	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if evm.depth > int(params.CallCreateDepth) {
		return nil, types.Address{}, gas, ErrDepth
	}
	if !canTransfer(evm.StateDB, caller.Address(), value) {
		return nil, types.Address{}, gas, ErrInsufficientBalance
	}
	if caller.Address() != types.EmptyAddress {
		// bump caller's nonce
		nonce := evm.StateDB.GetSeqno(caller.Address())
		if nonce+1 < nonce {
			return nil, types.Address{}, gas, ErrNonceUintOverflow
		}
		evm.StateDB.SetSeqno(caller.Address(), nonce+1)
	}

	// Ensure there's no existing contract already at the designated address.
	if evm.StateDB.ContractExists(address) {
		if evm.Config.Tracer != nil && evm.Config.Tracer.OnGasChange != nil {
			evm.Config.Tracer.OnGasChange(gas, 0, tracing.GasChangeCallFailedExecution)
		}
		return nil, types.Address{}, 0, ErrContractAddressCollision
	}
	// Create a new account on the state only if the object was not present.
	// It might be possible the contract code is deployed to a pre-existent
	// account with non-zero balance.
	snapshot := evm.StateDB.Snapshot()
	if !evm.StateDB.Exist(address) {
		evm.StateDB.CreateAccount(address)
	}
	// CreateContract means that regardless of whether the account previously existed
	// in the state trie or not, it _now_ becomes created as a _contract_ account.
	// This is performed _prior_ to executing the initcode,  since the initcode
	// acts inside that account.
	evm.StateDB.CreateContract(address)

	evm.StateDB.SetSeqno(address, 1)
	transfer(evm.StateDB, caller.Address(), address, value)

	// Initialise a new contract and set the code that is to be used by the EVM.
	// The contract is a scoped environment for this execution context only.
	contract := NewContract(caller, AccountRef(address), value, gas)
	contract.SetCallCode(&address, codeAndHash.Hash(), codeAndHash)
	contract.IsDeployment = true

	if err == nil {
		ret, err = evm.interpreter.Run(contract, nil, false)
	}

	// Check whether the max code size has been exceeded (EIP-158)
	if err == nil && len(ret) > params.MaxCodeSize {
		err = ErrMaxCodeSizeExceeded
	}

	// Reject code starting with 0xEF (EIP-3541)
	if err == nil && len(ret) >= 1 && ret[0] == 0xEF {
		err = ErrInvalidCode
	}

	if err == nil {
		// TODO: calculate gas required to store the code
		evm.StateDB.SetCode(address, ret)
	}

	// When an error was returned by the EVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally,
	// this also counts for code storage gas errors.
	if err != nil {
		evm.StateDB.RevertToSnapshot(snapshot)
		if !errors.Is(err, ErrExecutionReverted) {
			contract.UseGas(contract.Gas, evm.Config.Tracer, tracing.GasChangeCallFailedExecution)
		}
	}

	return ret, address, contract.Gas, err
}

// Deploy deploys a new contract from a deployment message
func (evm *EVM) Deploy(addr types.Address, caller ContractRef, code []byte, gas uint64, value *uint256.Int) (ret []byte, deployAddr types.Address, leftOverGas uint64, err error) {
	return evm.create(caller, code, gas, value, addr)
}

// Create creates a new contract using code as deployment code.
func (evm *EVM) Create(caller ContractRef, code []byte, gas uint64, value *uint256.Int) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error) {
	contractAddr = types.CreateAddress(evm.Origin.ShardId(), code)
	return evm.create(caller, types.Code(code), gas, value, contractAddr)
}

// Create2 creates a new contract using code as deployment code.
//
// The different between Create2 with Create is Create2 uses keccak256(0xff ++ msg.sender ++ salt ++ keccak256(init_code))[12:]
// instead of the usual sender-and-nonce-hash as the address where the contract is initialized at.
func (evm *EVM) Create2(caller ContractRef, code []byte, gas uint64, endowment *uint256.Int, salt *uint256.Int) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error) {
	contractAddr = types.CreateAddressWithSalt(caller.Address().ShardId(), code, common.BytesToHash(salt.Bytes()))
	return evm.create(caller, types.Code(code), gas, endowment, contractAddr)
}

// canTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func canTransfer(db StateDB, addr types.Address, amount *uint256.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// transfer subtracts amount from sender and adds amount to recipient using the given Db
func transfer(db StateDB, sender, recipient types.Address, amount *uint256.Int) {
	db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
	db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}
