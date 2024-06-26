package nilservice

import (
	"context"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/holiman/uint256"
)

func startRpcServer(ctx context.Context, cfg *Config, db db.ReadOnlyDB, pools []msgpool.Pool) error {
	logger := logging.NewLogger("RPC")

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

const defaultCollatorTickPeriodMs = 2000

// Run starts message pools and collators for given shards, creates a single RPC server for all shards.
// It waits until one of the events:
//   - all goroutines finish successfully,
//   - a goroutine returns an error,
//   - SIGTERM or SIGINT is caught.
//
// It returns a value suitable for os.Exit().
func Run(ctx context.Context, cfg *Config, database db.DB, workers ...concurrent.Func) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := logging.NewLogger("nil")

	funcs := make([]concurrent.Func, 0, cfg.NShards+2+len(workers))
	if cfg.GracefulShutdown {
		funcs = []concurrent.Func{
			func(ctx context.Context) error {
				concurrent.OnSignal(ctx, cancel, syscall.SIGTERM, syscall.SIGINT)
				return nil
			},
		}
	}

	if cfg.CollatorTickPeriodMs == 0 {
		cfg.CollatorTickPeriodMs = defaultCollatorTickPeriodMs
	}
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)

	msgPools := make([]msgpool.Pool, cfg.NShards)
	for i := range cfg.NShards {
		msgPool := msgpool.New(msgpool.DefaultConfig)
		collator := collate.NewScheduler(database, msgPool, execution.BlockGeneratorParams{
			ShardId:       types.ShardId(i),
			NShards:       cfg.NShards,
			TraceEVM:      cfg.TraceEVM,
			Timer:         common.NewTimer(),
			GasBasePrice:  uint256.NewInt(cfg.GasBasePrice),
			GasPriceScale: cfg.GasPriceScale,
		}, collate.GetShardTopologyById(cfg.Topology), collatorTickPeriod)
		if len(cfg.ZeroState) != 0 {
			collator.ZeroState = cfg.ZeroState
		} else {
			collator.ZeroState = execution.DefaultZeroStateConfig
		}
		collator.MainKeysOutPath = cfg.MainKeysOutPath
		funcs = append(funcs, func(ctx context.Context) error {
			if err := collator.Run(ctx); err != nil {
				logger.Error().
					Err(err).
					Stringer(logging.FieldShardId, types.ShardId(i)).
					Msg("Collator goroutine failed")
				return err
			}
			return nil
		})

		msgPools[i] = msgPool
	}

	funcs = append(funcs, func(ctx context.Context) error {
		if err := startRpcServer(ctx, cfg, database, msgPools); err != nil {
			logger.Error().Err(err).Msg("RPC server goroutine failed")
			return err
		}
		return nil
	})

	funcs = append(funcs, workers...)

	logger.Info().Msg("Starting services...")

	if err := concurrent.Run(ctx, funcs...); err != nil {
		logger.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return 1
	}

	logger.Info().Msg("App is terminated.")
	return 0
}
