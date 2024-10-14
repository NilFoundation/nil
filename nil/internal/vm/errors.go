// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
)

// List evm execution errors
var (
	ErrOutOfGas                 = types.NewVmError(types.ErrorOutOfGas)
	ErrCodeStoreOutOfGas        = types.NewVmError(types.ErrorCodeStoreOutOfGas)
	ErrDepth                    = types.NewVmError(types.ErrorCallDepthExceeded)
	ErrInsufficientBalance      = types.NewVmError(types.ErrorInsufficientBalance)
	ErrContractAddressCollision = types.NewVmError(types.ErrorContractAddressCollision)
	ErrExecutionReverted        = types.NewVmError(types.ErrorExecutionReverted)
	ErrMaxCodeSizeExceeded      = types.NewVmError(types.ErrorMaxCodeSizeExceeded)
	ErrMaxInitCodeSizeExceeded  = types.NewVmError(types.ErrorMaxInitCodeSizeExceeded)
	ErrInvalidJump              = types.NewVmError(types.ErrorInvalidJump)
	ErrWriteProtection          = types.NewVmError(types.ErrorWriteProtection)
	ErrReturnDataOutOfBounds    = types.NewVmError(types.ErrorReturnDataOutOfBounds)
	ErrGasUintOverflow          = types.NewVmError(types.ErrorGasUintOverflow)
	ErrInvalidCode              = types.NewVmError(types.ErrorInvalidCode)
	ErrNonceUintOverflow        = types.NewVmError(types.ErrorNonceUintOverflow)
	ErrInvalidInputLength       = types.NewVmError(types.ErrorInvalidInputLength)
	ErrCrossShardMessage        = types.NewVmError(types.ErrorCrossShardMessage)
	ErrUnexpectedPrecompileType = types.NewVmError(types.ErrorUnexpectedPrecompileType)
	ErrMessageToMainShard       = types.NewVmError(types.ErrorMessageToMainShard)

	// errStopToken is an internal token indicating interpreter loop termination,
	// never returned to outside callers.
	errStopToken = types.NewVmError(types.ErrorStopToken)
)

// StackUnderflowError happens when the items on the stack less
// than the minimal requirement.
func StackUnderflowError(stackLen int, required int, op OpCode) error {
	return types.NewVmVerboseError(types.ErrorStackUnderflow,
		fmt.Sprintf("stack:%d < required:%d, opcode: %s", stackLen, required, op))
}

// StackOverflowError happens when the items on the stack exceeds
// the maximum allowance.
func StackOverflowError(stackLen int, limit int, op OpCode) error {
	return types.NewVmVerboseError(types.ErrorStackOverflow,
		fmt.Sprintf("stack: %d, limit: %d, opcode: %s", stackLen, limit, op))
}

// InvalidOpCodeError happens when an invalid opcode is encountered.
func InvalidOpCodeError(op OpCode) error {
	return types.NewVmVerboseError(types.ErrorInvalidOpcode, fmt.Sprintf("invalid opcode: %s", op))
}
