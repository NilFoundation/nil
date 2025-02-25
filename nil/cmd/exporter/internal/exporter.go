package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog/log"
)

const (
	BlockBufferSize     = 10000
	InitialRoundsAmount = 1000
)

func StartExporter(ctx context.Context, cfg *Cfg) error {
	logger.Info().Msg("Starting exporter...")

	shards, err := setupExporter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to setup exporter: %w", err)
	}

	workers := make([]concurrent.Func, 0, 2*len(shards)+1)
	for _, shard := range shards {
		workers = append(workers, func(ctx context.Context) error {
			startTopFetcher(ctx, cfg, shard)
			return nil
		})
		workers = append(workers, func(ctx context.Context) error {
			startBottomFetcher(ctx, cfg, shard)
			return nil
		})
	}
	workers = append(workers, func(ctx context.Context) error {
		startDriverExport(ctx, cfg)
		return nil
	})

	return concurrent.Run(ctx, workers...)
}

func setupExporter(ctx context.Context, cfg *Cfg) ([]types.ShardId, error) {
	if err := cfg.ExporterDriver.SetupScheme(ctx); err != nil {
		return nil, err
	}

	shards, err := cfg.FetchShards(ctx)
	if err != nil {
		return nil, err
	}

	return append(shards, types.MainShardId), nil
}

func startTopFetcher(ctx context.Context, cfg *Cfg, shardId types.ShardId) {
	logger := logger.With().Stringer(logging.FieldShardId, shardId).Logger()
	logger.Info().Msg("Starting top fetcher...")

	ticker := time.NewTicker(1 * time.Second)
	curExportRound := cfg.exportRound.Load() + InitialRoundsAmount
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newExportRound := cfg.exportRound.Load()
			if curExportRound == newExportRound {
				continue
			}
			lastProcessedBlock, isSetLastProcessed, err := cfg.ExporterDriver.FetchLatestProcessedBlock(ctx, shardId)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch last processed block.")
				continue
			}

			topBlock, err := cfg.FetchBlock(ctx, shardId, "latest")
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch last block.")
				continue
			}

			// totally synced on top level
			if lastProcessedBlock != nil && isSetLastProcessed && topBlock.Id == lastProcessedBlock.Id {
				continue
			}

			var firstPoint types.BlockNumber = 0
			if lastProcessedBlock != nil && isSetLastProcessed {
				firstPoint = lastProcessedBlock.Id + 1
			}

			curBlock := topBlock

			for curBlock != nil && curBlock.Id >= firstPoint {
				cfg.BlocksChan <- &BlockWithShardId{curBlock, shardId}
				curExportRound = newExportRound
				if len(curBlock.PrevBlock.Bytes()) == 0 {
					break
				}
				curBlock, err = cfg.FetchBlock(ctx, shardId, curBlock.PrevBlock)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to fetch block")
					break
				}
			}
		}
	}
}

func startBottomFetcher(ctx context.Context, cfg *Cfg, shardId types.ShardId) {
	logger := logger.With().Stringer(logging.FieldShardId, shardId).Logger()
	logger.Info().Msg("Starting bottom fetcher...")

	ticker := time.NewTicker(1 * time.Second)
	curExportRound := cfg.exportRound.Load() + InitialRoundsAmount
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newExportRound := cfg.exportRound.Load()
			if curExportRound == newExportRound {
				continue
			}
			absentBlockNumber, isSet, err := cfg.ExporterDriver.FetchEarliestAbsentBlock(ctx, shardId)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch absent block.")
				continue
			}
			if !isSet {
				logger.Info().Msg("Empty database. No blocks to fetch from bottom")
				return
			}
			zeroBlock, isSet, err := cfg.ExporterDriver.FetchBlock(ctx, shardId, types.BlockNumber(0))
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch zero block.")
				continue
			}

			startBlockId := absentBlockNumber
			if zeroBlock == nil || !isSet {
				startBlockId = 0
			}

			log.Info().
				Stringer(logging.FieldBlockNumber, startBlockId).
				Msg("Fetching from bottom block...")

			nextPresentId, isSet, err := cfg.ExporterDriver.FetchNextPresentBlock(ctx, shardId, startBlockId)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch next present block.")
				continue
			}
			if !isSet {
				logger.Info().Msg("No more blocks to fetch from bottom")
				return
			}

			logger.Info().Msgf("Fetching shard %s blocks from %d to %d", shardId.String(), startBlockId, nextPresentId)
			const batchSize = 10
			for curBlockId := startBlockId; curBlockId < nextPresentId; curBlockId += batchSize {
				batchEndId := curBlockId + batchSize
				if batchEndId > nextPresentId {
					batchEndId = nextPresentId
				}
				blocks, err := cfg.FetchBlocks(ctx, shardId, curBlockId, batchEndId)
				if err != nil {
					logger.Error().Err(err).Msg("Failed to fetch blocks")
					continue
				}
				for _, b := range blocks {
					cfg.BlocksChan <- &BlockWithShardId{b, shardId}
				}
				curExportRound = newExportRound
			}
		}
	}
}

func startDriverExport(ctx context.Context, cfg *Cfg) {
	logger.Info().Msg("Starting driver export...")

	ticker := time.NewTicker(1 * time.Second)
	var blockBuffer []*BlockWithShardId
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// read available blocks
			for len(cfg.BlocksChan) > 0 && len(blockBuffer) < BlockBufferSize {
				blockBuffer = append(blockBuffer, <-cfg.BlocksChan)
			}
			if len(cfg.BlocksChan) > 0 {
				logger.Warn().Msg("Block buffer is full!")
			}

			if len(blockBuffer) == 0 {
				continue
			}

			if err := cfg.ExporterDriver.ExportBlocks(ctx, blockBuffer); err != nil {
				logger.Error().Err(err).Msg("Failed to export blocks; will retry in the next round.")
				continue
			}
			blockBuffer = blockBuffer[:0]
			cfg.incrementRound()
		}
	}
}
