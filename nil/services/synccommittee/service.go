package synccommittee

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/listener"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover"
	"github.com/rs/zerolog"
)

type SyncCommittee struct {
	cfg          *Config
	database     db.DB
	logger       zerolog.Logger
	client       *rpc.Client
	proposer     *Proposer
	aggregator   *Aggregator
	taskListener *listener.TaskListener
	provers      *[]prover.Prover // At this point provers are embedded into sync committee
}

func New(cfg *Config, database db.DB) (*SyncCommittee, error) {
	logger := logging.NewLogger("sync_committee")
	metrics, err := NewMetricsHandler("github.com/NilFoundation/nil/nil/services/sync_committee")
	if err != nil {
		return nil, err
	}

	client := rpc.NewClient(cfg.RpcEndpoint, logger)

	proposer, err := NewProposer(cfg.L1Endpoint, cfg.L1ChainId, cfg.PrivateKey, cfg.L1ContractAddress, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create proposer: %w", err)
	}

	aggregator, err := NewAggregator(client, proposer, database, logger, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator: %w", err)
	}

	taskListener := listener.NewTaskListener(
		&listener.TaskListenerConfig{HttpEndpoint: cfg.OwnRpcEndpoint},
		listener.NewTaskRequestRpcServer(),
		logger,
	)

	provers, err := initializeProvers(cfg, logger)
	if err != nil {
		return nil, err
	}

	s := &SyncCommittee{
		cfg:          cfg,
		database:     database,
		logger:       logger,
		client:       client,
		proposer:     proposer,
		aggregator:   aggregator,
		taskListener: taskListener,
		provers:      provers,
	}

	return s, nil
}

func initializeProvers(cfg *Config, logger zerolog.Logger) (*[]prover.Prover, error) {
	proverConfig := prover.DefaultConfig()

	provers := make([]prover.Prover, cfg.ProversCount)

	for i := range cfg.ProversCount {
		localProver, err := prover.NewProver(
			proverConfig,
			prover.NewTaskRequestRpcClient(cfg.OwnRpcEndpoint, logger),
			prover.NewTaskHandler(logger),
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create local prover: %w", err)
		}
		provers[i] = *localProver
	}

	return &provers, nil
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
		s.processingLoop,
		s.taskListener.Run,
	}

	for _, proverWorker := range *s.provers {
		functions = append(functions, proverWorker.Run)
	}

	if err := concurrent.Run(ctx, functions...); err != nil {
		s.logger.Error().Err(err).Msg("app encountered an error and will be terminated")
	}

	return nil
}

func (s *SyncCommittee) processingLoop(ctx context.Context) error {
	s.logger.Info().Msg("starting sync committee service processing loop")

	concurrent.RunTickerLoop(ctx, s.cfg.PollingDelay,
		func(ctx context.Context) {
			if err := s.aggregator.ProcessNewBlocks(ctx); err != nil {
				s.logger.Error().Err(err).Msg("error during processing new blocks")
				return
			}
		},
	)

	s.logger.Info().Msg("sync committee service processing loop stopped")
	return nil
}
