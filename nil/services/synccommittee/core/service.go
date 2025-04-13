package core

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
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

	logger.Info().Msgf("Use RPC endpoint %v", cfg.RpcEndpoint)
	client := rpc.NewRetryClient(cfg.RpcEndpoint, logger)

	clock := clockwork.NewRealClock()
	blockStorage := storage.NewBlockStorage(
		database, storage.DefaultBlockStorageConfig(), clock, metricsHandler, logger)
	taskStorage := storage.NewTaskStorage(database, clock, metricsHandler, logger)

	// todo: add reset logic to TaskStorage (implement StateResetter interface)
	//  and pass it here in https://github.com/NilFoundation/nil/pull/419
	stateResetter := reset.NewStateResetter(logger, blockStorage)

	rollupContractWrapper, err := rollupcontract.NewWrapper(
		ctx,
		cfg.ContractWrapperConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing rollup contract wrapper: %w", err)
	}

	agg := fetching.NewAggregator(
		client,
		blockStorage,
		taskStorage,
		stateResetter,
		rollupContractWrapper,
		clock,
		logger,
		metricsHandler,
		cfg.AggregatorConfig,
	)

	lagTracker := fetching.NewLagTracker(
		client, blockStorage, metricsHandler, fetching.NewDefaultLagTrackerConfig(), logger,
	)

	prop, err := NewProposer(
		cfg.ProposerParams,
		blockStorage,
		rollupContractWrapper,
		client,
		metricsHandler,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposer: %w", err)
	}

	syncCommittee := &SyncCommittee{}

	resetLauncher := reset.NewResetLauncher(agg, stateResetter, syncCommittee, logger)

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(blockStorage, resetLauncher, logger),
		metricsHandler,
		logger,
	)

	taskListener := rpc.NewTaskListener(
		&rpc.TaskListenerConfig{HttpEndpoint: cfg.TaskListenerRpcEndpoint},
		taskScheduler,
		logger,
	)

	syncCommittee.Service = srv.NewService(
		logger,
		prop, agg, lagTracker, taskScheduler, taskListener,
	)

	return syncCommittee, nil
}
