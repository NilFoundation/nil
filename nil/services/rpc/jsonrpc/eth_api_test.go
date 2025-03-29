package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	"github.com/stretchr/testify/require"
)

func NewPools(t *testing.T, ctx context.Context, n int) map[types.ShardId]txnpool.Pool {
	t.Helper()

	pools := make(map[types.ShardId]txnpool.Pool, n)
	for i := range types.ShardId(n) {
		pool, err := txnpool.New(ctx, txnpool.NewConfig(i), nil)
		require.NoError(t, err)
		pools[i] = pool
	}

	return pools
}

func NewTestEthAPI(t *testing.T, ctx context.Context, db db.DB, nShards int) *APIImpl {
	t.Helper()

	shardApis := make(map[types.ShardId]rawapi.ShardApi)
	pools := NewPools(t, ctx, nShards)
	for shardId := range types.ShardId(nShards) {
		var err error
		shardApi := rawapi.NewLocalShardApi(shardId, db, pools[shardId], false)
		shardApis[shardId], err = rawapi.NewLocalRawApiAccessor(shardId, shardApi)
		require.NoError(t, err)
	}
	rawApi := rawapi.NewNodeApiOverShardApis(shardApis)
	return NewEthAPI(ctx, rawApi, db, true, false)
}

func TestGetTransactionReceipt(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	badger, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer badger.Close()

	api := NewTestEthAPI(t, ctx, badger, 1)

	// Call GetBlockByNumber for transaction which is not in the database
	_, err = api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
