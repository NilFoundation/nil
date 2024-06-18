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
)

// every execution logic error should inherit this
type VMError struct {
	err string
}

func (e VMError) Error() string {
	return e.err
}

// List evm execution errors
var (
	ErrOutOfGas                 = VMError{"out of gas"}
	ErrCodeStoreOutOfGas        = VMError{"contract creation code storage out of gas"}
	ErrDepth                    = VMError{"max call depth exceeded"}
	ErrInsufficientBalance      = VMError{"insufficient balance for transfer"}
	ErrContractAddressCollision = VMError{"contract address collision"}
	ErrExecutionReverted        = VMError{"execution reverted"}
	ErrMaxCodeSizeExceeded      = VMError{"max code size exceeded"}
	ErrMaxInitCodeSizeExceeded  = VMError{"max initcode size exceeded"}
	ErrInvalidJump              = VMError{"invalid jump destination"}
	ErrWriteProtection          = VMError{"write protection"}
	ErrReturnDataOutOfBounds    = VMError{"return data out of bounds"}
	ErrGasUintOverflow          = VMError{"gas uint64 overflow"}
	ErrInvalidCode              = VMError{"invalid code: must not begin with 0xef"}
	ErrNonceUintOverflow        = VMError{"nonce uint64 overflow"}

	ErrInvalidInputLength = VMError{"invalid input length"}

	// errStopToken is an internal token indicating interpreter loop termination,
	// never returned to outside callers.
	errStopToken = VMError{"stop token"}
)

// StackUnderflowError happens when the items on the stack less
// than the minimal requirement.
func StackUnderflowError(stackLen int, required int, op OpCode) VMError {
	return VMError{fmt.Sprintf("stack underflow (%d <=> %d) [%s]", stackLen, required, op)}
}

// StackOverflowError happens when the items on the stack exceeds
// the maximum allowance.
func StackOverflowError(stackLen int, limit int, op OpCode) VMError {
	return VMError{fmt.Sprintf("stack limit reached %d (%d) [%s]", stackLen, limit, op)}
}

// InvalidOpCodeError happens when an invalid opcode is encountered.
func InvalidOpCodeError(op OpCode) VMError {
	return VMError{fmt.Sprintf("invalid opcode: %s", op)}
}
