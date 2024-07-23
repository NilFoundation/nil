package nilservice

import (
	"context"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/admin"
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
	dbImpl := jsonrpc.NewDbAPI(db, logger)

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
		{
			Namespace: "db",
			Public:    true,
			Service:   jsonrpc.DbAPI(dbImpl),
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}

func startAdminServer(ctx context.Context, cfg *Config) error {
	config := &admin.ServerConfig{
		Enabled:        cfg.AdminSocketPath != "",
		UnixSocketPath: cfg.AdminSocketPath,
	}
	return admin.StartAdminServer(ctx, config, logging.NewLogger("admin"))
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

	collators, err := CreateCollators(cfg, database)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create collators")
		return 1
	}
	msgPools := make([]msgpool.Pool, cfg.NShards)
	for i, collator := range collators {
		msgPools[i] = collator.GetMsgPool()
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
	}

	funcs = append(funcs, func(ctx context.Context) error {
		if err := startRpcServer(ctx, cfg, database, msgPools); err != nil {
			logger.Error().Err(err).Msg("RPC server goroutine failed")
			return err
		}
		return nil
	})

	funcs = append(funcs, func(ctx context.Context) error {
		if err := startAdminServer(ctx, cfg); err != nil {
			logger.Error().Err(err).Msg("Admin server goroutine failed")
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

// RunCollatorsOnly is same as `Run`, but runs only collators, without any other workers.
func RunCollatorsOnly(ctx context.Context, collators []*collate.Scheduler, cfg *Config) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := logging.NewLogger("nil")

	funcs := make([]concurrent.Func, 0, cfg.NShards+2)
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

	msgPools := make([]msgpool.Pool, cfg.NShards)
	for i, collator := range collators {
		msgPools[i] = collator.GetMsgPool()
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
	}

	logger.Info().Msg("Starting collators...")

	if err := concurrent.Run(ctx, funcs...); err != nil {
		logger.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return 1
	}

	logger.Info().Msg("App is terminated.")
	return 0
}

func CreateCollators(cfg *Config, database db.DB) ([]*collate.Scheduler, error) {
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)

	collators := make([]*collate.Scheduler, 0, cfg.NShards)

	for i := range cfg.NShards {
		msgPool := msgpool.New(msgpool.DefaultConfig)
		collator := collate.NewScheduler(database, msgPool, collate.Params{
			BlockGeneratorParams: execution.BlockGeneratorParams{
				ShardId:       types.ShardId(i),
				NShards:       cfg.NShards,
				TraceEVM:      cfg.TraceEVM,
				Timer:         common.NewTimer(),
				GasBasePrice:  types.NewValueFromUint64(cfg.GasBasePrice),
				GasPriceScale: cfg.GasPriceScale,
			},
		}, collate.GetShardTopologyById(cfg.Topology), collatorTickPeriod)
		collators = append(collators, collator)
		if len(cfg.ZeroState) != 0 {
			collator.ZeroState = cfg.ZeroState
		} else {
			collator.ZeroState = execution.DefaultZeroStateConfig
		}
		collator.MainKeysOutPath = cfg.MainKeysOutPath
	}
	return collators, nil
}
