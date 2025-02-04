package nilservice

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/consensus/ibft"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/signer"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/admin"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"
)

// syncer will pull blocks actively if no blocks appear for 5 rounds
const syncTimeoutFactor = 5

func startRpcServer(ctx context.Context, cfg *Config, rawApi rawapi.NodeApi, db db.ReadOnlyDB, client client.Client) error {
	logger := logging.NewLogger("RPC")

	addr := cfg.HttpUrl
	if addr == "" {
		addr = fmt.Sprintf("tcp://127.0.0.1:%d", cfg.RPCPort)
	}

	httpConfig := &httpcfg.HttpCfg{
		HttpURL:         addr,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		HttpCORSDomain:  []string{"*"},
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

	debugImpl := jsonrpc.NewDebugAPI(rawApi, logger)

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

	if cfg.Cometa != nil {
		cmt, err := cometa.NewService(ctx, cfg.Cometa, client)
		if err != nil {
			return fmt.Errorf("failed to create cometa service: %w", err)
		}
		apiList = append(apiList, cmt.GetRpcApi())
	}

	if cfg.IsFaucetApiEnabled() {
		faucet, err := faucet.NewService(client)
		if err != nil {
			return fmt.Errorf("failed to create faucet service: %w", err)
		}
		apiList = append(apiList, faucet.GetRpcApi())
	}

	if cfg.RunMode == NormalRunMode {
		dbImpl := jsonrpc.NewDbAPI(db, logger)
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
	TxnPools map[types.ShardId]txnpool.Pool
}

func getRawApi(cfg *Config, networkManager *network.Manager, database db.DB, txnPools map[types.ShardId]txnpool.Pool) (*rawapi.NodeApiOverShardApis, error) {
	var myShards []uint
	switch cfg.RunMode {
	case BlockReplayRunMode:
		txnPools = nil
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
			shardApis[shardId] = rawapi.NewLocalShardApi(shardId, database, txnPools[shardId])
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
		if err := rawapi.SetShardApiAsP2pRequestHandlersIfAllowed(api, ctx, networkManager, readonly, logger); err != nil {
			logger.Error().Err(err).Stringer(logging.FieldShardId, shardId).Msg("Failed to set raw API request handler")
			return err
		}
	}
	return nil
}

func validateArchiveNodeConfig(cfg *Config, nm *network.Manager) error {
	if nm == nil {
		return errors.New("Failed to start archive node without network configuration")
	}
	if len(cfg.BootstrapPeers) > 0 && len(cfg.BootstrapPeers) != int(cfg.NShards) {
		return errors.New("On archive node, number of bootstrap peers must be equal to the number of shards")
	}
	if !slices.Contains(cfg.MyShards, uint(types.MainShardId)) {
		return errors.New("On archive node, main shard must be included in MyShards")
	}
	return nil
}

func createArchiveSyncers(cfg *Config, nm *network.Manager, database db.DB, logger zerolog.Logger) ([]concurrent.Func, error) {
	if err := validateArchiveNodeConfig(cfg, nm); err != nil {
		logger.Error().Err(err).Msg("Invalid configuration")
		return nil, err
	}

	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)
	syncerTimeout := syncTimeoutFactor * collatorTickPeriod

	var wgFetch sync.WaitGroup
	wgFetch.Add(int(cfg.NShards))

	funcs := make([]concurrent.Func, 0, cfg.NShards)

	for i := range cfg.NShards {
		shardId := types.ShardId(i)

		var bootstrapPeer *network.AddrInfo
		if len(cfg.BootstrapPeers) > 0 {
			bootstrapPeer = &cfg.BootstrapPeers[i]
		}

		zeroState := execution.DefaultZeroStateConfig
		zeroStateConfig := cfg.ZeroState
		if len(cfg.ZeroStateYaml) != 0 {
			zeroState = cfg.ZeroStateYaml
		}

		blockVerifier := signer.NewBlockVerifier(shardId, cfg.Validators[shardId])

		syncer, err := collate.NewSyncer(collate.SyncerConfig{
			ShardId:              shardId,
			Timeout:              syncerTimeout,
			BootstrapPeer:        bootstrapPeer,
			ReplayBlocks:         cfg.IsShardActive(shardId),
			BlockGeneratorParams: cfg.BlockGeneratorParams(shardId),
			ZeroState:            zeroState,
			ZeroStateConfig:      zeroStateConfig,
			BlockVerifier:        blockVerifier,
		}, database, nm)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, func(ctx context.Context) error {
			if err := syncer.Run(ctx, &wgFetch); err != nil {
				logger.Error().
					Err(err).
					Stringer(logging.FieldShardId, shardId).
					Msg("Syncer goroutine failed")
				return err
			}
			return nil
		})
	}
	return funcs, nil
}

type Node struct {
	NetworkManager *network.Manager
	funcs          []concurrent.Func
	logger         zerolog.Logger
	ctx            context.Context
}

func (i *Node) Run() error {
	if err := concurrent.Run(i.ctx, i.funcs...); err != nil {
		i.logger.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return err
	}
	i.logger.Info().Msg("App is terminated.")
	return nil
}

