package shardchain

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBlock(t *testing.T) {
	t.Parallel()
	db, err := db.NewBadgerDb(t.TempDir())
	require.NoError(t, err)
	shardId := types.ShardId(1)
	shard := NewShardChain(shardId, db, 2)

	var m types.Message
	m.From = common.HexToAddress("9405832983856CB0CF6CD570F071122F1BEA2F20")
	m.Data = hexutil.FromHex("6009600c60003960096000f3600054600101600055")

	_, err = shard.GenerateBlock(context.Background(), []*types.Message{&m})
	require.NoError(t, err)

	m.To = execution.CreateAddress(m.From, m.Seqno)

	_, err = shard.GenerateBlock(context.Background(), []*types.Message{&m})
	require.NoError(t, err)

	tx, err := db.CreateRoTx(context.Background())
	require.NoError(t, err)

	es, err := execution.NewExecutionStateForShard(tx, shardId)
	require.NoError(t, err)

	r, err := es.GetReceipt(0)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), r.MsgIndex)
	assert.Equal(t, m.Hash(), r.MsgHash)
}
