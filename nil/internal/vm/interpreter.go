package vm

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/math"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/holiman/uint256"
)

// Config are the configuration options for the Interpreter
type Config struct {
	Tracer                  *tracing.Hooks
	NoBaseFee               bool  // Forces the EIP-1559 baseFee to 0 (needed for 0 price calls)
	EnablePreimageRecording bool  // Enables recording of SHA3/keccak preimages
	ExtraEips               []int // Additional EIPs that are to be enabled
}

// ScopeContext contains the things that are per-call, such as stack and memory,
// but not transients like pc and gas
type ScopeContext struct {
	Memory   *Memory
	Stack    *Stack
	Contract *Contract
}

type EvmRestoreData struct {
	EvmState types.EvmState

	ReturnData []byte
	Result     bool
}

// MemoryData returns the underlying memory slice. Callers must not modify the contents
// of the returned data.
func (ctx *ScopeContext) MemoryData() []byte {
	if ctx.Memory == nil {
		return nil
	}
	return ctx.Memory.Data()
}

// StackData returns the stack data. Callers must not modify the contents
// of the returned data.
func (ctx *ScopeContext) StackData() []uint256.Int {
	if ctx.Stack == nil {
		return nil
	}
	return ctx.Stack.Data()
}

// Caller returns the current caller.
func (ctx *ScopeContext) Caller() types.Address {
	return ctx.Contract.Caller()
}

// Address returns the address where this scope of execution is taking place.
func (ctx *ScopeContext) Address() types.Address {
	return ctx.Contract.Address()
}

// CallValue returns the value supplied with this call.
func (ctx *ScopeContext) CallValue() *uint256.Int {
	return ctx.Contract.Value()
}

// CallInput returns the input/calldata with this call. Callers must not modify
// the contents of the returned data.
func (ctx *ScopeContext) CallInput() []byte {
	return ctx.Contract.Input
}

// Address returns the address where this scope of execution is taking place. Callers must not modify the contents
func (ctx *ScopeContext) Code() []byte {
	return ctx.Contract.Code
}

// EVMInterpreter represents an EVM interpreter
type EVMInterpreter struct {
	evm   *EVM
	table *JumpTable
	// stopAndDumpState indicates that Interpreter should stop after current instruction and dump the state.
	stopAndDumpState      bool
	continuationGasCredit types.Gas
	// restoredState is the state to restore the EVM from a specific point of execution.
	restoredState *EvmRestoreData

	readOnly   bool   // Whether to throw on stateful modifications
	returnData []byte // Last CALL's return data for subsequent reuse
}

func NewEVMInterpreter(evm *EVM, state *EvmRestoreData) *EVMInterpreter {
	return &EVMInterpreter{evm: evm, table: &CancunInstructionSet, restoredState: state}
}

