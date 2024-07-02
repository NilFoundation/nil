//go:build test

package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

func GenerateZeroState(t *testing.T, ctx context.Context,
	shardId types.ShardId, txFabric db.DB,
) {
	t.Helper()

	txOwner, err := NewTxOwner(ctx, txFabric)
	require.NoError(t, err)
	g, err := NewBlockGenerator(NewBlockGeneratorParams(shardId, 1, types.NewValueFromUint64(10), 0), txOwner)
	require.NoError(t, err)
	require.NoError(t, g.GenerateZeroState(DefaultZeroStateConfig))
}

func GenerateBlockFromMessages(t *testing.T, ctx context.Context,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, msgs ...*types.Message,
) common.Hash {
	t.Helper()
	return generateBlockFromMessages(t, ctx, true, shardId, blockId, prevBlock, txFabric, msgs...)
}

func GenerateBlockFromMessagesWithoutExecution(t *testing.T, ctx context.Context,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, msgs ...*types.Message,
) common.Hash {
	t.Helper()
	return generateBlockFromMessages(t, ctx, false, shardId, blockId, prevBlock, txFabric, msgs...)
}

func generateBlockFromMessages(t *testing.T, ctx context.Context, execute bool,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, msgs ...*types.Message,
) common.Hash {
	t.Helper()

	tx, err := txFabric.CreateRwTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	es, err := NewExecutionState(tx, shardId, prevBlock, common.NewTestTimer(0))
	require.NoError(t, err)

	for _, msg := range msgs {
		es.AddInMessage(msg)

		if !execute {
			es.AddReceipt(0, nil)
			continue
		}

		gas := msg.GasLimit
		var leftOverGas types.Gas
		switch {
		case msg.IsDeploy():
			leftOverGas, err = es.HandleDeployMessage(ctx, msg)
			require.NoError(t, err)
		case msg.IsRefund():
			err = es.HandleRefundMessage(ctx, msg)
			require.NoError(t, err)
		default:
			leftOverGas, _, err = es.HandleExecutionMessage(ctx, msg)
			require.NoError(t, err)
		}

		es.AddReceipt(gas.Sub(leftOverGas), err)
	}

	blockHash, err := es.Commit(blockId)
	require.NoError(t, err)

	block, err := PostprocessBlock(tx, shardId, types.NewValueFromUint64(10), 0, blockHash)
	require.NoError(t, err)
	require.NotNil(t, block)

	require.NoError(t, tx.Commit())

	return blockHash
}

func NewDeployMessage(payload types.DeployPayload,
	shardId types.ShardId, from types.Address, seqno types.Seqno,
) *types.Message {
	return &types.Message{
		Flags:    types.NewMessageFlags(types.MessageFlagInternal, types.MessageFlagDeploy),
		Data:     payload.Bytes(),
		From:     from,
		Seqno:    seqno,
		GasLimit: 100000,
		To:       types.CreateAddress(shardId, payload),
	}
}

func NewExecutionMessage(from, to types.Address, seqno types.Seqno, callData []byte) *types.Message {
	return &types.Message{
		From:     from,
		To:       to,
		Data:     callData,
		Seqno:    seqno,
		GasLimit: 100000,
	}
}

func Deploy(t *testing.T, ctx context.Context, es *ExecutionState,
	payload types.DeployPayload, shardId types.ShardId, from types.Address, seqno types.Seqno,
) types.Address {
	t.Helper()

	msg := NewDeployMessage(payload, shardId, from, seqno)
	gas := msg.GasLimit
	es.AddInMessage(msg)
	gasLeft, err := es.HandleDeployMessage(ctx, msg)
	require.NoError(t, err)
	es.AddReceipt(gas.Sub(gasLeft), nil)

	return msg.To
}
