package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/rpc/transport/rpccfg"
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
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), badger, []msgpool.Pool{pool}, true, logging.NewLogger("Test"))
	require.NoError(t, err)

	// Call GetBlockByNumber for transaction which is not in the database
	_, err = api.GetBlockByNumber(ctx, types.MainShardId, transport.LatestBlockNumber, false)
	require.ErrorIs(t, err, db.ErrKeyNotFound)
}
