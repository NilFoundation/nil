package rpctest

import (
	"context"

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

func startRpcServer(ctx context.Context, nshards int, dbpath string) {
	logger := common.NewLogger("RPC", false)

	db, err := db.NewBadgerDb(dbpath)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	// each shard will interact with DB via this client
	shards := make([]*shardchain.ShardChain, 0)
	for i := 0; i < nshards; i++ {
		shards = append(shards, shardchain.NewShardChain(i, db))
	}

	numClusterTicks := 2
	for t := 0; t < numClusterTicks; t++ {
		var wg sync.WaitGroup

		for i := 0; i < nshards; i++ {
			wg.Add(1)
			go shards[i].Collate(&wg)
		}

		wg.Wait()
	}

	httpConfig := httpcfg.HttpCfg{
		Enabled:           true,
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
		logger.Fatal().Msg(err.Error())
	}
}
