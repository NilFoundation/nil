//go:build test

package contracts

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

const (
	FileNameWallet = "Wallet"

	FileNameCounter        = "tests/Counter"
	FileNameCounterPayable = "tests/CounterPayable"
	FileNameMessageCheck   = "tests/MessageCheck"
)

func CounterDeployPayload(t *testing.T) types.DeployPayload {
	t.Helper()

	code, err := GetCode(FileNameCounter)
	require.NoError(t, err)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

func CounterAddress(t *testing.T, shardId types.ShardId) types.Address {
	t.Helper()

	return types.CreateAddress(shardId, CounterDeployPayload(t))
}

func NewCallData(t *testing.T, fileName, methodName string, args ...any) []byte {
	t.Helper()

	abiCallee, err := GetAbi(fileName)
	require.NoError(t, err)
	callData, err := abiCallee.Pack(methodName, args...)
	require.NoError(t, err)

	return callData
}

func NewCounterAddCallData(t *testing.T, value int32) []byte {
	t.Helper()

	return NewCallData(t, FileNameCounter, "add", value)
}

func NewCounterGetCallData(t *testing.T) []byte {
	t.Helper()

	return NewCallData(t, FileNameCounter, "get")
}

func NewWalletSendCallData(t *testing.T,
	bytecode types.Code, gasLimit *types.Uint256, value *types.Uint256,
	currencies []types.CurrencyBalance, contractAddress types.Address, kind types.MessageKind,
) []byte {
	t.Helper()

	intMsg := &types.InternalMessagePayload{
		Data:     bytecode,
		To:       contractAddress,
		Value:    *value,
		GasLimit: *gasLimit,
		Currency: currencies,
		Kind:     kind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	require.NoError(t, err)

	return NewCallData(t, FileNameWallet, "send", intMsgData)
}

func CounterPayableDeployPayload(t *testing.T) types.DeployPayload {
	t.Helper()

	code, err := GetCode(FileNameCounterPayable)
	require.NoError(t, err)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

func NewCounterPayableAddCallData(t *testing.T, value int32) []byte {
	t.Helper()

	return NewCallData(t, FileNameCounterPayable, "add", value)
}

func NewCounterPayableGetCallData(t *testing.T) []byte {
	t.Helper()

	return NewCallData(t, FileNameCounterPayable, "get")
}
