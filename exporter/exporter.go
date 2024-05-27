package exporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog/log"
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

	for {
		err := cfg.ExporterDriver.SetupScheme(ctx)
		if err != nil {
			cfg.ErrorChan <- fmt.Errorf("failed to setup scheme: %w", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	var shards []types.ShardId
	for {
		fetchedShards, err := cfg.FetchShards(ctx)
		if err != nil {
			cfg.ErrorChan <- fmt.Errorf("failed to fetch shards: %w", err)
			time.Sleep(3 * time.Second)
			continue
		}
		shards = fetchedShards
		break
	}

	workers := make([]concurrent.Func, 0)
	workers = append(workers, func(ctx context.Context) error {
		startTopFetcher(ctx, cfg, types.MasterShardId)
		return nil
	})
	workers = append(workers, func(ctx context.Context) error {
		startBottomFetcher(ctx, cfg, types.MasterShardId)
		return nil
	})

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
	curExportRound := cfg.exportRound.Load()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newExportRound := cfg.exportRound.Load()
			// because we could start reexport already exported block to database so we await to be pushed to database
			if curExportRound == newExportRound {
				continue
			}
			curExportRound = newExportRound
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
			if lastProcessedBlock != nil && isSetLastProcessed && topBlock.Id == lastProcessedBlock.Id {
				continue
			}

			var firstPoint types.BlockNumber = 0
			if lastProcessedBlock != nil && isSetLastProcessed {
				firstPoint = lastProcessedBlock.Id + 1
			}

			curBlock := topBlock

			for curBlock != nil && curBlock.Id >= firstPoint {
				blockMsg := wrapBlockWithShard(shardId, curBlock)
				cfg.BlocksChan <- blockMsg
				if len(curBlock.PrevBlock.Bytes()) == 0 {
					break
				}
				curBlock, err = cfg.FetchBlockByHash(ctx, shardId, curBlock.PrevBlock)
				if err != nil {
					cfg.ErrorChan <- fmt.Errorf("top fetcher for shard %s: failed to fetch block: %w", shardId.String(), err)
					time.Sleep(1 * time.Second)
					break
				}
			}
		}
	}
}

func startBottomFetcher(ctx context.Context, cfg *Cfg, shardId types.ShardId) {
	log.Info().Msgf("Starting bottom fetcher for shard %s...", shardId)
	ticker := time.NewTicker(1 * time.Second)
	curExportRound := cfg.exportRound.Load() + 1
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newExportRound := cfg.exportRound.Load()
			if curExportRound == newExportRound {
				continue
			}
			curExportRound = newExportRound
			absentBlockNumber, isSet, err := cfg.ExporterDriver.FetchEarliestAbsentBlock(ctx, shardId)
			if err != nil {
				cfg.ErrorChan <- fmt.Errorf("bottom fetcher for shard %s: failed to fetch absent block: %w", shardId.String(), err)
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

			log.Info().Msgf("Fetching from bottom block %d with shard id %s", startBlockId, shardId.String())

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
				blockMsg := wrapBlockWithShard(shardId, curBlock)
				cfg.BlocksChan <- blockMsg
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
		// if buffer size is more than 1000 stop read from channel to block fetchers
		if len(blockBuffer) > 10000 {
			if !fullMode {
				log.Info().Msgf("Buffer is full. Stop reading from channel. Buffer size: %d", len(blockBuffer))
				fullMode = true
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := cfg.ExporterDriver.ExportBlocks(ctx, blockBuffer); err != nil {
					cfg.ErrorChan <- fmt.Errorf("exporter: failed to export blocks: %w", err)
					continue
				}
				blockBuffer = blockBuffer[:0]
				cfg.incrementRound()
			}
		} else {
			if fullMode {
				log.Info().Msg("Buffer is not full. Start reading from channel")
				fullMode = false
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if len(blockBuffer) == 0 {
					continue
				}
				if err := cfg.ExporterDriver.ExportBlocks(ctx, blockBuffer); err != nil {
					cfg.ErrorChan <- fmt.Errorf("failed to export blocks: %w", err)
					continue
				}
				blockBuffer = blockBuffer[:0]
				cfg.incrementRound()
			case block := <-cfg.BlocksChan:
				blockBuffer = append(blockBuffer, block)
			}
		}
	}
}
