package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/require"
)

func TestGetTransactionReceipt(t *testing.T) {
	db, err := db.NewBadgerDbInMemory()
	require.NoError(t, err)
	defer db.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))

	// Call GetBlockByNumber for transaction which is not in the database
	_, err = api.GetBlockByNumber(context.Background(), transport.LatestBlockNumber, false)
	require.NoError(t, err)
}
