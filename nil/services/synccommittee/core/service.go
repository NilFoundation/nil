package core

import (
	"context"
	"fmt"
	"math/big"
	"os/signal"
	"syscall"

	nilrpc "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
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

func New(cfg *Config, database db.DB) (*SyncCommittee, error) {
	logger := logging.NewLogger("sync_committee")
	metrics, err := NewMetricsHandler("github.com/NilFoundation/nil/nil/services/sync_committee")
	if err != nil {
		return nil, err
	}

	logger.Info().Msgf("Use RPC endpoint %v", cfg.RpcEndpoint)
	client := nilrpc.NewClient(cfg.RpcEndpoint, logger)

	blockStorage := storage.NewBlockStorage(database, logger)
	taskStorage := storage.NewTaskStorage(database, logger)

	aggregator, err := NewAggregator(client, blockStorage, taskStorage, logger, metrics, cfg.PollingDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator: %w", err)
	}

	chainId, ok := new(big.Int).SetString(cfg.L1ChainId, 10)
	if !ok {
		return nil, fmt.Errorf("invalid L1ChainId: %s", cfg.L1ChainId)
	}
	proposerParams := ProposerParams{
		cfg.L1Endpoint, chainId, cfg.PrivateKey, cfg.L1ContractAddress, cfg.SelfAddress, DefaultProposingInterval,
	}
	proposer, err := NewProposer(proposerParams, blockStorage, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposer: %w", err)
	}

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(blockStorage, logger),
		logger,
	)

	taskListener := rpc.NewTaskListener(
		&rpc.TaskListenerConfig{HttpEndpoint: cfg.OwnRpcEndpoint},
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

	if err := telemetry.Init(ctx, s.cfg.Telemetry); err != nil {
		s.logger.Error().Err(err).Msg("failed to initialize telemetry")
		return err
	}
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