// Run loops and evaluates the contract's code with the given input data and returns
// the return byte-slice and an error if one occurred.
//
// It's important to note that any errors returned by the interpreter should be
// considered a revert-and-consume-all-gas operation except for
// ErrExecutionReverted which means revert-and-keep-gas-left.
func (in *EVMInterpreter) Run(contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	// Increment the call depth which is restricted to 1024
	in.evm.depth++
	defer func() { in.evm.depth-- }()

	// Make sure the readOnly is only set if we aren't in readOnly yet.
	// This also makes sure that the readOnly flag isn't removed for child calls.
	if readOnly && !in.readOnly {
		in.readOnly = true
		defer func() { in.readOnly = false }()
	}

	// Reset the previous call's return data. It's unimportant to preserve the old buffer
	// as every returning call will return new data anyway.
	in.returnData = nil

	// Don't bother with the execution if there's no code.
	if len(contract.Code) == 0 {
		return nil, nil
	}

	var (
		op          OpCode        // current opcode
		mem         = NewMemory() // bound memory
		stack       = newStack()  // local stack
		callContext = &ScopeContext{
			Memory:   mem,
			Stack:    stack,
			Contract: contract,
		}
		// For optimisation reason we're using uint64 as the program counter.
		// It's theoretically possible to go above 2^64. The YP defines the PC
		// to be uint256. Practically much less so feasible.
		pc   = uint64(0) // program counter
		cost uint64
		// copies used by tracer
		pcCopy    uint64 // needed for the deferred EVMLogger
		gasCopy   uint64 // for EVMLogger to log gas remaining before execution
		logged    bool   // deferred EVMLogger should ignore already logged steps
		res       []byte // result of the opcode execution function
		hasTracer = in.evm.Config.Tracer != nil
	)

	if in.restoredState != nil {
		pc = in.restoredState.EvmState.Pc
		memorySize := uint64(len(in.restoredState.EvmState.Memory))
		mem.Resize(memorySize)
		mem.Set(0, memorySize, in.restoredState.EvmState.Memory)
		if err = callContext.Stack.FromBytes(in.restoredState.EvmState.Stack); err != nil {
			return nil, err
		}

		// Push 1 to the stack to indicate that the EVM call instruction was successful
		callContext.Stack.Data()[stack.len()-1].SetOne()

		// Encode return data, which has the following layout:
		// 1. Offset to the data (3. point)
		// 2. Boolean flag indicating whether the call was successful
		// 3. Length of the data
		// 4. The data itself
		length := uint256.NewInt(uint64(len(in.restoredState.ReturnData))).Bytes32()
		offset := uint256.NewInt(64).Bytes32()
		in.returnData = make([]byte, 0, len(in.restoredState.ReturnData)+64)
		in.returnData = append(in.returnData, offset[:]...)
		if in.restoredState.Result {
			offset = uint256.NewInt(1).Bytes32()
		} else {
			offset = uint256.NewInt(0).Bytes32()
		}
		in.returnData = append(in.returnData, offset[:]...)
		in.returnData = append(in.returnData, length[:]...)
		in.returnData = append(in.returnData, in.restoredState.ReturnData...)
	}

	// Don't move this deferred function, it's placed before the OnOpcode-deferred method,
	// so that it gets executed _after_: the OnOpcode needs the stacks before
	// they are returned to the pools
	defer func() {
		returnStack(stack)
	}()
	contract.Input = input

	if hasTracer {
		defer func() { // this deferred method handles exit-with-error
			if err == nil {
				return
			}
			if !logged && in.evm.Config.Tracer.OnOpcode != nil {
				in.evm.Config.Tracer.OnOpcode(
					pcCopy, byte(op), gasCopy, cost, callContext, in.returnData, in.evm.depth, err)
			}
			if logged && in.evm.Config.Tracer.OnFault != nil {
				in.evm.Config.Tracer.OnFault(pcCopy, byte(op), gasCopy, cost, callContext, in.evm.depth, err)
			}
		}()
	}
	// The Interpreter main run loop (contextual). This loop runs until either an
	// explicit STOP, RETURN or SELFDESTRUCT is executed, an error occurred during
	// the execution of one of the operations, or until the done flag is set by the
	// parent context.
	for {
		if hasTracer {
			// Capture pre-execution values for tracing.
			logged, pcCopy, gasCopy = false, pc, contract.Gas
		}
		// Get the operation from the jump table and validate the stack to ensure there are
		// enough stack items available to perform the operation.
		op = contract.GetOp(pc)
		operation := in.table[op]
		cost = operation.constantGas // For tracing

		// Validate stack
		if sLen := stack.len(); sLen < operation.minStack {
			return nil, StackUnderflowError(sLen, operation.minStack, op)
		} else if sLen > operation.maxStack {
			return nil, StackOverflowError(sLen, operation.maxStack, op)
		}
		if !contract.UseGas(cost, in.evm.Config.Tracer, tracing.GasChangeIgnored) {
			return nil, ErrOutOfGas
		}

		// All ops with a dynamic memory usage also have a dynamic gas cost.
		var memorySize uint64
		if operation.dynamicGas != nil {
			memSize, dynamicCost, err := calcDynamicCosts(contract, operation, stack, in, mem)
			if err != nil {
				return nil, err
			}

			memorySize = memSize
			cost += dynamicCost
		}

		// Do tracing before memory expansion
		if hasTracer {
			if in.evm.Config.Tracer.OnGasChange != nil {
				in.evm.Config.Tracer.OnGasChange(gasCopy, gasCopy-cost, tracing.GasChangeCallOpCode)
			}
			if in.evm.Config.Tracer.OnOpcode != nil {
				in.evm.Config.Tracer.OnOpcode(
					pc, byte(op), gasCopy, cost, callContext, in.returnData, in.evm.depth, err)
				logged = true
			}
		}

		if memorySize > 0 {
			mem.Resize(memorySize)
		}

		// execute the operation
		res, err = operation.execute(&pc, in, callContext)

		if in.stopAndDumpState {
			// Save current VM state
			state := types.EvmState{
				Memory: callContext.Memory.Data(),
				Stack:  callContext.Stack.CopyToBytes(),
				Pc:     pc + 1,
			}
			if err = in.evm.StateDB.SaveVmState(&state, in.continuationGasCredit); err != nil {
				return nil, err
			}
			break
		}
		if err != nil {
			break
		}
		pc++
	}

	if errors.Is(err, errStopToken) {
		err = nil // clear stop token error
	} else {
		in.evm.DebugInfo = &DebugInfo{Pc: pc}
	}

	return res, err
}

func calcDynamicCosts(
	contract *Contract,
	operation *operation,
	stack *Stack,
	in *EVMInterpreter,
	mem *Memory,
) (uint64, uint64, error) {
	// Calculate the new memory size and expand the memory to fit the operation.
	// Memory check needs to be done prior to evaluating the dynamic gas portion
	// to detect calculation overflows.
	var memorySize uint64
	if operation.memorySize != nil {
		memSize, overflow := operation.memorySize(stack)
		if overflow {
			return 0, 0, ErrGasUintOverflow
		}
		// memory is expanded in words of 32 bytes. Gas
		// is also calculated in words.
		if memorySize, overflow = math.SafeMul(toWordSize(memSize), 32); overflow {
			return 0, 0, ErrGasUintOverflow
		}
	}

	// Consume the gas and return an error if not enough gas is available.
	// Cost is explicitly set so that the capture state defer method can get the proper cost.
	dynamicCost, err := operation.dynamicGas(in.evm, contract, stack, mem, memorySize)
	if err != nil {
		return 0, 0, types.NewWrapError(types.ErrorOutOfGasDynamic, err)
	}
	if !contract.UseGas(dynamicCost, in.evm.Config.Tracer, tracing.GasChangeIgnored) {
		return 0, 0, types.NewVerboseError(types.ErrorOutOfGasDynamic,
			fmt.Sprintf("%d < %d", contract.Gas, dynamicCost))
	}
	return memorySize, dynamicCost, nil
}
