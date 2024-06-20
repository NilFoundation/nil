//go:build test

package contracts

import (
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

const fileNameCounter = "tests/Counter"

func NewCounterDeployMessage(t *testing.T, shardId types.ShardId, from types.Address, seqno types.Seqno) *types.Message {
	t.Helper()

	code, err := GetCode(fileNameCounter)
	require.NoError(t, err)

	data := types.BuildDeployPayload(code, common.EmptyHash).Bytes()
	return &types.Message{
		Kind:     types.DeployMessageKind,
		From:     from,
		To:       types.CreateAddress(shardId, data),
		Data:     data,
		Seqno:    seqno,
		GasLimit: *types.NewUint256(100000),
	}
}

func NewCounterExecuteMessage(t *testing.T, shardId types.ShardId, from types.Address, seqno types.Seqno) *types.Message {
	t.Helper()

	code, err := GetCode(fileNameCounter)
	require.NoError(t, err)
	abiCallee, err := GetAbi(fileNameCounter)
	require.NoError(t, err)
	callData, err := abiCallee.Pack("add", int32(11))
	require.NoError(t, err)

	return &types.Message{
		Kind:     types.ExecutionMessageKind,
		From:     from,
		To:       types.CreateAddress(shardId, types.BuildDeployPayload(code, common.EmptyHash).Bytes()),
		Data:     callData,
		Seqno:    seqno,
		GasLimit: *types.NewUint256(100000),
	}
}
