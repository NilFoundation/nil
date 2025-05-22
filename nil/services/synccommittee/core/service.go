package core

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/constraints"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/bridgecontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/feeupdater"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/syncer"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/l1client"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/jonboulle/clockwork"
)

type SyncCommittee struct {
	srv.Service
}

func New(ctx context.Context, cfg *Config, database db.DB) (*SyncCommittee, error) {
	logger := logging.NewLogger("sync_committee")

	if err := telemetry.Init(ctx, cfg.Telemetry); err != nil {
		logger.Error().Err(err).Msg("failed to initialize telemetry")
		return nil, err
	}
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics: %w", err)
	}

	logger.Info().Msgf("Use RPC endpoint %v", cfg.NilRpcEndpoint)

	nilClient := rpc.NewRetryClient(cfg.NilRpcEndpoint, logger)

	fetcher := fetching.NewFetcher(
		nilClient,
		logger,
	)

	clock := clockwork.NewRealClock()
	blockStorage := storage.NewBlockStorage(
		database, storage.DefaultBlockStorageConfig(), clock, metricsHandler, logger)
	taskStorage := storage.NewTaskStorage(database, clock, metricsHandler, logger)

	rollupContractWrapper, err := rollupcontract.NewWrapper(
		ctx,
		cfg.ContractWrapperConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing rollup contract wrapper: %w", err)
	}

	stateRootSyncer := syncer.NewStateRootSyncer(
		fetcher, rollupContractWrapper, blockStorage, logger, syncer.NewConfig(!cfg.ContractWrapperConfig.DisableL1),
	)
	syncRunner := syncer.NewRunner(stateRootSyncer, logger)
	// todo: add reset logic to TaskStorage
	//  and pass it here in https://github.com/NilFoundation/nil/pull/419

	syncCommittee := &SyncCommittee{}
	resetLauncher := reset.NewResetLauncher(blockStorage, stateRootSyncer, syncCommittee, logger)

	batchChecker := constraints.NewChecker(
		constraints.DefaultBatchConstraints(),
		clock,
		logger,
	)

	agg := fetching.NewAggregator(
		fetcher,
		batchChecker,
		blockStorage,
		taskStorage,
		resetLauncher,
		rollupContractWrapper,
		clock,
		logger,
		metricsHandler,
		cfg.AggregatorConfig,
	)

	lagTracker := fetching.NewLagTracker(
		fetcher, blockStorage, metricsHandler, fetching.NewDefaultLagTrackerConfig(), logger,
	)

	l2BridgeMessengerAddress := types.HexToAddress(cfg.L2BridgeMessegerAdddress)

	bridgeStateGetter := bridgecontract.NewBridgeStateGetter(
		nilClient,
		l2BridgeMessengerAddress,
	)

	proposer, err := NewProposer(
		cfg.ProposerParams,
		blockStorage,
		bridgeStateGetter,
		rollupContractWrapper,
		resetLauncher,
		metricsHandler,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposer: %w", err)
	}

	resetLauncher.AddPausableComponent(agg)
	resetLauncher.AddPausableComponent(proposer)

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(blockStorage, resetLauncher, logger),
		metricsHandler,
		logger,
	)

	blockDebugger := debug.NewBlockDebugger(rollupContractWrapper, blockStorage)

	rpcServer := rpc.NewServerWithTasks(
		rpc.NewServerConfig(cfg.OwnRpcEndpoint),
		logger,
		taskScheduler,
		debug.NewTaskDebugger(taskStorage, logger),
		rpc.DebugBlocksServerHandler(blockDebugger),
	)

	feeUpdaterMetrics, err := metrics.NewFeeUpdaterMetrics()
	if err != nil {
		return nil, err
	}

	l1Client, err := l1client.NewRetryingEthClient(
		ctx,
		cfg.ContractWrapperConfig.Endpoint,
		cfg.ContractWrapperConfig.RequestsTimeout,
		logger,
	)
	if err != nil {
		return nil, err
	}

	feeUpdaterContract, err := feeupdater.NewWrapper(ctx, &cfg.L1FeeUpdateContractConfig, l1Client)
	if err != nil {
		return nil, fmt.Errorf("error initializing fee updater contract wrapper: %w", err)
	}
	feeUpdater := feeupdater.NewUpdater(
		cfg.L1FeeUpdateConfig,
		fetcher,
		logger,
		clock,
		feeUpdaterContract,
		feeUpdaterMetrics,
	)

	syncCommittee.Service = srv.NewServiceWithHeartbeat(
		metricsHandler,
		logger,
		syncRunner, proposer, agg, lagTracker, taskScheduler, feeUpdater, rpcServer,
	)

	return syncCommittee, nil
}
