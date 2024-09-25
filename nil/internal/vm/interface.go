package vm

import (
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type StateDBReadOnly interface {
	// IsInternalMessage returns true if the message that initiated execution is internal.
	// Synchronous calls inside one contract are also considered internal.
	IsInternalMessage() bool

	GetCurrencies(types.Address) map[types.CurrencyId]types.Value
	GetGasPrice(types.ShardId) (types.Value, error)
}

type StateDB interface {
	StateDBReadOnly

	CreateAccount(types.Address) error
	CreateContract(types.Address) error

	SubBalance(types.Address, types.Value, tracing.BalanceChangeReason) error
	AddBalance(types.Address, types.Value, tracing.BalanceChangeReason) error
	GetBalance(types.Address) (types.Value, error)

	AddCurrency(to types.Address, currencyId types.CurrencyId, amount types.Value) error
	SubCurrency(to types.Address, currencyId types.CurrencyId, amount types.Value) error
	SetCurrencyTransfer([]types.CurrencyBalance)

	GetSeqno(types.Address) (types.Seqno, error)
	SetSeqno(types.Address, types.Seqno) error

	GetCode(types.Address) ([]byte, common.Hash, error)
	SetCode(types.Address, []byte) error

	AddRefund(uint64)
	SubRefund(uint64)
	GetRefund() uint64

	GetCommittedState(types.Address, common.Hash) common.Hash
	GetState(types.Address, common.Hash) (common.Hash, error)
	SetState(types.Address, common.Hash, common.Hash) error
	GetStorageRoot(addr types.Address) (common.Hash, error)

	GetTransientState(addr types.Address, key common.Hash) common.Hash
	SetTransientState(addr types.Address, key, value common.Hash)

	HasSelfDestructed(types.Address) (bool, error)

	Selfdestruct6780(types.Address) error

	// Exist reports whether the given account exists in state.
	// Notably this should also return true for self-destructed accounts.
	Exists(types.Address) (bool, error)
	// Empty returns whether the given account is empty. Empty
	// is defined according to EIP161 (balance = nonce = code = 0).
	Empty(types.Address) (bool, error)
	// ContractExists is used to check whether we can deploy to an address
	ContractExists(types.Address) (bool, error)

	AddressInAccessList(addr types.Address) bool
	SlotInAccessList(addr types.Address, slot common.Hash) (addressOk bool, slotOk bool)
	// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
	// even if the feature/fork is not active yet
	AddAddressToAccessList(addr types.Address)
	// AddSlotToAccessList adds the given (address, slot) to the access list. This operation is safe to perform
	// even if the feature/fork is not active yet
	AddSlotToAccessList(addr types.Address, slot common.Hash)

	// Prepare(sender, coinbase common.Address, dest *common.Address, precompiles []common.Address)

	RevertToSnapshot(int)
	Snapshot() int

	AddLog(*types.Log)

	// AddOutMessage adds internal out message for current transaction
	AddOutMessage(caller types.Address, payload *types.InternalMessagePayload) (*types.Message, error)

	// AddOutRequestMessage adds outbound request message for current transaction
	AddOutRequestMessage(
		caller types.Address,
		payload *types.InternalMessagePayload,
		responseProcessingGas types.Gas,
		isAwait bool,
	) (*types.Message, error)

	// Get current message
	GetInMessage() *types.Message

	// Get execution context shard id
	GetShardID() types.ShardId

	// SaveVmState saves current VM state
	SaveVmState(state *types.EvmState, continuationGasCredit types.Gas) error

	GetConfigAccessor() *config.ConfigAccessor
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
