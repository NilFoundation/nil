package vm

import (
	"math/big"
	"sync/atomic"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/params"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog/log"
)

type (
	// CanTransferFunc is the signature of a transfer guard function
	CanTransferFunc func(StateDB, common.Address, *uint256.Int) bool
	// TransferFunc is the signature of a transfer function
	TransferFunc func(StateDB, common.Address, common.Address, *uint256.Int)
	// GetHashFunc returns the n'th block hash in the blockchain
	// and is used by the BLOCKHASH EVM op code.
	GetHashFunc func(uint64) common.Hash
)

// BlockContext provides the EVM with auxiliary information. Once provided
// it shouldn't be modified.
type BlockContext struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers ether from one account to the other
	Transfer TransferFunc
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    uint64         // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        uint64         // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY
	BaseFee     *big.Int       // Provides information for BASEFEE (0 if vm runs with NoBaseFee flag and 0 gas price)
	BlobBaseFee *big.Int       // Provides information for BLOBBASEFEE (0 if vm runs with NoBaseFee flag and 0 blob gas price)
	Random      *common.Hash   // Provides information for PREVRANDAO
}

// TxContext provides the EVM with information about a transaction.
// All fields can change between transactions.
type TxContext struct {
	// Message information
	Origin     common.Address // Provides information for ORIGIN
	GasPrice   *big.Int       // Provides information for GASPRICE (and is used to zero the basefee if NoBaseFee is set)
	BlobHashes []common.Hash  // Provides information for BLOBHASH
	BlobFeeCap *big.Int       // Is used to zero the blobbasefee if NoBaseFee is set
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

	// abort is used to abort the EVM calling operations
	abort atomic.Bool
	// callGasTemp holds the gas available for the current call. This is needed because the
	// available gas is calculated in gasCall* according to the 63/64 rule and later
	// applied in opCall*.
	callGasTemp uint64
}

// Call executes the contract associated with the addr with the given input as
// parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
func (evm *EVM) Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	panic("CALL not implemented")
}

// CallCode executes the contract associated with the addr with the given input
// as parameters. It also handles any necessary value transfer required and takes
// the necessary steps to create accounts and reverses the state in case of an
// execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address'
// code with the caller as context.
func (evm *EVM) CallCode(caller ContractRef, addr common.Address, input []byte, gas uint64, value *uint256.Int) (ret []byte, leftOverGas uint64, err error) {
	panic("CALLCODE not implemented")
}

// DelegateCall executes the contract associated with the addr with the given input
// as parameters. It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address'
// code with the caller as context and the caller is set to the caller of the caller.
func (evm *EVM) DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	panic("DELEGATECALL not implemented")
}

// StaticCall executes the contract associated with the addr with the given input
// as parameters while disallowing any modifications to the state during the call.
// Opcodes that attempt to perform such modifications will result in exceptions
// instead of performing the modifications.
func (evm *EVM) StaticCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	panic("STATICCALL not implemented")
}

// Create creates a new contract using code as deployment code.
func (evm *EVM) Create(caller ContractRef, code []byte, gas uint64, value *uint256.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	panic("CREATE not implemented")
}

// Create2 creates a new contract using code as deployment code.
//
// The different between Create2 with Create is Create2 uses keccak256(0xff ++ msg.sender ++ salt ++ keccak256(init_code))[12:]
// instead of the usual sender-and-nonce-hash as the address where the contract is initialized at.
func (evm *EVM) Create2(caller ContractRef, code []byte, gas uint64, endowment *uint256.Int, salt *uint256.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	panic("CREATE2 not implemented")
}

type Facade struct {
	state *execution.ExecutionState
}

// AddAddressToAccessList implements StateDB.
func (f *Facade) AddAddressToAccessList(addr common.Address) {
	panic("unimplemented")
}

// AddBalance implements StateDB.
func (f *Facade) AddBalance(common.Address, *uint256.Int, tracing.BalanceChangeReason) {
	panic("unimplemented")
}

// AddLog implements StateDB.
func (f *Facade) AddLog(*types.Log) {
	panic("unimplemented")
}

// AddRefund implements StateDB.
func (f *Facade) AddRefund(uint64) {
	panic("unimplemented")
}

// AddSlotToAccessList implements StateDB.
func (f *Facade) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	panic("unimplemented")
}

// AddressInAccessList implements StateDB.
func (f *Facade) AddressInAccessList(addr common.Address) bool {
	panic("unimplemented")
}

// Empty implements StateDB.
func (f *Facade) Empty(common.Address) bool {
	panic("unimplemented")
}

// Exist implements StateDB.
func (f *Facade) Exist(common.Address) bool {
	panic("unimplemented")
}

// GetBalance implements StateDB.
func (f *Facade) GetBalance(common.Address) *uint256.Int {
	panic("unimplemented")
}

// GetCode implements StateDB.
func (f *Facade) GetCode(common.Address) []byte {
	panic("unimplemented")
}

// GetCodeHash implements StateDB.
func (f *Facade) GetCodeHash(common.Address) common.Hash {
	panic("unimplemented")
}

// GetCodeSize implements StateDB.
func (f *Facade) GetCodeSize(common.Address) int {
	panic("unimplemented")
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (f *Facade) GetCommittedState(common.Address, common.Hash) common.Hash {
	return common.Hash{}
}

// GetRefund implements StateDB.
func (f *Facade) GetRefund() uint64 {
	panic("unimplemented")
}

// GetTransientState implements StateDB.
func (f *Facade) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	panic("unimplemented")
}

// HasSelfDestructed implements StateDB.
func (f *Facade) HasSelfDestructed(common.Address) bool {
	panic("unimplemented")
}

// SelfDestruct implements StateDB.
func (f *Facade) SelfDestruct(common.Address) {
	panic("unimplemented")
}

// Selfdestruct6780 implements StateDB.
func (f *Facade) Selfdestruct6780(common.Address) {
	panic("unimplemented")
}

// SetCode implements StateDB.
func (f *Facade) SetCode(common.Address, []byte) {
	panic("unimplemented")
}

// SetTransientState implements StateDB.
func (f *Facade) SetTransientState(addr common.Address, key common.Hash, value common.Hash) {
	panic("unimplemented")
}

func (f *Facade) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return true, true // FIXME
}

// SubBalance implements StateDB.
func (f *Facade) SubBalance(common.Address, *uint256.Int, tracing.BalanceChangeReason) {
	panic("unimplemented")
}

// SubRefund implements StateDB.
func (f *Facade) SubRefund(uint64) {
	panic("unimplemented")
}

func NewStateDB(state *execution.ExecutionState) StateDB {
	return &Facade{state}
}

func (f *Facade) GetState(addr common.Address, key common.Hash) common.Hash {
	val, err := f.state.GetState(addr, key)
	log.Debug().Msgf("get state of contract %s at %v: %v", addr, key, val)
	if err == db.ErrKeyNotFound {
		return common.EmptyHash
	}
	if err != nil {
		panic(err)
	}
	return val.Bytes32()
}

func (f *Facade) SetState(addr common.Address, key common.Hash, val common.Hash) {
	log.Debug().Msgf("set state of contract %s at %v: %v", addr, key, val)
	err := f.state.SetState(addr, key, *uint256.MustFromBig(val.Big()))
	if err != nil {
		panic(err)
	}
}

func (f *Facade) GetStorageRoot(addr common.Address) common.Hash {
	return common.Hash{}
}
