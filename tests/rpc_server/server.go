package rpctest

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/rs/zerolog/log"
)

func startRpcServer(ctx context.Context, nShards int, dbpath string) {
	common.SetupGlobalLogger()
	logger := common.NewLogger("RPC", false)

	badger, err := db.NewBadgerDb(dbpath)
	if err != nil {
		log.Fatal().Msgf("Failed to open db: %s", err.Error())
	}

	httpConfig := &httpcfg.HttpCfg{
		Enabled:           true,
		HttpListenAddress: "127.0.0.1",
		HttpPort:          8529,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}

	base := jsonrpc.NewBaseApi(rpccfg.DefaultEvmCallTimeout)

	pool := msgpool.New(msgpool.DefaultConfig)
	if pool == nil {
		log.Fatal().Msgf("Failed to create message pool")
	}

	ethImpl := jsonrpc.NewEthAPI(base, badger, pool, logger)
	debugImpl := jsonrpc.NewDebugAPI(base, badger, logger)

	apiList := []transport.API{
		{
			Namespace: "eth",
			Public:    true,
			Service:   jsonrpc.EthAPI(ethImpl),
			Version:   "1.0",
		},
		{
			Namespace: "debug",
			Public:    true,
			Service:   jsonrpc.DebugAPI(debugImpl),
			Version:   "1.0",
		},
	}

	if err := concurrent.Run(ctx,
		func(ctx context.Context) error {
			shards := make([]*shardchain.ShardChain, nShards)
			for i := 0; i < nShards; i++ {
				shards[i] = shardchain.NewShardChain(types.ShardId(i), badger, nShards)
			}

			collator := collate.NewCollator(shards)
			return collator.Run(ctx)
		},
		func(ctx context.Context) error {
			return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
		},
	); err != nil {
		log.Fatal().Err(err).Msg("RPC server stopped.")
	}
}
