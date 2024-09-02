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
	"github.com/rs/zerolog"
)

type SyncCommittee struct {
	cfg        *Config
	database   db.DB
	logger     zerolog.Logger
	client     *rpc.Client
	proposer   *Proposer
	aggregator *Aggregator
}

func New(cfg *Config, database db.DB) (*SyncCommittee, error) {
	logger := logging.NewLogger("sync_committee")

	client := rpc.NewClient(cfg.RpcEndpoint, logger)

	proposer := NewProposer("", logger)

	aggregator, err := NewAggregator(client, logger, proposer, database)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregator: %w", err)
	}

	s := &SyncCommittee{
		cfg:        cfg,
		database:   database,
		logger:     logger,
		client:     client,
		proposer:   proposer,
		aggregator: aggregator,
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

	s.logger.Info().Msg("starting sync committee service processing loop")
	s.processingLoop(ctx)
	s.logger.Info().Msg("sync committee service processing loop stopped")

	return nil
}

func (s *SyncCommittee) processingLoop(ctx context.Context) {
	concurrent.RunTickerLoop(ctx, s.cfg.PollingDelay,
		func(ctx context.Context) {
			if err := s.aggregator.ProcessNewBlocks(ctx); err != nil {
				s.logger.Error().Err(err).Msg("error during processing new blocks")
				return
			}
		},
	)
}
