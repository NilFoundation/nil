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
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

type SyncCommittee struct {
	cfg      *Config
	database db.DB
	logger   zerolog.Logger
	client   *rpc.Client
	storage  *BlockStorage
	metrics  *MetricsHandler
	observer *BlockObserver
}

func New(cfg *Config, database db.DB) (*SyncCommittee, error) {
	logger := logging.NewLogger("sync_committee")

	metrics, err := NewMetricsHandler("github.com/NilFoundation/nil/nil/services/sync_committee")
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics handler: %w", err)
	}

	client := rpc.NewClient(cfg.RpcEndpoint, logger)
	storage := NewBlockStorage()

	s := &SyncCommittee{
		cfg:      cfg,
		database: database,
		logger:   logger,
		client:   client,
		storage:  storage,
		metrics:  metrics,
	}

	s.observer = NewBlockObserver(client, storage, metrics, logger)

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
			if err := s.observer.ProcessNewBlocks(ctx); err != nil {
				s.logger.Error().Err(err).Msg("error during processing new blocks")
				return
			}
			if s.proofThresholdMet() {
				s.createProofTasks(ctx)
			}
		},
	)
}

func (s *SyncCommittee) proofThresholdMet() bool {
	lastProvedBlkNum := s.storage.GetLastProvedBlockNum(types.MainShardId)
	lastFetchedBlock := s.storage.GetLastFetchedBlock(types.MainShardId)
	return lastProvedBlkNum != lastFetchedBlock.Number
}

func (s *SyncCommittee) createProofTasks(ctx context.Context) {
	lastProvedBlkNum := s.storage.GetLastProvedBlockNum(types.MainShardId)
	lastFetchedBlock := s.storage.GetLastFetchedBlock(types.MainShardId)
	blocksForProof := s.storage.GetBlocksRange(types.MainShardId, lastProvedBlkNum+1, lastFetchedBlock.Number+1)

	// TODO: add actual creation logic here

	blocksInTask := int64(len(blocksForProof))
	s.logger.Info().Int64("blkCount", blocksInTask).Msg("proof tasks created")
	s.metrics.RecordBlocksInTasks(ctx, blocksInTask)

	s.updateLastProvedBlockNumForAllShards()

	s.storage.CleanupStorage()
}

func (s *SyncCommittee) updateLastProvedBlockNumForAllShards() {
	shardIdList, err := s.observer.getShardIdList()
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get shard list for updating last proved block numbers")
		return
	}

	for _, shardId := range shardIdList {
		lastFetchedBlock := s.storage.GetLastFetchedBlock(shardId)
		if lastFetchedBlock != nil {
			s.storage.SetLastProvedBlockNum(shardId, lastFetchedBlock.Number)
		} else {
			s.logger.Warn().
				Stringer(logging.FieldShardId, shardId).
				Msg("no last fetched block found for shard")
		}
	}
}
