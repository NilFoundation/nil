package nilservice

import (
	"context"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/admin"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/rpc/transport/rpccfg"
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
	defer ethImpl.Shutdown()

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

// used to access started service from outside of `Run` call
type ServiceInterop struct {
	MsgPools []msgpool.Pool
}

// Run starts message pools and collators for given shards, creates a single RPC server for all shards.
// It waits until one of the events:
//   - all goroutines finish successfully,
//   - a goroutine returns an error,
//   - SIGTERM or SIGINT is caught.
//
// It returns a value suitable for os.Exit().
func Run(ctx context.Context, cfg *Config, database db.DB, interop chan<- ServiceInterop, workers ...concurrent.Func) int {
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

	var msgPools []msgpool.Pool
	var networkManager *network.Manager
	var err error

	switch cfg.RunMode {
	case NormalRunMode:
		networkManager, err = createNetworkManager(ctx, cfg)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to create network manager")
			return 1
		}
		if networkManager != nil {
			defer networkManager.Close()

			// todo: listen to select shards only
			shardsToListen := make([]types.ShardId, cfg.NShards)
			for i := range cfg.NShards {
				shardsToListen[i] = types.ShardId(i)
			}
			funcs = append(funcs, func(ctx context.Context) error {
				return collate.StartBlockListeners(ctx, networkManager, database, shardsToListen)
			})
		}

		fallthrough
	case CollatorsOnlyRunMode:
		collators := createCollators(cfg, database, networkManager)

		msgPools = make([]msgpool.Pool, cfg.NShards)
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
	case BlockReplayRunMode:
		replayer := collate.NewReplayScheduler(database, collate.ReplayParams{
			BlockGeneratorParams: execution.BlockGeneratorParams{
				ShardId:       cfg.ReplayShardId,
				NShards:       cfg.NShards,
				TraceEVM:      cfg.TraceEVM,
				Timer:         common.NewTimer(),
				GasBasePrice:  types.NewValueFromUint64(cfg.GasBasePrice),
				GasPriceScale: cfg.GasPriceScale,
			},
			Timeout:           time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs),
			ReplayBlockNumber: cfg.ReplayBlockId,
		})

		funcs = append(funcs, func(ctx context.Context) error {
			if err := replayer.Run(ctx); err != nil {
				logger.Error().
					Err(err).
					Stringer(logging.FieldShardId, cfg.ReplayShardId).
					Msg("Replayer goroutine failed")
				return err
			}
			return nil
		})

		msgPools = make([]msgpool.Pool, uint(cfg.ReplayShardId)+1)
	default:
		panic("unsupported run mode")
	}

	if interop != nil {
		interop <- ServiceInterop{MsgPools: msgPools}
	}

	if cfg.RunMode != CollatorsOnlyRunMode {
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
	} else {
		logger.Info().Msg("Starting collators...")
	}

	if err := concurrent.Run(ctx, funcs...); err != nil {
		logger.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return 1
	}

	logger.Info().Msg("App is terminated.")
	return 0
}

func createNetworkManager(ctx context.Context, cfg *Config) (*network.Manager, error) {
	networkConfig := &network.Config{
		TcpPort:  cfg.Libp2pTcpPort,
		QuicPort: cfg.Libp2pQuicPort,
		UseMdns:  cfg.UseMdns,
	}
	if !networkConfig.Enabled() {
		return nil, nil
	}
	return network.NewManager(ctx, networkConfig)
}

type AbstractCollator interface {
	Run(ctx context.Context) error
	GetMsgPool() msgpool.Pool
}

func createCollators(cfg *Config, database db.DB, networkManager *network.Manager) []AbstractCollator {
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)

	collators := make([]AbstractCollator, 0, cfg.NShards)

	for i := range cfg.NShards {
		collator := createActiveCollator(i, cfg, collatorTickPeriod, database, networkManager)
		collators = append(collators, collator)
	}
	return collators
}

func createActiveCollator(i int, cfg *Config, collatorTickPeriod time.Duration, database db.DB, networkManager *network.Manager) *collate.Scheduler {
	msgPool := msgpool.New(msgpool.DefaultConfig)
	collatorCfg := collate.Params{
		BlockGeneratorParams: execution.BlockGeneratorParams{
			ShardId:       types.ShardId(i),
			NShards:       cfg.NShards,
			TraceEVM:      cfg.TraceEVM,
			Timer:         common.NewTimer(),
			GasBasePrice:  types.NewValueFromUint64(cfg.GasBasePrice),
			GasPriceScale: cfg.GasPriceScale,
		},
		CollatorTickPeriod: collatorTickPeriod,
		Timeout:            collatorTickPeriod,
		ZeroState:          execution.DefaultZeroStateConfig,
		MainKeysOutPath:    cfg.MainKeysOutPath,
		Topology:           collate.GetShardTopologyById(cfg.Topology),
	}
	if len(cfg.ZeroState) != 0 {
		collatorCfg.ZeroState = cfg.ZeroState
	}

	collator := collate.NewScheduler(database, msgPool, collatorCfg, networkManager)
	return collator
}
