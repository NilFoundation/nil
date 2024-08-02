//go:build test

package contracts

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

const (
	NameCounter      = "tests/Counter"
	NameMessageCheck = "tests/MessageCheck"
	NameSender       = "tests/Sender"
	NameTest         = "tests/Test"
	NameTokensTest   = "tests/TokensTest"
)

func CounterDeployPayload(t *testing.T) types.DeployPayload {
	t.Helper()

	code, err := GetCode(NameCounter)
	require.NoError(t, err)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

func CounterAddress(t *testing.T, shardId types.ShardId) types.Address {
	t.Helper()

	return types.CreateAddress(shardId, CounterDeployPayload(t))
}

func WalletAddress(t *testing.T, shardId types.ShardId, salt, pubKey []byte) types.Address {
	t.Helper()

	res, err := CalculateAddress(NameWallet, shardId, salt, pubKey)
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

func GetCounterValue(t *testing.T, data []byte) int32 {
	t.Helper()

	res, err := UnpackData(NameCounter, "get", data)
	require.NoError(t, err)

	val, ok := res[0].(int32)
	require.True(t, ok)
	return val
}

func NewWalletSendCallData(t *testing.T,
	bytecode types.Code, gasLimit types.Gas, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, kind types.MessageKind,
) []byte {
	t.Helper()

	intMsg := &types.InternalMessagePayload{
		Data:        bytecode,
		To:          contractAddress,
		Value:       value,
		FeeCredit:   gasLimit.ToValue(types.NewValueFromUint64(10)),
		ForwardKind: types.ForwardKindNone,
		Currency:    currencies,
		Kind:        kind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	require.NoError(t, err)

	return NewCallDataT(t, NameWallet, "send", intMsgData)
}
