package nilservice

import (
	"context"
	"syscall"

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

func startRpcServer(ctx context.Context, db db.DB, pool msgpool.Pool) error {
	logger := common.NewLogger("RPC", false)

	httpConfig := &httpcfg.HttpCfg{
		Enabled:           true,
		HttpListenAddress: "127.0.0.1",
		HttpPort:          8529,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}

	base := jsonrpc.NewBaseApi(rpccfg.DefaultEvmCallTimeout)

	ethImpl := jsonrpc.NewEthAPI(ctx, base, db, pool, logger)
	debugImpl := jsonrpc.NewDebugAPI(base, db, logger)

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

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}

func Run(ctx context.Context, nShards int, database db.DB) int {
	common.SetupGlobalLogger()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgPools := make([]msgpool.Pool, nShards)
	for i := range nShards {
		msgPools[i] = msgpool.New(msgpool.DefaultConfig)
	}

	log.Info().Msg("Starting services...")

	if err := concurrent.Run(ctx,
		func(ctx context.Context) error {
			concurrent.OnSignal(ctx, cancel, syscall.SIGTERM, syscall.SIGINT)
			return nil
		},
		func(ctx context.Context) error {
			shards := make([]*shardchain.ShardChain, nShards)
			for i := range nShards {
				shards[i] = shardchain.NewShardChain(types.ShardId(i), database, nShards)
			}

			collator := collate.NewCollator(shards, msgPools)
			return collator.Run(ctx)
		},
		func(ctx context.Context) error {
			return startRpcServer(ctx, database, msgPools[0])
		},
	); err != nil {
		log.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return 1
	}

	log.Warn().Msg("App is terminated.")
	return 0
}
