package jsonrpc

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/assert"
)

func TestGetBlockByNumber(t *testing.T) {
	db, err := db.NewSqlite(t.TempDir() + "/foo-2.db")
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()
	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))
	_, err = api.GetBlockByNumber(context.Background(), transport.LatestBlockNumber, false)
	assert.Equal(t, err.Error(), "not implemented")
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	ctx := context.Background()

	blockHash := common.HexToHash("0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")

	db, err := db.NewSqlite(t.TempDir() + "/foo-3.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))
	_, err = api.GetBlockTransactionCountByHash(ctx, blockHash)
	assert.Equal(t, err.Error(), "not implemented")
}

func TestGetBlockTransactionCountByNumber(t *testing.T) {
	ctx := context.Background()

	db, err := db.NewSqlite(t.TempDir() + "/foo-4.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	api := NewEthAPI(NewBaseApi(rpccfg.DefaultEvmCallTimeout), db, common.NewLogger("Test", false))
	_, err = api.GetBlockTransactionCountByNumber(ctx, transport.LatestBlockNumber)
	assert.Equal(t, err.Error(), "not implemented")
}
