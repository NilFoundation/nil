package nilservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/consensus/ibft"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/admin"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/indexer"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
	"github.com/NilFoundation/nil/nil/services/rollup"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

// syncer will pull blocks actively if no blocks appear for 5 rounds
const defaultSyncTimeoutFactor = 5

func startRpcServer(
	ctx context.Context,
	cfg *Config,
	rawApi rawapi.NodeApi,
	db db.ReadOnlyDB,
	client client.Client,
) error {
	logger := logging.NewLogger("RPC").With().
		Int(logging.FieldRpcPort, cfg.RPCPort).
		Logger()

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
		KeepHeaders:     []string{"Client-Version", "Client-Type", "X-UID"},
	}

	ctx, cancel := context.WithCancel(ctx)
	pollBlocksForLogs := cfg.RunMode == NormalRunMode

	var ethApiService any
	if cfg.RunMode == NormalRunMode || cfg.RunMode == RpcRunMode {
		ethImpl := jsonrpc.NewEthAPI(ctx, rawApi, db, pollBlocksForLogs, cfg.LogClientRpcEvents)
		defer ethImpl.Shutdown()
		ethApiService = ethImpl
	} else {
		ethImpl := jsonrpc.NewEthAPIRo(ctx, rawApi, db, pollBlocksForLogs, cfg.LogClientRpcEvents)
		defer ethImpl.Shutdown()
		ethApiService = ethImpl
	}
	defer cancel()

	debugImpl := jsonrpc.NewDebugAPI(rawApi, logger)
	web3Impl := jsonrpc.NewWeb3API(rawApi)

	txpoolImpl := jsonrpc.NewTxPoolAPI(rawApi, logger)

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
		{
			Namespace: "web3",
			Public:    true,
			Service:   jsonrpc.Web3API(web3Impl),
			Version:   "1.0",
		},
		{
			Namespace: "txpool",
			Public:    true,
			Service:   jsonrpc.TxPoolAPI(txpoolImpl),
			Version:   "1.0",
		},
	}

	if cfg.EnableDevApi {
		devImpl := jsonrpc.NewDevAPI(rawApi)
		apiList = append(apiList, transport.API{
			Namespace: "dev",
			Public:    true,
			Service:   devImpl,
			Version:   "1.0",
		})
	}

	if cfg.Cometa != nil {
		cmt, err := cometa.NewService(ctx, cfg.Cometa, client)
		if err != nil {
			return fmt.Errorf("failed to create cometa service: %w", err)
		}
		apiList = append(apiList, cmt.GetRpcApi())
	}

	if cfg.Indexer != nil {
		idx, err := indexer.NewService(ctx, cfg.Indexer)
		if err != nil {
			return fmt.Errorf("failed to create indexer service: %w", err)
		}
		apiList = append(apiList, idx.GetRpcApi())

		check.PanicIfErr(err)
		task := concurrent.MakeTask(
			"indexer",
			func(ctx context.Context) (err error) {
				return indexer.StartIndexer(ctx, &indexer.Cfg{
					Client:        client,
					IndexerDriver: idx.Driver,
					BlocksChan:    make(chan *driver.BlockWithShardId, 1000),
				})
			})
		if err := concurrent.Run(ctx, task); err != nil {
			return err
		}
	}

	if cfg.IsFaucetApiEnabled() {
		f, err := faucet.NewService(client)
		if err != nil {
			return fmt.Errorf("failed to create faucet service: %w", err)
		}
		apiList = append(apiList, f.GetRpcApi())
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

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger, nil)
}

func startAdminServer(ctx context.Context, cfg *Config) error {
	return admin.StartAdminServer(ctx,
		&admin.ServerConfig{
			Enabled:        cfg.AdminSocketPath != "",
			UnixSocketPath: cfg.AdminSocketPath,
		},
		logging.NewLogger("admin"))
}

const defaultCollatorTickPeriodMs = 2000

// used to access started service from outside of `Run` call
type ServiceInterop struct {
	TxnPools map[types.ShardId]txnpool.Pool
}

