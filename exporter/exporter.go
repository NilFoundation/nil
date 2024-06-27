package exporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog/log"
)

const (
	BlockBufferSize     = 10000
	InitialRoundsAmount = 1000
)

type ExportMessage struct {
	BlockNumber transport.BlockNumber
}

func StartExporter(ctx context.Context, cfg *Cfg) error {
	if cfg.used {
		return errors.New("exporter already started")
	}
	cfg.used = true
	defer func() { cfg.used = false }()

	for err := cfg.ExporterDriver.SetupScheme(ctx); err != nil; {
		cfg.ErrorChan <- fmt.Errorf("failed to setup scheme: %w", err)
		time.Sleep(3 * time.Second)
	}

	var err error
	var shards []types.ShardId
	for shards, err = cfg.FetchShards(ctx); err != nil; {
		cfg.ErrorChan <- fmt.Errorf("failed to fetch shards: %w", err)
		time.Sleep(3 * time.Second)
	}

	workers := make([]concurrent.Func, 0)
	shards = append(shards, types.MasterShardId)
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

func startTopFetcher(ctx context.Context, cfg *Cfg, shardId types.ShardId) {
	log.Info().Msgf("Starting top fetcher for shard %s...", shardId.String())
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
				cfg.ErrorChan <- fmt.Errorf("top fetcher for shard %s: failed to fetch last processed block: %w", shardId.String(), err)
				continue
			}

			topBlock, err := cfg.FetchLastBlock(ctx, shardId)
			if err != nil {
				cfg.ErrorChan <- fmt.Errorf("top fetcher for shard %s: failed to fetch last block: %w", shardId, err)
				continue
			}

			// totally synced on top level
			if lastProcessedBlock != nil && isSetLastProcessed && topBlock.Block.Id == lastProcessedBlock.Id {
				continue
			}

			var firstPoint types.BlockNumber = 0
			if lastProcessedBlock != nil && isSetLastProcessed {
				firstPoint = lastProcessedBlock.Id + 1
			}

			curBlock := topBlock

			for curBlock != nil && curBlock.Block.Id >= firstPoint {
				cfg.BlocksChan <- curBlock
				curExportRound = newExportRound
				if len(curBlock.Block.PrevBlock.Bytes()) == 0 {
					break
				}
				curBlock, err = cfg.FetchBlockByHash(ctx, shardId, curBlock.Block.PrevBlock)
				if err != nil {
					cfg.ErrorChan <- fmt.Errorf("top fetcher for shard %s: failed to fetch block: %w", shardId.String(), err)
					break
				}
			}
		}
	}
}

func startBottomFetcher(ctx context.Context, cfg *Cfg, shardId types.ShardId) {
	log.Info().
		Stringer(logging.FieldShardId, shardId).
		Msg("Starting bottom fetcher...")

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
				cfg.ErrorChan <- fmt.Errorf("bottom fetcher for shard %s: failed to fetch absent block: %w", shardId, err)
				continue
			}
			if !isSet {
				log.Info().Msg("Empty database. No blocks to fetch from bottom")
				return
			}
			zeroBlock, isSet, err := cfg.ExporterDriver.FetchBlock(ctx, shardId, types.BlockNumber(0))
			if err != nil {
				cfg.ErrorChan <- fmt.Errorf("bottom fetcher for shard %s: failed to fetch zero block: %w", shardId.String(), err)
				continue
			}

			startBlockId := absentBlockNumber
			if zeroBlock == nil || !isSet {
				startBlockId = 0
			}

			log.Info().
				Stringer(logging.FieldShardId, shardId).
				Stringer(logging.FieldBlockNumber, startBlockId).
				Msg("Fetching from bottom block...")

			nextPresentId, isSet, err := cfg.ExporterDriver.FetchNextPresentBlock(ctx, shardId, startBlockId)
			if err != nil {
				cfg.ErrorChan <- fmt.Errorf("bottom fetcher for shard %s: failed to fetch next present block: %w", shardId.String(), err)
				continue
			}
			if !isSet {
				log.Info().Msg("No more blocks to fetch from bottom")
				return
			}

			log.Info().Msgf("Fetching shard %s blocks from %d to %d", shardId.String(), startBlockId, nextPresentId)
			for curBlockId := startBlockId; curBlockId < nextPresentId; curBlockId++ {
				curBlock, err := cfg.FetchBlockByNumber(ctx, shardId, transport.BlockNumber(curBlockId))
				if err != nil {
					cfg.ErrorChan <- fmt.Errorf("bottom fetcher for shard %s: failed to fetch block: %w", shardId.String(), err)
					continue
				}
				cfg.BlocksChan <- curBlock
				curExportRound = newExportRound
			}
		}
	}
}

func startDriverExport(ctx context.Context, cfg *Cfg) {
	log.Info().Msg("Starting driver export...")
	ticker := time.NewTicker(1 * time.Second)
	fullMode := false
	var blockBuffer []*BlockMsg
	for {
		if len(blockBuffer) > BlockBufferSize {
			if !fullMode {
				log.Info().Msgf("Buffer is full. Stop reading from channel. Buffer size: %d", len(blockBuffer))
				fullMode = true
			}
		} else {
			if fullMode {
				log.Info().Msg("Buffer is not full. Start reading from channel")
				fullMode = false
			}
			blockBuffer = append(blockBuffer, <-cfg.BlocksChan)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(blockBuffer) == 0 {
				continue
			}
			if err := cfg.ExporterDriver.ExportBlocks(ctx, blockBuffer); err != nil {
				cfg.ErrorChan <- fmt.Errorf("exporter: failed to export blocks: %w", err)
				continue
			}
			blockBuffer = blockBuffer[:0]
			cfg.incrementRound()
		}
	}
}
