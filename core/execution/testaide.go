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

		switch msg.Kind {
		case types.DeployMessageKind:
			_, err := es.HandleDeployMessage(ctx, msg)
			require.NoError(t, err)
		case types.ExecutionMessageKind:
			_, _, err := es.HandleExecutionMessage(ctx, msg)
			require.NoError(t, err)
		case types.RefundMessageKind:
			err := es.HandleRefundMessage(ctx, msg)
			require.NoError(t, err)
		default:
			panic("unreachable")
		}
	}

	blockHash, err := es.Commit(blockId)
	require.NoError(t, err)

	block, err := PostprocessBlock(tx, shardId, blockHash)
	require.NoError(t, err)
	require.NotNil(t, block)

	require.NoError(t, tx.Commit())

	return blockHash
}

func NewDeployMessage(payload types.DeployPayload,
	shardId types.ShardId, from types.Address, seqno types.Seqno,
) *types.Message {
	data := payload.Bytes()
	return &types.Message{
		Internal: true,
		Kind:     types.DeployMessageKind,
		Data:     data,
		From:     from,
		Seqno:    seqno,
		GasLimit: *types.NewUint256(100000),
		To:       types.CreateAddress(shardId, data),
	}
}

func Deploy(t *testing.T, ctx context.Context, es *ExecutionState,
	payload types.DeployPayload, shardId types.ShardId, from types.Address, seqno types.Seqno,
) types.Address {
	t.Helper()

	msg := NewDeployMessage(payload, shardId, from, seqno)
	es.AddInMessage(msg)
	_, err := es.HandleDeployMessage(ctx, msg)
	require.NoError(t, err)

	return msg.To
}
