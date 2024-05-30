package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
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

	api := NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), database, []msgpool.Pool{pool}, common.NewLogger("Test"))

	tx, err := database.CreateRwTx(ctx)
	defer tx.Rollback()
	require.NoError(t, err)

	message := types.Message{Data: []byte("data")}
	receipt := types.Receipt{MsgHash: message.Hash()}

	blockHash := writeTestBlock(
		t, tx, types.MasterShardId, types.BlockNumber(0), []*types.Message{&message}, []*types.Receipt{&receipt})
	_, err = execution.PostprocessBlock(tx, types.MasterShardId, blockHash)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	data, err := api.GetInMessageByHash(context.Background(), types.MasterShardId, message.Hash())
	require.NoError(t, err)
	assert.Equal(t, message.Hash(), data.Hash)
}
