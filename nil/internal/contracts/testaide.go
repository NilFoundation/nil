//go:build test

package contracts

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

const (
	NameCounter                    = "tests/Counter"
	NameDeployer                   = "tests/Deployer"
	NameDeployee                   = "tests/Deployee"
	NameTransactionCheck           = "tests/TransactionCheck"
	NameSender                     = "tests/Sender"
	NameTest                       = "tests/Test"
	NameTokensTest                 = "tests/TokensTest"
	NameTokensTestNoExternalAccess = "tests/TokensTestNoExternalAccess"
	NameRequestResponseTest        = "tests/RequestResponseTest"
	NamePrecompilesTest            = "tests/PrecompilesTest"
	NameConfigTest                 = "tests/ConfigTest"
	NameStresser                   = "tests/Stresser"
)

func GetDeployPayload(t *testing.T, name string) types.DeployPayload {
	t.Helper()

	code, err := GetCode(name)
	require.NoError(t, err)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

func CounterDeployPayload(t *testing.T) types.DeployPayload {
	t.Helper()

	return GetDeployPayload(t, NameCounter)
}

func CounterAddress(t *testing.T, shardId types.ShardId) types.Address {
	t.Helper()

	return types.CreateAddress(shardId, CounterDeployPayload(t))
}

func FaucetDeployPayload(t *testing.T) types.DeployPayload {
	t.Helper()

	return GetDeployPayload(t, NameFaucet)
}

func SmartAccountAddress(t *testing.T, shardId types.ShardId, salt, pubKey []byte) types.Address {
	t.Helper()

	res, err := CalculateAddress(NameSmartAccount, shardId, salt, pubKey)
	require.NoError(t, err)
	return res
}

func NewCallDataT(t *testing.T, fileName, methodName string, args ...any) []byte {
	t.Helper()

	callData, err := NewCallData(fileName, methodName, args...)
	require.NoError(t, err)

	return callData
}

func NewCounterAddCallData(t *testing.T, value int32) []byte {
	t.Helper()

	return NewCallDataT(t, NameCounter, "add", value)
}

func NewCounterGetCallData(t *testing.T) []byte {
	t.Helper()

	return NewCallDataT(t, NameCounter, "get")
}

func NewFaucetWithdrawToCallData(t *testing.T, dst types.Address, value types.Value) []byte {
	t.Helper()

	return NewCallDataT(t, NameFaucet, "withdrawTo", dst, value.ToBig())
}

func GetCounterValue(t *testing.T, data []byte) int32 {
	t.Helper()

	res, err := UnpackData(NameCounter, "get", data)
	require.NoError(t, err)

	val, ok := res[0].(int32)
	require.True(t, ok)
	return val
}

func NewSmartAccountSendCallData(t *testing.T,
	bytecode types.Code, gasLimit types.Gas, value types.Value,
	tokens []types.TokenBalance, contractAddress types.Address, kind types.TransactionKind,
) []byte {
	t.Helper()

	intTxn := &types.InternalTransactionPayload{
		Data:        bytecode,
		To:          contractAddress,
		Value:       value,
		FeeCredit:   gasLimit.ToValue(types.DefaultGasPrice),
		ForwardKind: types.ForwardKindNone,
		Token:       tokens,
		Kind:        kind,
	}

	intTxnData, err := intTxn.MarshalSSZ()
	require.NoError(t, err)

	return NewCallDataT(t, NameSmartAccount, "send", intTxnData)
}
