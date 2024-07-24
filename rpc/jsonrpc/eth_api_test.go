package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionReceipt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badger, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer badger.Close()

	pool := msgpool.New(msgpool.DefaultConfig)
	require.NotNil(t, pool)

	api, err := NewEthAPI(ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), badger, []msgpool.Pool{pool}, logging.NewLogger("Test"))
	require.NoError(t, err)

	// Call GetBlockByNumber for transaction which is not in the database
	_, err = api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
