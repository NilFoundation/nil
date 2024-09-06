package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/require"
)

func NewPools(t *testing.T, ctx context.Context, n int) []msgpool.Pool {
	t.Helper()

	pools := make([]msgpool.Pool, n)
	for i := range pools {
		pool, err := msgpool.New(ctx, msgpool.NewConfig(types.ShardId(i)), nil)
		require.NoError(t, err)
		pools[i] = pool
	}

	return pools
}

func NewTestEthAPI(t *testing.T, ctx context.Context, db db.DB, nShards int) *APIImpl {
	t.Helper()

	api, err := NewEthAPI(ctx, db, NewPools(t, ctx, nShards), true)
	require.NoError(t, err)
	return api
}

func TestGetTransactionReceipt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badger, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer badger.Close()

	api := NewTestEthAPI(t, ctx, badger, 1)

	// Call GetBlockByNumber for transaction which is not in the database
	_, err = api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