func getRawApi(
	cfg *Config,
	networkManager network.Manager,
	database db.DB,
	txnPools map[types.ShardId]txnpool.Pool,
) rawapi.NodeApi {
	nodeApiBuilder := rawapi.NodeApiBuilder(database, networkManager)

	switch cfg.RunMode {
	case RpcRunMode:
		for shardId := range types.ShardId(cfg.NShards) {
			nodeApiBuilder.
				WithNetworkShardApiClientRo(shardId).
				WithNetworkShardApiClientRw(shardId).
				WithNetworkShardApiClientDev(shardId)
		}

	case ArchiveRunMode:
		for shardId := range types.ShardId(cfg.NShards) {
			nodeApiBuilder.WithLocalShardApiRo(shardId)
		}

	case NormalRunMode:
		for shardId := range types.ShardId(cfg.NShards) {
			nodeApiBuilder.WithLocalShardApiRo(shardId)
			if cfg.IsShardActive(shardId) {
				nodeApiBuilder.WithLocalShardApiRw(shardId, txnPools[shardId])
			}
			if cfg.EnableDevApi {
				nodeApiBuilder.WithLocalShardApiDev(shardId)
			}
		}

	case BlockReplayRunMode:
		nodeApiBuilder.WithLocalShardApiRo(cfg.Replay.ShardId)

	case CollatorsOnlyRunMode:
		return nil
	}

	return nodeApiBuilder.BuildAndReset()
}

func validateArchiveNodeConfig(_ *Config, nm network.Manager) error {
	if nm == nil {
		return errors.New("failed to start archive node without network configuration")
	}
	return nil
}

func initSyncers(ctx context.Context, syncers []*collate.Syncer, allowDbDrop bool) error {
	if err := syncers[0].Init(ctx, allowDbDrop); err != nil {
		return err
	}
	for _, syncer := range syncers {
		if err := syncer.GenerateZerostateIfShardIsEmpty(ctx); err != nil {
			return err
		}
	}
	return nil
}

func getSyncerConfig(name string, cfg *Config, shardId types.ShardId) *collate.SyncerConfig {
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)
	syncerTimeout := collatorTickPeriod * time.Duration(cfg.SyncTimeoutFactor)

	return &collate.SyncerConfig{
		Name:                 name,
		ShardId:              shardId,
		Timeout:              syncerTimeout,
		BootstrapPeers:       cfg.BootstrapPeers,
		BlockGeneratorParams: cfg.BlockGeneratorParams(shardId),
		ZeroStateConfig:      cfg.ZeroState,
	}
}

type syncersResult struct {
	funcs   []concurrent.Task
	syncers []*collate.Syncer
	wgInit  sync.WaitGroup
	result  error
}

func (s *syncersResult) Wait() error {
	s.wgInit.Wait()
	return s.result
}

func createSyncers(
	name string,
	cfg *Config,
	validators []*collate.Validator,
	nm network.Manager,
	database db.DB,
	logger logging.Logger,
) (*syncersResult, error) {
	res := &syncersResult{
		funcs:   make([]concurrent.Task, 0, cfg.NShards+2),
		syncers: make([]*collate.Syncer, 0, cfg.NShards),
	}
	res.wgInit.Add(1)

	for i := range cfg.NShards {
		shardId := types.ShardId(i)
		syncerConfig := getSyncerConfig(name, cfg, shardId)
		syncer, err := collate.NewSyncer(syncerConfig, validators[i], database, nm)
		if err != nil {
			return nil, err
		}
		res.syncers = append(res.syncers, syncer)
		res.funcs = append(res.funcs, concurrent.MakeTask(
			fmt.Sprintf("[%d] syncer", i),
			func(ctx context.Context) error {
				if err := res.Wait(); err != nil { // Wait for syncers initialization
					return err
				}
				if err := syncer.Run(ctx); err != nil {
					logger.Error().
						Err(err).
						Stringer(logging.FieldShardId, shardId).
						Msg("Syncer goroutine failed")
					return err
				}
				return nil
			}))
	}
	res.funcs = append(res.funcs, concurrent.MakeTask(
		"init syncers",
		func(ctx context.Context) (err error) {
			defer func() {
				res.result = err
				res.wgInit.Done()
			}()
			if err = initSyncers(ctx, res.syncers, cfg.AllowDbDrop); err != nil {
				logger.Error().Err(err).Msg("Failed to initialize syncers")
			}
			return
		}))
	res.funcs = append(res.funcs, concurrent.MakeTask(
		"set syncer handlers",
		func(ctx context.Context) error {
			for _, syncer := range res.syncers {
				if err := syncer.WaitComplete(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					return err
				}
			}
			return res.syncers[0].SetHandlers(ctx)
		}))

	return res, nil
}

