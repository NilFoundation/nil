package nilservice

import (
	"context"
	"fmt"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/admin"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

// syncer will pull blocks actively if no blocks appear for 5 rounds
const syncTimeoutFactor = 5

func startRpcServer(ctx context.Context, cfg *Config, rawApi rawapi.NodeApi, db db.ReadOnlyDB) error {
	logger := logging.NewLogger("RPC")

	addr := cfg.HttpUrl
	if addr == "" {
		addr = fmt.Sprintf("tcp://127.0.0.1:%d", cfg.RPCPort)
	}

	httpConfig := &httpcfg.HttpCfg{
		Enabled:         true,
		HttpURL:         addr,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
	}

	ctx, cancel := context.WithCancel(ctx)
	pollBlocksForLogs := cfg.RunMode == NormalRunMode

	var ethApiService any
	if cfg.RunMode == NormalRunMode || cfg.RunMode == RpcRunMode {
		ethImpl := jsonrpc.NewEthAPI(ctx, rawApi, db, pollBlocksForLogs)
		defer ethImpl.Shutdown()
		ethApiService = ethImpl
	} else {
		ethImpl := jsonrpc.NewEthAPIRo(ctx, rawApi, db, pollBlocksForLogs)
		defer ethImpl.Shutdown()
		ethApiService = ethImpl
	}
	defer cancel()

	debugImpl := jsonrpc.NewDebugAPI(rawApi, db, logger)
	dbImpl := jsonrpc.NewDbAPI(db, logger)

	apiList := []transport.API{
		{
			Namespace: "eth",
			Public:    true,
			Service:   ethApiService,
			Version:   "1.0",
		},
		{
			Namespace: "debug",
			Public:    true,
			Service:   jsonrpc.DebugAPI(debugImpl),
			Version:   "1.0",
		},
	}

	if cfg.RunMode == NormalRunMode {
		apiList = append(apiList, transport.API{
			Namespace: "db",
			Public:    true,
			Service:   jsonrpc.DbAPI(dbImpl),
			Version:   "1.0",
		})
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
	MsgPools map[types.ShardId]msgpool.Pool
}

func getRawApi(cfg *Config, networkManager *network.Manager, database db.DB, msgPools map[types.ShardId]msgpool.Pool) (*rawapi.NodeApiOverShardApis, error) {
	var myShards []uint
	switch cfg.RunMode {
	case BlockReplayRunMode:
		msgPools = nil
		fallthrough
	case NormalRunMode, ArchiveRunMode:
		myShards = cfg.GetMyShards()
	case RpcRunMode:
		break
	case CollatorsOnlyRunMode:
		return nil, nil
	default:
		panic("unsupported run mode for raw API")
	}

	shardApis := make(map[types.ShardId]rawapi.ShardApi)
	for shardId := range types.ShardId(cfg.NShards) {
		var err error
		if slices.Contains(myShards, uint(shardId)) {
			shardApis[shardId] = rawapi.NewLocalShardApi(shardId, database, msgPools[shardId])
			if assert.Enable {
				shardApis[shardId], err = rawapi.NewLocalRawApiAccessor(shardId, shardApis[shardId].(*rawapi.LocalShardApi))
			}
		} else {
			shardApis[shardId], err = rawapi.NewNetworkRawApiAccessor(shardId, networkManager)
		}
		if err != nil {
			return nil, err
		}
	}
	rawApi := rawapi.NewNodeApiOverShardApis(shardApis)
	return rawApi, nil
}

func setP2pRequestHandlers(ctx context.Context, rawApi *rawapi.NodeApiOverShardApis, networkManager *network.Manager, readonly bool, logger zerolog.Logger) error {
	if networkManager == nil {
		return nil
	}
	for shardId, api := range rawApi.Apis {
		var shardApi *rawapi.LocalShardApi
		switch api := api.(type) {
		case *rawapi.LocalShardApiAccessor:
			shardApi = api.RawApi
		case *rawapi.LocalShardApi:
			shardApi = api
		}
		if shardApi != nil {
			if err := rawapi.SetRawApiRequestHandlers(ctx, shardId, shardApi, networkManager, readonly, logger); err != nil {
				logger.Error().Err(err).Stringer(logging.FieldShardId, shardId).Msg("Failed to set raw API request handler")
				return err
			}
		}
	}
	return nil
}

// Run starts message pools and collators for given shards, creates a single RPC server for all shards.
// It waits until one of the events:
//   - all goroutines finish successfully,
//   - a goroutine returns an error,
//   - SIGTERM or SIGINT is caught.
//
// It returns a value suitable for os.Exit().
func Run(ctx context.Context, cfg *Config, database db.DB, interop chan<- ServiceInterop, workers ...concurrent.Func) int {
	logger := logging.NewLogger("nil")

	if err := cfg.Validate(); err != nil {
		logger.Error().Err(err).Msg("Configuration is invalid")
		return 1
	}

	if cfg.GracefulShutdown {
		signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		ctx = signalCtx
	}

	if err := telemetry.Init(ctx, cfg.Telemetry); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize telemetry")
		return 1
	}
	defer telemetry.Shutdown(ctx)

	funcs := make([]concurrent.Func, 0, int(cfg.NShards)+2+len(workers))

	if cfg.CollatorTickPeriodMs == 0 {
		cfg.CollatorTickPeriodMs = defaultCollatorTickPeriodMs
	}

	var msgPools map[types.ShardId]msgpool.Pool
	networkManager, err := createNetworkManager(ctx, cfg)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create network manager")
		return 1
	}
	if networkManager != nil {
		defer networkManager.Close()
	}

	switch cfg.RunMode {
	case NormalRunMode, CollatorsOnlyRunMode:
		var collators []AbstractCollator
		collators, msgPools, err = createCollators(ctx, cfg, database, networkManager)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to create collators")
			return 1
		}

		for i, collator := range collators {
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
	case ArchiveRunMode:
		if networkManager == nil {
			logger.Error().Msg("Failed to start archive node without network configuration")
			return 1
		}

		nodeShards := make([]types.ShardId, 0, len(cfg.MyShards)+1)
		for _, shardId := range cfg.MyShards {
			nodeShards = append(nodeShards, types.ShardId(shardId))
		}
		if slices.Contains(nodeShards, types.MainShardId) {
			nodeShards = append(nodeShards, types.MainShardId)
		}

		collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)
		syncerTimeout := syncTimeoutFactor * collatorTickPeriod

		for _, shardId := range nodeShards {
			collator, err := createSyncCollator(ctx, shardId, cfg, syncerTimeout, database, networkManager)
			check.PanicIfErr(err)
			funcs = append(funcs, func(ctx context.Context) error {
				if err := collator.Run(ctx); err != nil {
					logger.Error().
						Err(err).
						Stringer(logging.FieldShardId, shardId).
						Msg("Collator goroutine failed")
					return err
				}
				return nil
			})
		}
	case BlockReplayRunMode:
		replayer := collate.NewReplayScheduler(database, collate.ReplayParams{
			BlockGeneratorParams: execution.BlockGeneratorParams{
				ShardId:       cfg.Replay.ShardId,
				NShards:       cfg.NShards,
				TraceEVM:      cfg.TraceEVM,
				Timer:         common.NewTimer(),
				GasBasePrice:  types.NewValueFromUint64(cfg.GasBasePrice),
				GasPriceScale: cfg.GasPriceScale,
			},
			Timeout:          time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs),
			ReplayFirstBlock: cfg.Replay.BlockIdFirst,
			ReplayLastBlock:  cfg.Replay.BlockIdLast,
		})

		funcs = append(funcs, func(ctx context.Context) error {
			if err := replayer.Run(ctx); err != nil {
				logger.Error().
					Err(err).
					Stringer(logging.FieldShardId, cfg.Replay.ShardId).
					Msg("Replayer goroutine failed")
				return err
			}
			return nil
		})
	case RpcRunMode:
		if networkManager == nil {
			logger.Error().Msg("Failed to start rpc node without network configuration")
			return 1
		}
	default:
		panic("unsupported run mode")
	}

	if interop != nil {
		interop <- ServiceInterop{MsgPools: msgPools}
	}

	funcs = append(funcs, func(ctx context.Context) error {
		if err := startAdminServer(ctx, cfg); err != nil {
			logger.Error().Err(err).Msg("Admin server goroutine failed")
			return err
		}
		return nil
	})

	rawApi, err := getRawApi(cfg, networkManager, database, msgPools)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create raw API")
		return 1
	}

	if (cfg.RPCPort != 0 || cfg.HttpUrl != "") && rawApi != nil {
		funcs = append(funcs, func(ctx context.Context) error {
			if err := startRpcServer(ctx, cfg, rawApi, database); err != nil {
				logger.Error().Err(err).Msg("RPC server goroutine failed")
				return err
			}
			return nil
		})
	}

	if cfg.RunMode != CollatorsOnlyRunMode && cfg.RunMode != RpcRunMode {
		readonly := cfg.RunMode != NormalRunMode
		if err := setP2pRequestHandlers(ctx, rawApi, networkManager, readonly, logger); err != nil {
			return 1
		}

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
	if cfg.Network == nil || !cfg.Network.Enabled() {
		return nil, nil
	}

	if cfg.Network.PrivateKey == nil {
		privKey, err := network.LoadOrGenerateKeys(cfg.NetworkKeysPath)
		if err != nil {
			return nil, err
		}

		cfg.Network.PrivateKey = privKey
	}

	return network.NewManager(ctx, cfg.Network)
}

