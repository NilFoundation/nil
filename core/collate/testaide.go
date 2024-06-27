//go:build test

package collate

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type MockMsgPool struct {
	Msgs []*types.Message
}

var _ MsgPool = (*MockMsgPool)(nil)

func (m *MockMsgPool) Peek(context.Context, int, uint64) ([]*types.Message, error) {
	return m.Msgs, nil
}

func (m *MockMsgPool) OnNewBlock(context.Context, *types.Block, []*types.Message) error {
	return nil
}

func GenerateZeroState(t *testing.T, ctx context.Context,
	shardId types.ShardId, txFabric db.DB,
) {
	t.Helper()

	c := newCollator(execution.NewBlockGeneratorParams(shardId, 0, uint256.NewInt(10), 0), nil, nil, sharedLogger)
	err := c.GenerateZeroState(ctx, txFabric, execution.DefaultZeroStateConfig)
	require.NoError(t, err)
}

func GenerateBlockWithMessages(t *testing.T, ctx context.Context,
	shardId types.ShardId, nShards int, txFabric db.DB, msgs ...*types.Message,
) {
	t.Helper()

	pool := &MockMsgPool{Msgs: msgs}
	c := newCollator(execution.NewBlockGeneratorParams(shardId, nShards, uint256.NewInt(10), 0), new(TrivialShardTopology), pool, sharedLogger)
	require.NoError(t, c.GenerateBlock(ctx, txFabric))
}
