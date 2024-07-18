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

	g, err := NewBlockGenerator(ctx,
		NewBlockGeneratorParams(shardId, 1, types.NewValueFromUint64(10), 0),
		txFabric)
	require.NoError(t, err)
	defer g.Rollback()
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

	es, err := NewExecutionState(tx, shardId, prevBlock, common.NewTestTimer(0), 1)
	require.NoError(t, err)

	for _, msg := range msgs {
		es.AddInMessage(msg)

		if !execute {
			es.AddReceipt(0, nil)
			continue
		}

		gas := msg.FeeCredit.ToGas(es.GasPrice)
		var leftOverGas types.Gas
		switch {
		case msg.IsDeploy():
			leftOverGas, _, err = es.HandleDeployMessage(ctx, msg)
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

	require.NoError(t, db.WriteBlockTimestamp(tx, shardId, blockHash, 0))

	require.NoError(t, tx.Commit())

	return blockHash
}

func NewDeployMessage(payload types.DeployPayload,
	shardId types.ShardId, from types.Address, seqno types.Seqno,
) *types.Message {
	gasCredit := types.Gas(100_000).ToValue(types.DefaultGasPrice)
	return &types.Message{
		Flags:     types.NewMessageFlags(types.MessageFlagInternal, types.MessageFlagDeploy),
		Data:      payload.Bytes(),
		From:      from,
		Seqno:     seqno,
		Value:     types.NewValueFromUint64(1_000_000),
		FeeCredit: gasCredit,
		To:        types.CreateAddress(shardId, payload),
	}
}

func NewExecutionMessage(from, to types.Address, seqno types.Seqno, callData []byte) *types.Message {
	gasCredit := types.Gas(100_000).ToValue(types.DefaultGasPrice)
	return &types.Message{
		From:      from,
		To:        to,
		Data:      callData,
		Seqno:     seqno,
		FeeCredit: gasCredit,
	}
}

func Deploy(t *testing.T, ctx context.Context, es *ExecutionState,
	payload types.DeployPayload, shardId types.ShardId, from types.Address, seqno types.Seqno,
) types.Address {
	t.Helper()

	msg := NewDeployMessage(payload, shardId, from, seqno)
	gas := msg.FeeCredit.ToGas(es.GasPrice)
	es.AddInMessage(msg)
	gasLeft, _, err := es.HandleDeployMessage(ctx, msg)
	require.NoError(t, err)
	es.AddReceipt(gas.Sub(gasLeft), nil)

	return msg.To
}
