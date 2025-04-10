//go:build test

package testaide

import (
	"context"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type CallContractMock struct {
	methodsReturnValue map[string][][]any
	abi                *abi.ABI
}

func NewCallContractMock(abi *abi.ABI) *CallContractMock {
	callContractMock := CallContractMock{abi: abi}
	callContractMock.Reset()
	return &callContractMock
}

func (c *CallContractMock) Reset() {
	c.methodsReturnValue = make(map[string][][]any)
}

type NoValue struct{}

func (c *CallContractMock) AddExpectedCall(methodName string, returnValues ...any) {
	c.methodsReturnValue[methodName] = append(c.methodsReturnValue[methodName], returnValues)
}

func (c *CallContractMock) CallContract(
	ctx context.Context,
	call ethereum.CallMsg,
	blockNumber *big.Int,
) ([]byte, error) {
	methodId := call.Data[:4]
	method, err := c.abi.MethodById(methodId)
	if err != nil {
		return nil, err
	}

	returnValuesSlice, ok := c.methodsReturnValue[method.Name]
	if !ok {
		return nil, fmt.Errorf("method not mocked: %s", method.Name)
	}

	if len(returnValuesSlice) == 0 {
		return nil, fmt.Errorf("not enough return values for method: %s", method.Name)
	}
	returnValues := returnValuesSlice[0]
	c.methodsReturnValue[method.Name] = returnValuesSlice[1:]

	if len(returnValues) == 1 {
		if _, ok := returnValues[0].(NoValue); ok {
			// If it's NoValue, call Pack with no arguments
			return method.Outputs.Pack()
		}
	}

	return method.Outputs.Pack(returnValues...)
}

func (c *CallContractMock) EverythingCalled() error {
	for methodName, returnValues := range c.methodsReturnValue {
		if len(returnValues) != 0 {
			return fmt.Errorf("not all calls were executed for %s", methodName)
		}
	}
	return nil
}
