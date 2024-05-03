package vm

import (
	"errors"
)

// EVMInterpreter represents an EVM interpreter
type EVMInterpreter struct {
	evm *EVM
}

func NewEVMInterpreter(evm *EVM) *EVMInterpreter {
	return &EVMInterpreter{evm: evm}
}

var (
	OutOfGas = errors.New("out of gas")
)

func (in *EVMInterpreter) Run(contract *Contract, input []byte, readOnly bool) (ret []byte, err error) {
	return nil, OutOfGas
}
