package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionReceipt(t *testing.T) {
	db, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer db.Close()

	pool := msgpool.New(msgpool.DefaultConfig)
	require.NotNil(t, pool)

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, pool, common.NewLogger("Test", false))

	// Call GetBlockByNumber for transaction which is not in the database
	_, err = api.GetBlockByNumber(context.Background(), types.MasterShardId, transport.LatestBlockNumber, false)
	require.NoError(t, err)
}
