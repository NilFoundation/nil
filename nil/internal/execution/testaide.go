//go:build test

package execution

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

func GenerateZeroState(t *testing.T, ctx context.Context,
	shardId types.ShardId, txFabric db.DB,
) common.Hash {
	t.Helper()

	g, err := NewBlockGenerator(ctx,
		NewBlockGeneratorParams(shardId, 1, types.NewValueFromUint64(10), 0),
		txFabric)
	require.NoError(t, err)
	defer g.Rollback()
	block, err := g.GenerateZeroState(DefaultZeroStateConfig, nil)
	require.NoError(t, err)
	require.NotNil(t, block)
	return block.Hash()
}

func GenerateBlockFromMessages(t *testing.T, ctx context.Context,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, childChainBlocks map[types.ShardId]common.Hash, msgs ...*types.Message,
) common.Hash {
	t.Helper()
	return generateBlockFromMessages(t, ctx, true, shardId, blockId, prevBlock, txFabric, childChainBlocks, msgs...)
}

func GenerateBlockFromMessagesWithoutExecution(t *testing.T, ctx context.Context,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, msgs ...*types.Message,
) common.Hash {
	t.Helper()
	return generateBlockFromMessages(t, ctx, false, shardId, blockId, prevBlock, txFabric, nil, msgs...)
}

func generateBlockFromMessages(t *testing.T, ctx context.Context, execute bool,
	shardId types.ShardId, blockId types.BlockNumber, prevBlock common.Hash,
	txFabric db.DB, childChainBlocks map[types.ShardId]common.Hash, msgs ...*types.Message,
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
			es.AddReceipt(NewExecutionResult())
			continue
		}

		var execResult *ExecutionResult
		switch {
		case msg.IsDeploy():
			execResult = es.HandleDeployMessage(ctx, msg)
			require.False(t, execResult.Failed())
		case msg.IsRefund():
			execResult = NewExecutionResult()
			execResult.SetFatal(es.HandleRefundMessage(ctx, msg))
			require.False(t, execResult.Failed())
		default:
			execResult = es.HandleExecutionMessage(ctx, msg)
			require.False(t, execResult.Failed())
		}

		es.AddReceipt(execResult)
	}

	es.ChildChainBlocks = childChainBlocks

	blockHash, err := es.Commit(blockId)
	require.NoError(t, err)

	block, err := PostprocessBlock(tx, shardId, types.NewValueFromUint64(10), blockHash)
	require.NoError(t, err)
	require.NotNil(t, block)

	require.NoError(t, db.WriteBlockTimestamp(tx, shardId, blockHash, 0))

	require.NoError(t, tx.Commit())

	return blockHash
}

func NewDeployMessage(payload types.DeployPayload,
	shardId types.ShardId, from types.Address, seqno types.Seqno, value types.Value,
) *types.Message {
	gasCredit := types.Gas(100_000).ToValue(types.DefaultGasPrice)
	return &types.Message{
		Flags:     types.NewMessageFlags(types.MessageFlagInternal, types.MessageFlagDeploy),
		Data:      payload.Bytes(),
		From:      from,
		Seqno:     seqno,
		FeeCredit: gasCredit,
		To:        types.CreateAddress(shardId, payload),
		Value:     value,
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

	msg := NewDeployMessage(payload, shardId, from, seqno, types.Value{})
	es.AddInMessage(msg)
	execResult := es.HandleDeployMessage(ctx, msg)
	require.False(t, execResult.Failed())
	es.AddReceipt(execResult)

	return msg.To
}