func (i *Node) Close(ctx context.Context) {
	if i.NetworkManager != nil {
		i.NetworkManager.Close()
	}
	telemetry.Shutdown(ctx)
}

func CreateNode(ctx context.Context, name string, cfg *Config, database db.DB, interop chan<- ServiceInterop, workers ...concurrent.Func) (*Node, error) {
	logger := logging.NewLogger(name)

	if err := cfg.Validate(); err != nil {
		logger.Error().Err(err).Msg("Configuration is invalid")
		return nil, err
	}

	if err := telemetry.Init(ctx, cfg.Telemetry); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize telemetry")
		return nil, err
	}

	funcs := make([]concurrent.Func, 0, int(cfg.NShards)+2+len(workers))

	if cfg.CollatorTickPeriodMs == 0 {
		cfg.CollatorTickPeriodMs = defaultCollatorTickPeriodMs
	}

	var txnPools map[types.ShardId]txnpool.Pool
	if cfg.Network != nil && cfg.RunMode != NormalRunMode {
		cfg.Network.DHTMode = dht.ModeClient
	}
	networkManager, err := createNetworkManager(ctx, cfg)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create network manager")
		return nil, err
	}

	switch cfg.RunMode {
	case NormalRunMode, CollatorsOnlyRunMode:
		var shardFuncs []concurrent.Func
		shardFuncs, txnPools, err = createShards(ctx, cfg, database, networkManager, logger)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to create collators")
			return nil, err
		}

		funcs = append(funcs, shardFuncs...)
	case ArchiveRunMode:
		archiveNodeFunc, err := createArchiveSyncers(cfg, networkManager, database, logger)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, archiveNodeFunc...)
	case BlockReplayRunMode:
		replayer := collate.NewReplayScheduler(database, collate.ReplayParams{
			BlockGeneratorParams: cfg.BlockGeneratorParams(cfg.Replay.ShardId),
			Timeout:              time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs),
			ReplayFirstBlock:     cfg.Replay.BlockIdFirst,
			ReplayLastBlock:      cfg.Replay.BlockIdLast,
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
			err := errors.New("Failed to start rpc node without network configuration")
			logger.Error().Err(err).Send()
			return nil, err
		}
		funcs = append(funcs, func(ctx context.Context) error {
			network.ConnectToPeers(ctx, cfg.RpcNode.ArchiveNodeList, *networkManager, logger)
			return nil
		})
	default:
		panic("unsupported run mode")
	}

	if interop != nil {
		interop <- ServiceInterop{TxnPools: txnPools}
	}

	funcs = append(funcs, func(ctx context.Context) error {
		if err := startAdminServer(ctx, cfg); err != nil {
			logger.Error().Err(err).Msg("Admin server goroutine failed")
			return err
		}
		return nil
	})

	rawApi, err := getRawApi(cfg, networkManager, database, txnPools)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create raw API")
		return nil, err
	}

	if (cfg.RPCPort != 0 || cfg.HttpUrl != "") && rawApi != nil {
		funcs = append(funcs, func(ctx context.Context) error {
			var cl client.Client
			if cfg.Cometa != nil || cfg.IsFaucetApiEnabled() {
				cl, err = client.NewEthClient(ctx, database, types.ShardId(cfg.NShards), txnPools, logger)
				if err != nil {
					return fmt.Errorf("failed to create node client: %w", err)
				}
			}
			if err := startRpcServer(ctx, cfg, rawApi, database, cl); err != nil {
				logger.Error().Err(err).Msg("RPC server goroutine failed")
				return err
			}
			return nil
		})
	}

	if cfg.RunMode != CollatorsOnlyRunMode && cfg.RunMode != RpcRunMode {
		readonly := cfg.RunMode != NormalRunMode
		if err := setP2pRequestHandlers(ctx, rawApi, networkManager, readonly, logger); err != nil {
			return nil, err
		}

		funcs = append(funcs, workers...)

		logger.Info().Msg("Starting services...")
	} else {
		logger.Info().Msg("Starting collators...")
	}

	return &Node{
		NetworkManager: networkManager,
		funcs:          funcs,
		logger:         logger,
		ctx:            ctx,
	}, nil
}

// Run starts transaction pools and collators for given shards, creates a single RPC server for all shards.
// It waits until one of the events:
//   - all goroutines finish successfully,
//   - a goroutine returns an error,
//   - SIGTERM or SIGINT is caught.
//
// It returns a value suitable for os.Exit().
func Run(ctx context.Context, cfg *Config, database db.DB, interop chan<- ServiceInterop, workers ...concurrent.Func) int {
	if cfg.GracefulShutdown {
		signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		ctx = signalCtx
	}

	node, err := CreateNode(ctx, "nil", cfg, database, interop, workers...)
	if err != nil {
		return 1
	}
	defer node.Close(ctx)

	if err := node.Run(); err != nil {
		return 1
	}
	return 0
}

