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

func startRpcServer(ctx context.Context, cfg *Config, db db.DB, pools []msgpool.Pool) error {
	logger := common.NewLogger("RPC")

	httpConfig := &httpcfg.HttpCfg{
		Enabled:           true,
		HttpListenAddress: "127.0.0.1",
		HttpPort:          cfg.HttpPort,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}

	base := jsonrpc.NewBaseApi(rpccfg.DefaultEvmCallTimeout)

	ethImpl, err := jsonrpc.NewEthAPI(ctx, base, db, pools, logger)
	if err != nil {
		return err
	}
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

// Run starts message pools and collators for given shards, creates a single RPC server for all shards.
// It waits until one of the events:
//   - all goroutines finish successfully,
//   - a goroutine returns an error,
//   - SIGTERM or SIGINT is caught.
//
// It returns a value suitable for os.Exit().
func Run(ctx context.Context, cfg *Config, database db.DB, workers ...concurrent.Func) int {
	common.SetupGlobalLogger()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	funcs := []concurrent.Func{
		func(ctx context.Context) error {
			concurrent.OnSignal(ctx, cancel, syscall.SIGTERM, syscall.SIGINT)
			return nil
		},
	}

	msgPools := make([]msgpool.Pool, cfg.NShards)
	for i := range cfg.NShards {
		msgPool := msgpool.New(msgpool.DefaultConfig)
		shard := shardchain.NewShardChain(types.ShardId(i), database)
		collator := collate.NewScheduler(shard, msgPool, types.ShardId(i), cfg.NShards, collate.GetShardTopologyById(cfg.Topology))
		funcs = append(funcs, func(ctx context.Context) error {
			return collator.Run(ctx)
		})

		msgPools[i] = msgPool
	}

	funcs = append(funcs, func(ctx context.Context) error {
		return startRpcServer(ctx, cfg, database, msgPools)
	})

	funcs = append(funcs, workers...)

	log.Info().Msg("Starting services...")

	if err := concurrent.Run(ctx, funcs...); err != nil {
		log.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return 1
	}

	log.Warn().Msg("App is terminated.")
	return 0
}
