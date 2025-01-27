package core

import (
	"context"
	"fmt"

	nilrpc "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
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

	timer := common.NewTimer()
	blockStorage := storage.NewBlockStorage(database, timer, metricsHandler, logger)
	taskStorage := storage.NewTaskStorage(database, timer, metricsHandler, logger)

	aggregator := NewAggregator(
		client,
		blockStorage,
		taskStorage,
		timer,
		logger,
		metricsHandler,
		cfg.PollingDelay,
	)

	proposer, err := NewProposer(
		context.Background(),
		cfg.ProposerParams,
		blockStorage,
		ethClient,
		metricsHandler,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposer: %w", err)
	}

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(blockStorage, logger),
		metricsHandler,
		logger,
	)

	taskListener := rpc.NewTaskListener(
		&rpc.TaskListenerConfig{HttpEndpoint: cfg.TaskListenerRpcEndpoint},
		taskScheduler,
		logger,
	)

	s := &SyncCommittee{
		Service: srv.NewService(
			cfg.Config,
			logger,
			aggregator, proposer, taskListener, taskScheduler,
		),
	}

	return s, nil
}
