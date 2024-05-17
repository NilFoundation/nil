package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMessageByHash(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	pool := msgpool.New(msgpool.DefaultConfig)
	require.NotNil(t, pool)

	api := NewEthAPI(ctx, NewBaseApi(rpccfg.DefaultEvmCallTimeout), database, pool, common.NewLogger("Test", false))

	tx, err := database.CreateRwTx(ctx)
	defer tx.Rollback()
	require.NoError(t, err)

	message := types.Message{ShardId: types.MasterShardId, Data: []byte("data")}

	err = db.WriteMessage(tx, types.MasterShardId, &message)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	data, err := api.GetMessageByHash(context.Background(), types.MasterShardId, message.Hash())
	require.NoError(t, err)
	assert.Equal(t, message, *data)
}
