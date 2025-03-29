package core

import (
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Contract struct {
	Address types.Address
	Abi     *abi.ABI
}

func NewContract(name string, address types.Address) (*Contract, error) {
	abi, err := contracts.GetAbi(name)
	if err != nil {
		return nil, err
	}
	return &Contract{Abi: abi, Address: address}, nil
}

func (c *Contract) PackCallData(method string, args ...any) ([]byte, error) {
	return c.Abi.Pack(method, args...)
}
