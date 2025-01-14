package core

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	nilrpc "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/rs/zerolog"
)

type SyncCommittee struct {
	cfg      *Config
	database db.DB
	logger   zerolog.Logger
	client   *nilrpc.Client

	proposer      *Proposer
	aggregator    *Aggregator
	taskListener  *rpc.TaskListener
	taskScheduler scheduler.TaskScheduler
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

	aggregator, err := NewAggregator(client, blockStorage, taskStorage, timer, logger, metricsHandler, cfg.PollingDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator: %w", err)
	}

	proposer, err := NewProposer(context.Background(), cfg.ProposerParams, blockStorage, ethClient, metricsHandler, logger)
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
		cfg:      cfg,
		database: database,
		logger:   logger,
		client:   client,

		proposer:      proposer,
		aggregator:    aggregator,
		taskListener:  taskListener,
		taskScheduler: taskScheduler,
	}

	return s, nil
}

func (s *SyncCommittee) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer telemetry.Shutdown(ctx)

	if s.cfg.GracefulShutdown {
		signalCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
		defer stop()
		ctx = signalCtx
	}

	functions := []concurrent.Func{
		s.proposer.Run,
		s.aggregator.Run,
		s.taskListener.Run,
		s.taskScheduler.Run,
	}

	if err := concurrent.Run(ctx, functions...); err != nil {
		s.logger.Error().Err(err).Msg("app encountered an error and will be terminated")
	}

	return nil
}
