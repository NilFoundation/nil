package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

func TestDebugGetBlock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	block := &types.Block{
		Id:                 259,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
	}

	hexBytes, err := block.MarshalSSZ()
	require.NoError(t, err)
	blockHex := hexutil.Encode(hexBytes)

	tx, err := database.CreateRwTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	err = db.WriteBlock(tx, types.MainShardId, block)
	require.NoError(t, err)

	_, err = execution.PostprocessBlock(tx, types.MainShardId, types.NewValueFromUint64(10), 0, block.Hash())
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	base := NewBaseApi(0)
	api := NewDebugAPI(base, database, log.Logger)

	// When: Get the latest block
	res1, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.NoError(t, err)

	content := res1.Content
	require.Equal(t, blockHex, content)

	// When: Get existing block
	res2, err := api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(block.Id), false)
	require.NoError(t, err)

	require.Equal(t, res1, res2)

	// When: Get nonexistent block
	_, err = api.GetBlockByNumber(ctx, types.MainShardId, transport.BlockNumber(block.Id+1), false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
