package core

import (
	"context"
	"fmt"

	nilrpc "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
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

func New(cfg *Config, database db.DB, ethClient rollupcontract.EthClient) (*SyncCommittee, error) {
	logger := logging.NewLogger("sync_committee")

	if err := telemetry.Init(context.Background(), cfg.Telemetry); err != nil {
		logger.Error().Err(err).Msg("failed to initialize telemetry")
		return nil, err
	}
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics: %w", err)
	}

	logger.Info().Msgf("Use RPC endpoint %v", cfg.RpcEndpoint)
	client := nilrpc.NewClient(cfg.RpcEndpoint, logger)

	clock := clockwork.NewRealClock()
	blockStorage := storage.NewBlockStorage(
		database, storage.DefaultBlockStorageConfig(), clock, metricsHandler, logger)
	taskStorage := storage.NewTaskStorage(database, clock, metricsHandler, logger)

	// todo: add reset logic to TaskStorage (implement StateResetter interface)
	//  and pass it here in https://github.com/NilFoundation/nil/pull/419
	stateResetter := reset.NewStateResetter(logger, blockStorage)

	agg := NewAggregator(
		client,
		blockStorage,
		taskStorage,
		stateResetter,
		clock,
		logger,
		metricsHandler,
		cfg.AggregatorConfig,
	)

	ctx := context.Background()

	prop, err := NewProposer(
		ctx,
		cfg.ProposerParams,
		blockStorage,
		ethClient,
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
		prop, agg, taskScheduler, taskListener,
	)

	return syncCommittee, nil
}
