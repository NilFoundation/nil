package exporter

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog/log"
)

type ExportMessage struct {
	BlockNumber transport.BlockNumber
}

func StartExporter(ctx context.Context, cfg Cfg) error {
	if cfg.used {
		return errors.New("exporter already started")
	}
	cfg.used = true
	defer func() { cfg.used = false }()

	err := cfg.ExporterDriver.SetupScheme(ctx)
	if err != nil {
		return err
	}

	if err = concurrent.Run(
		ctx,
		func(ctx context.Context) error {
			startTopFetcher(ctx, &cfg)
			return nil
		},
	); err != nil {
		return err
	}
	return nil
}

func startTopFetcher(ctx context.Context, cfg *Cfg) {
	log.Info().Msg("Starting top fetcher...")
	for {
		select {
		case <-ctx.Done():
			return
		default:
			log.Info().Msg("Fetching blocks...")
			lastProcessedBlock, err := cfg.ExporterDriver.FetchLatestBlock(ctx)
			if err != nil {
				cfg.ErrorChan <- err
				log.Err(err).Msg("Failed to fetch last processed block")
				time.Sleep(1 * time.Second)
				continue
			}

			topBlock, err := cfg.FetchLastBlock(ctx, types.MasterShardId)
			if err != nil {
				cfg.ErrorChan <- err
				log.Err(err).Msg("Failed to fetch last block from blockchain")
				time.Sleep(1 * time.Second)
				continue
			}

			if lastProcessedBlock != nil && topBlock.Id == lastProcessedBlock.Id {
				time.Sleep(1 * time.Second)
				continue
			}

			var firstPoint types.BlockNumber = 0
			if lastProcessedBlock != nil {
				firstPoint = lastProcessedBlock.Id + 1
			}

			curBlock := topBlock

			for curBlock != nil && curBlock.Id >= firstPoint {
				if err := cfg.ExporterDriver.ExportBlocks(ctx, []*types.Block{curBlock}); err != nil {
					cfg.ErrorChan <- err
					log.Err(err).Msg("Failed to export block")
					time.Sleep(1 * time.Second)
					continue
				}
				curBlock, err = cfg.FetchBlockByHash(ctx, types.MasterShardId, curBlock.PrevBlock)
				if err != nil {
					cfg.ErrorChan <- err
					log.Err(err).Msg("Failed to fetch block")
					time.Sleep(1 * time.Second)
					continue
				}
			}
		}
	}
}