type Node struct {
	NetworkManager network.Manager
	funcs          []concurrent.Task
	logger         logging.Logger
	ctx            context.Context
}

func (i *Node) Run() error {
	if err := concurrent.Run(i.ctx, i.funcs...); err != nil {
		var executionErr *concurrent.ExecutionError
		var protocolVersionMismatchErr *collate.ProtocolVersionMismatchError
		if errors.As(err, &executionErr) && errors.As(executionErr.Err, &protocolVersionMismatchErr) {
			i.logger.Error().
				Str("localVersion", protocolVersionMismatchErr.LocalVersion).
				Str("remoteVersion", protocolVersionMismatchErr.RemoteVersion).
				Msg("Protocol version mismatch. Probably nild executable is outdated.")
		} else {
			i.logger.Error().Err(err).Msg("App encountered an error and will be terminated.")
		}
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

func runNormalOrCollatorsOnly(
	ctx context.Context,
	funcs []concurrent.Task,
	cfg *Config,
	database db.DB,
	networkManager network.Manager,
	logger logging.Logger,
) ([]concurrent.Task, map[types.ShardId]txnpool.Pool, error) {
	if err := cfg.LoadValidatorKeys(); err != nil {
		return nil, nil, err
	}

	if !cfg.SplitShards && len(cfg.ZeroState.GetValidators()) == 0 {
		if err := initDefaultValidator(cfg); err != nil {
			return nil, nil, err
		}
	}

	validators, err := createValidators(ctx, cfg, database, networkManager)
	if err != nil {
		return nil, nil, err
	}

	syncersResult, err := createSyncers("sync", cfg, validators, networkManager, database, logger)
	if err != nil {
		return nil, nil, err
	}
	funcs = append(funcs, syncersResult.funcs...)

	shardFuncs, err := createShards(cfg, validators, syncersResult, database, networkManager, logger)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create collators")
		return nil, nil, err
	}

	txPools := make(map[types.ShardId]txnpool.Pool)
	for shardId, validator := range validators {
		if pool := validator.TxPool(); pool != nil {
			var ok bool
			txPools[types.ShardId(shardId)], ok = pool.(*txnpool.TxnPool)
			check.PanicIfNot(ok)
		}
	}

	funcs = append(funcs, shardFuncs...)
	return funcs, txPools, nil
}

func CreateNode(
	ctx context.Context,
	name string,
	cfg *Config,
	database db.DB,
	interop chan<- ServiceInterop,
	workers ...concurrent.Task,
) (*Node, error) {
	logger := logging.NewLogger(name)

	if err := cfg.Validate(); err != nil {
		logger.Error().Err(err).Msg("Configuration is invalid")
		return nil, err
	}

	if cfg.EnableConfigCache {
		if err := config.InitGlobalConfigCache(cfg.NShards, database); err != nil {
			logger.Error().Err(err).Msg("Failed to initialize global config cache")
			return nil, err
		}
	}

	if err := telemetry.Init(ctx, cfg.Telemetry); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize telemetry")
		return nil, err
	}

	if cfg.L1Fetcher == nil && (cfg.RunMode == NormalRunMode || cfg.RunMode == CollatorsOnlyRunMode) {
		cfg.L1Fetcher = rollup.NewL1BlockFetcherRpc(ctx)
	}

	funcs := make([]concurrent.Task, 0, int(cfg.NShards)+2+len(workers))

	if cfg.CollatorTickPeriodMs == 0 {
		cfg.CollatorTickPeriodMs = defaultCollatorTickPeriodMs
	}

	if cfg.SyncTimeoutFactor == 0 {
		cfg.SyncTimeoutFactor = defaultSyncTimeoutFactor
	}

	if cfg.ZeroState == nil {
		var err error
		cfg.ZeroState, err = execution.CreateDefaultZeroStateConfig(nil)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to create default zero state config")
			return nil, err
		}
	}

	createNetworkManager := cfg.NetworkManagerFactory
	if createNetworkManager == nil {
		createNetworkManager = CreateNetworkManager
	}
	networkManager, err := createNetworkManager(ctx, cfg, database)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create network manager")
		return nil, err
	}

	var txnPools map[types.ShardId]txnpool.Pool
	var syncersResult *syncersResult
	switch cfg.RunMode {
	case NormalRunMode, CollatorsOnlyRunMode:
		funcs, txnPools, err = runNormalOrCollatorsOnly(ctx, funcs, cfg, database, networkManager, logger)
		if err != nil {
			return nil, err
		}
	case ArchiveRunMode:
		if err := validateArchiveNodeConfig(cfg, networkManager); err != nil {
			logger.Error().Err(err).Msg("Invalid configuration")
			return nil, err
		}
		validators, err := createValidators(ctx, cfg, database, networkManager)
		if err != nil {
			return nil, err
		}
		syncersResult, err = createSyncers("archive-sync", cfg, validators, networkManager, database, logger)
		if err != nil {
			return nil, err
		}
		funcs = append(funcs, syncersResult.funcs...)
	case BlockReplayRunMode:
		replayer := collate.NewReplayScheduler(database, collate.ReplayParams{
			BlockGeneratorParams: cfg.BlockGeneratorParams(cfg.Replay.ShardId),
			Timeout:              time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs),
			ReplayFirstBlock:     cfg.Replay.BlockIdFirst,
			ReplayLastBlock:      cfg.Replay.BlockIdLast,
		})

		funcs = append(funcs, concurrent.MakeTask(
			"block-replay",
			func(ctx context.Context) error {
				if err := replayer.Run(ctx); err != nil {
					logger.Error().
						Err(err).
						Stringer(logging.FieldShardId, cfg.Replay.ShardId).
						Msg("Replayer goroutine failed")
					return err
				}
				return nil
			}))
	case RpcRunMode:
		if networkManager == nil {
			err := errors.New("failed to start rpc node without network configuration")
			logger.Error().Err(err).Send()
			return nil, err
		}
		funcs = append(funcs, concurrent.MakeTask(
			"connect to peers",
			func(ctx context.Context) error {
				network.ConnectToPeers(ctx, cfg.RpcNode.ArchiveNodeList, networkManager, logger)
				return nil
			}))
	default:
		panic("unsupported run mode")
	}

	if interop != nil {
		interop <- ServiceInterop{TxnPools: txnPools}
	}

	funcs = append(funcs, concurrent.MakeTask(
		"admin-api",
		func(ctx context.Context) error {
			if err := startAdminServer(ctx, cfg); err != nil {
				logger.Error().Err(err).Msg("Admin server goroutine failed")
				return err
			}
			return nil
		}))

	rawApi := getRawApi(cfg, networkManager, database, txnPools)
	funcs = addRpcServerWorkerIfEnabled(funcs, cfg, rawApi, syncersResult, database, logger)

	if cfg.RunMode != CollatorsOnlyRunMode && cfg.RunMode != RpcRunMode {
		if err := rawApi.SetP2pRequestHandlers(ctx, networkManager, logger); err != nil {
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

func addRpcServerWorkerIfEnabled(
	tasks []concurrent.Task,
	cfg *Config,
	rawApi rawapi.NodeApi,
	syncersResult *syncersResult,
	database db.DB,
	logger logging.Logger,
) []concurrent.Task {
	if (cfg.RPCPort == 0 && cfg.HttpUrl == "") || rawApi == nil {
		return tasks
	}

	return append(tasks, concurrent.MakeTask(
		"rpc-api",
		func(ctx context.Context) error {
			if syncersResult != nil {
				if err := syncersResult.Wait(); err != nil { // Wait for syncers initialization
					return err
				}
			}

			var cl client.Client
			if cfg.Cometa != nil || cfg.IsFaucetApiEnabled() || cfg.Indexer != nil {
				var err error
				cl, err = client.NewEthClient(ctx, database, rawApi, logger)
				if err != nil {
					return fmt.Errorf("failed to create node client: %w", err)
				}
			}
			if err := startRpcServer(ctx, cfg, rawApi, database, cl); err != nil {
				logger.Error().Err(err).Msg("RPC server goroutine failed")
				return err
			}
			return nil
		}))
}

// Run starts transaction pools and collators for given shards, creates a single RPC server for all shards.
// It waits until one of the events:
//   - all goroutines finish successfully,
//   - a goroutine returns an error,
//   - SIGTERM or SIGINT is caught.
//
// It returns a value suitable for os.Exit().
func Run(
	ctx context.Context,
	cfg *Config,
	database db.DB,
	interop chan<- ServiceInterop,
	workers ...concurrent.Task,
) int {
	if cfg.GracefulShutdown {
		signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		ctx = signalCtx
	}

	logging.ApplyComponentsFilterEnv()

	node, err := CreateNode(ctx, "nil", cfg, database, interop, workers...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create node: %s", err.Error())
		return 1
	}
	defer node.Close(ctx)

	if err := node.Run(); err != nil {
		return 1
	}
	return 0
}

func CreateNetworkManager(ctx context.Context, cfg *Config, database db.DB) (network.Manager, error) {
	if cfg.Network != nil && cfg.RunMode != NormalRunMode {
		cfg.Network.DHTMode = dht.ModeClient
	}

	if cfg.RunMode == RpcRunMode {
		return network.NewClientManager(ctx, cfg.Network, database)
	}

	if cfg.Network == nil || !cfg.Network.Enabled() {
		return nil, nil
	}

	return network.NewManager(ctx, cfg.Network, database)
}

func initDefaultValidator(cfg *Config) error {
	if cfg.ValidatorKeysManager == nil {
		return errors.New("validator keys manager is nil")
	}
	pubkey, err := cfg.ValidatorKeysManager.GetPublicKey()
	if err != nil {
		return err
	}
	validators := make([]config.ListValidators, cfg.NShards-1)
	for i := range validators {
		validators[i] = config.ListValidators{List: []config.ValidatorInfo{{PublicKey: config.Pubkey(pubkey)}}}
	}
	cfg.ZeroState.ConfigParams.Validators = config.ParamValidators{Validators: validators}
	return nil
}

func createValidators(
	ctx context.Context,
	cfg *Config,
	database db.DB,
	networkManager network.Manager,
) ([]*collate.Validator, error) {
	collatorTickPeriod := time.Millisecond * time.Duration(cfg.CollatorTickPeriodMs)

	list := make([]*collate.Validator, cfg.NShards)
	for i := range cfg.NShards {
		shardId := types.ShardId(i)
		params := createCollateParams(shardId, cfg, collatorTickPeriod)

		var err error
		var txpool *txnpool.TxnPool
		if cfg.IsShardActive(shardId) {
			txpool, err = txnpool.New(ctx, txnpool.NewConfig(shardId), networkManager)
			if err != nil {
				return nil, err
			}
		}

		list[i], err = collate.NewValidator(params, list[0], database, txpool, networkManager)
		if err != nil {
			return nil, err
		}
	}
	return list, nil
}

func createShards(
	cfg *Config,
	validators []*collate.Validator,
	syncers *syncersResult,
	database db.DB,
	networkManager network.Manager,
	logger logging.Logger,
) ([]concurrent.Task, error) {
	funcs := make([]concurrent.Task, 0, cfg.NShards)

	validatorsNum := len(cfg.ZeroState.GetValidators())
	if validatorsNum != int(cfg.NShards)-1 {
		return nil, fmt.Errorf("number of shards mismatch in the config, expected %d, got %d",
			cfg.NShards-1, validatorsNum)
	}

	for i := range cfg.NShards {
		shardId := types.ShardId(i)

		if cfg.IsShardActive(shardId) {
			pKey, err := cfg.ValidatorKeysManager.GetKey()
			if err != nil {
				return nil, err
			}

			consensus, err := ibft.NewConsensus(&ibft.ConsensusParams{
				ShardId:    shardId,
				Db:         database,
				Validator:  validators[i],
				NetManager: networkManager,
				PrivateKey: pKey,
			})
			if err != nil {
				return nil, err
			}
			collator := collate.NewScheduler(validators[i], database, consensus, networkManager)

			funcs = append(funcs, concurrent.MakeTask(
				fmt.Sprintf("[%d] collator", i),
				func(ctx context.Context) error {
					if err := syncers.Wait(); err != nil { // Wait for syncers initialization
						return err
					}
					if err := consensus.Init(ctx); err != nil {
						return err
					}
					if err := collator.Run(ctx); err != nil {
						logger.Error().
							Err(err).
							Stringer(logging.FieldShardId, shardId).
							Msg("Collator goroutine failed")
						return err
					}
					return nil
				}))
		} else if networkManager == nil {
			return nil, errors.New("trying to start syncer without network configuration")
		}
	}
	return funcs, nil
}

func createCollateParams(shard types.ShardId, cfg *Config, collatorTickPeriod time.Duration) *collate.Params {
	return &collate.Params{
		BlockGeneratorParams: cfg.BlockGeneratorParams(shard),
		CollatorTickPeriod:   collatorTickPeriod,
		Timeout:              collatorTickPeriod,
		Topology:             collate.GetShardTopologyById(cfg.Topology),
		L1Fetcher:            cfg.L1Fetcher,
	}
}
