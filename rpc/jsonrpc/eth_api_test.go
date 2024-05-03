package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/stretchr/testify/assert"

	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
)

func TestGetTransactionReceipt(t *testing.T) {
	db, err := db.NewSqlite(t.TempDir() + "/foo-1.db")
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))
	// Call GetBlockByNumber for transaction which is not in the database

	_, err = api.GetBlockByNumber(context.Background(), transport.LatestBlockNumber, false)
	assert.Equal(t, "not implemented", err.Error())
}
