package vm

import (
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

type StateDB interface {
	CreateAccount(types.Address)
	CreateContract(types.Address)

	SubBalance(types.Address, *uint256.Int, tracing.BalanceChangeReason)
	AddBalance(types.Address, *uint256.Int, tracing.BalanceChangeReason)
	GetBalance(types.Address) *uint256.Int

	GetSeqno(types.Address) types.Seqno
	SetSeqno(types.Address, types.Seqno)

	GetCodeHash(types.Address) common.Hash
	GetCode(types.Address) []byte
	SetCode(types.Address, []byte)
	GetCodeSize(types.Address) int

	AddRefund(uint64)
	SubRefund(uint64)
	GetRefund() uint64

	GetCommittedState(types.Address, common.Hash) common.Hash
	GetState(types.Address, common.Hash) common.Hash
	SetState(types.Address, common.Hash, common.Hash)
	GetStorageRoot(addr types.Address) common.Hash

	GetTransientState(addr types.Address, key common.Hash) common.Hash
	SetTransientState(addr types.Address, key, value common.Hash)

	HasSelfDestructed(types.Address) bool

	Selfdestruct6780(types.Address)

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for self-destructed accounts.
	Exist(types.Address) bool
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(types.Address) bool
	// ContractExists is used to check whether we can deploy to an address
	ContractExists(types.Address) bool

	AddressInAccessList(addr types.Address) bool
	SlotInAccessList(addr types.Address, slot common.Hash) (addressOk bool, slotOk bool)
	// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
	// even if the feature/fork is not active yet
	AddAddressToAccessList(addr types.Address)
	// AddSlotToAccessList adds the given (address,slot) to the access list. This operation is safe to perform
	// even if the feature/fork is not active yet
	AddSlotToAccessList(addr types.Address, slot common.Hash)

	// Prepare(sender, coinbase common.Address, dest *common.Address, precompiles []common.Address)

	RevertToSnapshot(int)
	Snapshot() int

	AddLog(*types.Log)

	// add out message for current transaction
	AddOutMessage(*types.Message)

	// IsInternalMessage returns true if the message that initiated execution is internal. Synchronous calls inside
	// one contract are also considered as internal.
	IsInternalMessage() bool

	// Get current message
	GetInMessage() *types.Message

	// Get execution context shard id
	GetShardID() types.ShardId
}

// CallContext provides a basic interface for the EVM calling conventions. The EVM
// depends on this context being implemented for doing subcalls and initialising new EVM contracts.
type CallContext interface {
	// Call calls another contract.
	Call(env *EVM, me ContractRef, addr types.Address, data []byte, gas, value *big.Int) ([]byte, error)
	// CallCode takes another contracts code and execute within our own context
	CallCode(env *EVM, me ContractRef, addr types.Address, data []byte, gas, value *big.Int) ([]byte, error)
	// DelegateCall is same as CallCode except sender and value is propagated from parent to child scope
	DelegateCall(env *EVM, me ContractRef, addr types.Address, data []byte, gas *big.Int) ([]byte, error)
	// Create creates a new contract
	Create(env *EVM, me ContractRef, data []byte, gas, value *big.Int) ([]byte, types.Address, error)
}
