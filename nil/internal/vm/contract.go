package vm

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/holiman/uint256"
)

// ContractRef is a reference to the contract's backing object
type ContractRef interface {
	Address() types.Address
	Token() []types.TokenBalance
}

// AccountRef implements ContractRef.
//
// Account references are used during EVM initialisation and
// its primary use is to fetch addresses. Removing this object
// proves difficult because of the cached jump destinations which
// are fetched from the parent contract (i.e. the caller), which
// is a ContractRef.
type AccountRef types.Address

// Address casts AccountRef to an Address
func (ar AccountRef) Address() types.Address { return (types.Address)(ar) }

func (ar AccountRef) Token() []types.TokenBalance {
	return nil
}

// Contract represents an ethereum contract in the state database. It contains
// the contract code, calling arguments. Contract implements ContractRef
type Contract struct {
	// CallerAddress is the result of the caller which initialised this
	// contract. However when the "call method" is delegated this value
	// needs to be initialised to that of the caller's caller.
	CallerAddress types.Address
	caller        ContractRef
	self          ContractRef

	jumpdests map[common.Hash]bitvec // Aggregated result of JUMPDEST analysis.
	analysis  bitvec                 // Locally cached result of JUMPDEST analysis

	Code     []byte
	CodeHash common.Hash
	CodeAddr types.Address
	Input    []byte

	Gas   uint64
	value *uint256.Int
	token []types.TokenBalance
}

// NewContract returns a new contract environment for the execution of EVM.
func NewContract(
	caller ContractRef,
	object ContractRef,
	value *uint256.Int,
	gas uint64,
	token []types.TokenBalance,
) *Contract {
	c := &Contract{CallerAddress: caller.Address(), caller: caller, self: object, token: token}

	if parent, ok := caller.(*Contract); ok {
		// Reuse JUMPDEST analysis from parent context if available.
		c.jumpdests = parent.jumpdests
	} else {
		c.jumpdests = make(map[common.Hash]bitvec)
	}

	// Gas should be a pointer so it can safely be reduced through the run
	// This pointer will be off the state transition
	c.Gas = gas
	// ensures a value is set
	c.value = value

	return c
}

func (c *Contract) validJumpdest(dest *uint256.Int) bool {
	udest, overflow := dest.Uint64WithOverflow()
	// PC cannot go beyond len(code) and certainly can't be bigger than 63bits.
	// Don't bother checking for JUMPDEST in that case.
	if overflow || udest >= uint64(len(c.Code)) {
		return false
	}
	// Only JUMPDESTs allowed for destinations
	if OpCode(c.Code[udest]) != JUMPDEST {
		return false
	}
	return c.isCode(udest)
}

// isCode returns true if the provided PC location is an actual opcode, as
// opposed to a data-segment following a PUSHN operation.
func (c *Contract) isCode(udest uint64) bool {
	// Do we already have an analysis laying around?
	if c.analysis != nil {
		return c.analysis.codeSegment(udest)
	}
	// Do we have a contract hash already?
	// If we do have a hash, that means it's a 'regular' contract. For regular
	// contracts ( not temporary initcode), we store the analysis in a map
	if c.CodeHash != (common.Hash{}) {
		// Does parent context have the analysis?
		analysis, exist := c.jumpdests[c.CodeHash]
		if !exist {
			// Do the analysis and save in parent context
			// We do not need to store it in c.analysis
			analysis = codeBitmap(c.Code)
			c.jumpdests[c.CodeHash] = analysis
		}
		// Also stash it in current contract for faster access
		c.analysis = analysis
		return analysis.codeSegment(udest)
	}
	// We don't have the code hash, most likely a piece of initcode not already
	// in state trie. In that case, we do an analysis, and save it locally, so
	// we don't have to recalculate it for every JUMP instruction in the execution
	// However, we don't save it within the parent context
	if c.analysis == nil {
		c.analysis = codeBitmap(c.Code)
	}
	return c.analysis.codeSegment(udest)
}

// AsDelegate sets the contract to be a delegate call and returns the current
// contract (for chaining calls)
func (c *Contract) AsDelegate() *Contract {
	// NOTE: caller must at all times be a contract. It should never happen
	// that caller is something other than a Contract.
	parent, ok := c.caller.(*Contract)
	check.PanicIfNot(ok)

	c.CallerAddress = parent.CallerAddress
	c.value = parent.value
	c.token = parent.token

	return c
}

// GetOp returns the n'th element in the contract's byte array
func (c *Contract) GetOp(n uint64) OpCode {
	if n < uint64(len(c.Code)) {
		return OpCode(c.Code[n])
	}

	return STOP
}

// Caller returns the caller of the contract.
//
// Caller will recursively call caller when the contract is a delegate
// call, including that of caller's caller.
func (c *Contract) Caller() types.Address {
	return c.CallerAddress
}

func (c *Contract) Token() []types.TokenBalance {
	return c.token
}

// UseGas attempts to use gas and subtracts it and returns true on success
func (c *Contract) UseGas(gas uint64, logger *tracing.Hooks, reason tracing.GasChangeReason) (ok bool) {
	if c.Gas < gas {
		return false
	}
	if logger != nil && logger.OnGasChange != nil && reason != tracing.GasChangeIgnored {
		logger.OnGasChange(c.Gas, c.Gas-gas, reason)
	}
	c.Gas -= gas
	return true
}

// RefundGas refunds gas to the contract
func (c *Contract) RefundGas(gas uint64, logger *tracing.Hooks, reason tracing.GasChangeReason) {
	if gas == 0 {
		return
	}
	if logger != nil && logger.OnGasChange != nil && reason != tracing.GasChangeIgnored {
		logger.OnGasChange(c.Gas, c.Gas+gas, reason)
	}
	c.Gas += gas
}

// Address returns the contracts address
func (c *Contract) Address() types.Address {
	return c.self.Address()
}

// Value returns the contract's value (sent to it from its caller)
func (c *Contract) Value() *uint256.Int {
	return c.value
}

// SetCallCode sets the code of the contract and address of the backing data
// object
func (c *Contract) SetCallCode(addr types.Address, hash common.Hash, code []byte) {
	c.Code = code
	c.CodeHash = hash
	c.CodeAddr = addr
}