// todo: get rid of it via separating syncers and collators
type AbstractCollator interface {
	Run(ctx context.Context) error
}

func createCollators(ctx context.Context, cfg *Config, database db.DB, networkManager *network.Manager) ([]AbstractCollator, map[types.ShardId]msgpool.Pool, error) {
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)
	syncerTimeout := syncTimeoutFactor * collatorTickPeriod

	collators := make([]AbstractCollator, cfg.NShards)
	pools := make(map[types.ShardId]msgpool.Pool)

	for i := range cfg.NShards {
		shard := types.ShardId(i)
		if cfg.IsShardActive(shard) {
			msgPool, err := msgpool.New(ctx, msgpool.NewConfig(shard), networkManager)
			if err != nil {
				return nil, nil, err
			}
			collator, err := createActiveCollator(shard, cfg, collatorTickPeriod, database, networkManager, msgPool)
			if err != nil {
				return nil, nil, err
			}

			pools[shard] = msgPool
			collators[i] = collator
		} else {
			collator, err := createSyncCollator(ctx, shard, cfg, syncerTimeout, database, networkManager)
			if err != nil {
				return nil, nil, err
			}
			collators[i] = collator
		}
	}
	return collators, pools, nil
}

func createSyncCollator(ctx context.Context, shard types.ShardId, cfg *Config, tick time.Duration,
	database db.DB, networkManager *network.Manager,
) (AbstractCollator, error) {
	return collate.NewSyncCollator(ctx, shard, tick, database, networkManager, cfg.BootstrapPeer)
}

func createActiveCollator(shard types.ShardId, cfg *Config, collatorTickPeriod time.Duration, database db.DB, networkManager *network.Manager, msgPool msgpool.Pool) (*collate.Scheduler, error) {
	collatorCfg := collate.Params{
		BlockGeneratorParams: execution.BlockGeneratorParams{
			ShardId:       shard,
			NShards:       cfg.NShards,
			TraceEVM:      cfg.TraceEVM,
			Timer:         common.NewTimer(),
			GasBasePrice:  types.NewValueFromUint64(cfg.GasBasePrice),
			GasPriceScale: cfg.GasPriceScale,
		},
		CollatorTickPeriod: collatorTickPeriod,
		Timeout:            collatorTickPeriod,
		ZeroState:          execution.DefaultZeroStateConfig,
		ZeroStateConfig:    cfg.ZeroState,
		MainKeysOutPath:    cfg.MainKeysOutPath,
		Topology:           collate.GetShardTopologyById(cfg.Topology),
	}
	if len(cfg.ZeroStateYaml) != 0 {
		collatorCfg.ZeroState = cfg.ZeroStateYaml
	}

	return collate.NewScheduler(database, msgPool, collatorCfg, networkManager)
}
