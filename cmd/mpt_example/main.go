package main

import (
	"context"
	"flag"
	"os"

	"sync"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/rpc"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/rs/zerolog/log"
)

func startRpcServer(ctx context.Context, db db.DB) {
	logger := common.NewLogger("RPC", false)

	httpConfig := httpcfg.HttpCfg{
		Enabled:           true,
		HttpServerEnabled: true,
		HttpURL:           "tcp://127.0.0.1:8529",
		HttpListenAddress: "127.0.0.1",
		HttpPort:          8529,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}

	base := jsonrpc.NewBaseApi(rpccfg.DefaultEvmCallTimeout)

	ethImpl := jsonrpc.NewEthAPI(base, db, logger)

	apiList := []transport.API{
		{
			Namespace: "eth",
			Public:    true,
			Service:   jsonrpc.EthAPI(ethImpl),
			Version:   "1.0",
		}}

	if err := rpc.StartRpcServer(ctx, &httpConfig, apiList, logger); err != nil {
		logger.Error().Msg(err.Error())
	}
}

func main() {
	// parse args
	nshards := flag.Int("nshards", 5, "number of shardchains")

	flag.Parse()

	ctx := context.Background()

	database, err := db.NewSqlite(os.TempDir() + "/foo-4.db")
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	// each shard will interact with DB via this client
	dbClient := db.NewDBClient()
	shards := make([]*shardchain.ShardChain, 0)
	for i := 0; i < *nshards; i++ {
		shards = append(shards, shardchain.NewShardChain(i, dbClient))
	}

	numClusterTicks := 2
	for t := 0; t < numClusterTicks; t++ {
		var wg sync.WaitGroup

		for i := 0; i < *nshards; i++ {
			wg.Add(1)
			go shards[i].Collate(&wg)
		}

		wg.Wait()
	}

	startRpcServer(ctx, database)
}
