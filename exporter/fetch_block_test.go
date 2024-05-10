package exporter

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/stretchr/testify/require"
)

func TestFetchBlock(t *testing.T) {
	ctx := context.Background()

	logger := common.NewLogger("RPC", false)

	database, err := db.NewBadgerDb(t.TempDir())
	require.NoError(t, err, "Failed to create database")

	var id uint64 = 0

	block := types.Block{
		Id:                 id,
		PrevBlock:          common.EmptyHash,
		SmartContractsRoot: common.EmptyHash,
		MessagesRoot:       common.EmptyHash,
	}
	blockHash := block.Hash()

	err = database.Put(db.LastBlockTable, []byte(strconv.FormatUint(id, 10)), blockHash.Bytes())
	require.NoError(t, err)

	tx, err := database.CreateRwTx(ctx)
	defer tx.Rollback()
	require.NoError(t, err)

	err = db.WriteBlock(tx, types.MasterShardId, &block)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	httpCfg := httpcfg.HttpCfg{
		Enabled:           true,
		HttpListenAddress: "127.0.0.1",
		HttpPort:          8345,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}
	apiList := []transport.API{
		{
			Namespace: "debug",
			Public:    true,
			Service:   jsonrpc.DebugAPI(jsonrpc.NewDebugAPI(jsonrpc.NewBaseApi(0), database, logger)),
			Version:   "1.0",
		},
	}
	go func() {
		_ = rpc.StartRpcServer(ctx, &httpCfg, apiList, logger)
	}()

	time.Sleep(1 * time.Second)

	cfg := Cfg{
		APIEndpoints: []string{"http://127.0.0.1:8345"},
	}

	fetchedBlock, err := cfg.FetchLastBlock(ctx, types.MasterShardId)
	require.NoError(t, err, "Failed to fetch block")

	require.NotNil(t, fetchedBlock, "Fetched block is nil")

	require.Equal(t, block.Id, fetchedBlock.Id)
	require.Equal(t, block.PrevBlock, fetchedBlock.PrevBlock)
	require.Equal(t, block.SmartContractsRoot, fetchedBlock.SmartContractsRoot)
	require.Equal(t, block.MessagesRoot, fetchedBlock.MessagesRoot)

	hashBlock, err := cfg.FetchBlockByHash(ctx, types.MasterShardId, block.Hash())
	require.NoError(t, err, "Failed to fetch block by hash")
	require.NotNil(t, hashBlock, "Fetched block by hash is nil")

	require.Equal(t, block.Id, hashBlock.Id)
	require.Equal(t, block.PrevBlock, hashBlock.PrevBlock)
	require.Equal(t, block.SmartContractsRoot, hashBlock.SmartContractsRoot)
	require.Equal(t, block.MessagesRoot, hashBlock.MessagesRoot)
}