func createNetworkManager(ctx context.Context, cfg *Config) (*network.Manager, error) {
	if cfg.RunMode == RpcRunMode {
		return network.NewClientManager(ctx, cfg.Network)
	}

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

func createShards(
	ctx context.Context, cfg *Config,
	database db.DB, networkManager *network.Manager,
	logger zerolog.Logger,
) ([]concurrent.Func, map[types.ShardId]txnpool.Pool, error) {
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)
	syncerTimeout := syncTimeoutFactor * collatorTickPeriod

	funcs := make([]concurrent.Func, 0, 2*cfg.NShards)
	pools := make(map[types.ShardId]txnpool.Pool)

	var wgFetch sync.WaitGroup
	wgFetch.Add(int(cfg.NShards))

	pKey, err := cfg.LoadValidatorPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	if !cfg.SplitShards && len(cfg.Validators) == 0 {
		pubkey, err := cfg.ValidatorKeysManager.GetPublicKey()
		if err != nil {
			return nil, nil, err
		}
		for i := range cfg.NShards {
			cfg.Validators[types.ShardId(i)] = []config.ValidatorInfo{
				{PublicKey: config.Pubkey(pubkey)},
			}
		}
	}

	for i := range cfg.NShards {
		shardId := types.ShardId(i)

		blockVerifier := signer.NewBlockVerifier(shardId, cfg.Validators[shardId])

		zeroState := execution.DefaultZeroStateConfig
		zeroStateConfig := cfg.ZeroState
		if len(cfg.ZeroStateYaml) != 0 {
			zeroState = cfg.ZeroStateYaml
		}

		syncerCfg := collate.SyncerConfig{
			ShardId:              shardId,
			ReplayBlocks:         shardId.IsMainShard() || cfg.IsShardActive(shardId),
			Timeout:              syncerTimeout,
			BlockGeneratorParams: cfg.BlockGeneratorParams(shardId),
			BlockVerifier:        blockVerifier,
			ZeroState:            zeroState,
			ZeroStateConfig:      zeroStateConfig,
		}

		if int(shardId) < len(cfg.BootstrapPeers) {
			syncerCfg.BootstrapPeer = &cfg.BootstrapPeers[shardId]
		}

		syncer, err := collate.NewSyncer(syncerCfg, database, networkManager)
		if err != nil {
			return nil, nil, err
		}

		funcs = append(funcs, func(ctx context.Context) error {
			if err := syncer.Run(ctx, &wgFetch); err != nil {
				logger.Error().
					Err(err).
					Stringer(logging.FieldShardId, shardId).
					Msg("Syncer goroutine failed")
				return err
			}
			return nil
		})

		if cfg.IsShardActive(shardId) {
			txnPool, err := txnpool.New(ctx, txnpool.NewConfig(shardId), networkManager)
			if err != nil {
				return nil, nil, err
			}

			collator := createActiveCollator(shardId, cfg, collatorTickPeriod, database, networkManager, txnPool)

			consensus := ibft.NewConsensus(&ibft.ConsensusParams{
				ShardId:    shardId,
				Db:         database,
				Validator:  collator.Validator(),
				NetManager: networkManager,
				PrivateKey: pKey,
				Validators: cfg.Validators[shardId],
			})
			if err := consensus.Init(ctx); err != nil {
				return nil, nil, err
			}

			pools[shardId] = txnPool
			funcs = append(funcs, func(ctx context.Context) error {
				wgFetch.Wait() // wait for all syncers to avoid data race in the DB
				if err := collator.Run(ctx, syncer, consensus); err != nil {
					logger.Error().
						Err(err).
						Stringer(logging.FieldShardId, shardId).
						Msg("Collator goroutine failed")
					return err
				}
				return nil
			})
		} else if networkManager == nil {
			return nil, nil, errors.New("trying to start syncer without network configuration")
		}
	}

	return funcs, pools, nil
}

func createActiveCollator(shard types.ShardId, cfg *Config, collatorTickPeriod time.Duration, database db.DB, networkManager *network.Manager, txnPool txnpool.Pool) *collate.Scheduler {
	collatorCfg := collate.Params{
		BlockGeneratorParams: execution.BlockGeneratorParams{
			ShardId:         shard,
			NShards:         cfg.NShards,
			TraceEVM:        cfg.TraceEVM,
			Timer:           common.NewTimer(),
			GasBasePrice:    types.NewValueFromUint64(cfg.GasBasePrice),
			GasPriceScale:   cfg.GasPriceScale,
			MainKeysOutPath: cfg.MainKeysOutPath,
		},
		CollatorTickPeriod: collatorTickPeriod,
		Timeout:            collatorTickPeriod,
		ZeroState:          execution.DefaultZeroStateConfig,
		ZeroStateConfig:    cfg.ZeroState,
		Topology:           collate.GetShardTopologyById(cfg.Topology),
	}
	if len(cfg.ZeroStateYaml) != 0 {
		collatorCfg.ZeroState = cfg.ZeroStateYaml
	}

	return collate.NewScheduler(database, txnPool, collatorCfg, networkManager)
}
