package jsonrpc

import (
	"context"
	"strconv"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/stretchr/testify/require"
)

func TestDebugGetBlock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	database, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer database.Close()

	block := types.Block{
		Id:                 259,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
	}
	blockHash := block.Hash()

	err = database.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), blockHash.Bytes())
	require.NoError(t, err)

	tx, err := database.CreateRwTx(ctx)
	defer tx.Rollback()
	require.NoError(t, err)

	err = db.WriteBlock(tx, types.MasterShardId, &block)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	base := NewBaseApi(0)

	api := NewDebugAPI(base, database, nil)

	res, err := api.GetBlockByNumber(ctx, types.MasterShardId, transport.LatestBlockNumber)
	require.NoError(t, err)

	// contains res
	require.NotNil(t, res)

	require.Contains(t, res, "content")
	content, ok := res["content"]

	// content is a map
	require.True(t, ok)
	require.NotNil(t, content)

	hexBytes, err := block.MarshalSSZ()
	require.NoError(t, err)

	// content contains hash
	require.Equal(t, content, hexutil.Encode(hexBytes))
}
