//go:build test

package contracts

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

const (
	FileNameCounter      = "tests/Counter"
	FileNameMessageCheck = "tests/MessageCheck"
)

func CounterDeployPayload(t *testing.T) types.DeployPayload {
	t.Helper()

	code, err := GetCode(FileNameCounter)
	require.NoError(t, err)
	return types.BuildDeployPayload(code, common.EmptyHash)
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
