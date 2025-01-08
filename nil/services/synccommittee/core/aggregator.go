package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type AggregatorMetrics interface {
	metrics.BasicMetrics
	RecordMainBlockFetched(ctx context.Context)
	RecordBlockBatchSize(ctx context.Context, batchSize int64)
}

type Aggregator struct {
	logger       zerolog.Logger
	rpcClient    client.Client
	blockStorage storage.BlockStorage
	taskStorage  storage.TaskStorage
	timer        common.Timer
	metrics      AggregatorMetrics
	pollingDelay time.Duration
}

func NewAggregator(
	rpcClient client.Client,
	blockStorage storage.BlockStorage,
	taskStorage storage.TaskStorage,
	timer common.Timer,
	logger zerolog.Logger,
	metrics AggregatorMetrics,
	pollingDelay time.Duration,
) (*Aggregator, error) {
	return &Aggregator{
		logger:       logger,
		rpcClient:    rpcClient,
		blockStorage: blockStorage,
		taskStorage:  taskStorage,
		timer:        timer,
		metrics:      metrics,
		pollingDelay: pollingDelay,
	}, nil
}

func (agg *Aggregator) Run(ctx context.Context) error {
	agg.logger.Info().Msg("starting blocks fetching")

	concurrent.RunTickerLoop(ctx, agg.pollingDelay,
		func(ctx context.Context) {
			err := agg.processNewBlocks(ctx)

			if errors.Is(err, types.ErrBatchNotReady) {
				agg.logger.Warn().Err(err).Msg("received unready block batch, skipping")
				return
			}

			if err != nil {
				agg.logger.Error().Err(err).Msg("error during processing new blocks")
				agg.metrics.RecordError(ctx, "aggregator")
			}
		},
	)

	agg.logger.Info().Msg("blocks fetching stopped")
	return nil
}

// processNewBlocks fetches and processes new blocks for all shards.
// It handles the overall flow of block synchronization and proof creation.
func (agg *Aggregator) processNewBlocks(ctx context.Context) error {
	latestBlock, err := agg.fetchLatestBlockRef(ctx)
	if err != nil {
		return err
	}

	if err := agg.processShardBlocks(ctx, *latestBlock); err != nil {
		// todo: launch block re-fetching in case of ErrBlockMismatch
		return fmt.Errorf("error processing blocks: %w", err)
	}

	return nil
}

// fetchLatestBlocks retrieves the latest block for main shard
func (agg *Aggregator) fetchLatestBlockRef(ctx context.Context) (*types.MainBlockRef, error) {
	block, err := agg.rpcClient.GetBlock(ctx, coreTypes.MainShardId, "latest", false)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest block from shard %d: %w", coreTypes.MainShardId, err)
	}
	return types.NewBlockRef(block)
}

// processShardBlocks handles the processing of new blocks for the main shard.
// It fetches new blocks, updates the storage, and records relevant metrics.
func (agg *Aggregator) processShardBlocks(ctx context.Context, actualLatest types.MainBlockRef) error {
	latestFetched, err := agg.blockStorage.TryGetLatestFetched(ctx)
	if err != nil {
		return fmt.Errorf("error reading latest fetched block for the main shard: %w", err)
	}

	fetchingRange, err := types.GetBlocksFetchingRange(latestFetched, actualLatest)
	if err != nil {
		return err
	}

	if fetchingRange == nil {
		agg.logger.Debug().
			Stringer(logging.FieldShardId, coreTypes.MainShardId).
			Stringer(logging.FieldBlockNumber, actualLatest.Number).
			Msg("no new blocks to fetch")
	} else {
		if err := agg.fetchAndProcessBlocks(ctx, *fetchingRange); err != nil {
			return fmt.Errorf("%w: %w", types.ErrBlockProcessing, err)
		}
	}

	return nil
}

// fetchAndProcessBlocks retrieves a range of blocks for a main shard, stores them, creates proof tasks
func (agg *Aggregator) fetchAndProcessBlocks(ctx context.Context, blocksRange types.BlocksRange) error {
	shardId := coreTypes.MainShardId
	const requestBatchSize = 20
	results, err := agg.rpcClient.GetBlocksRange(ctx, shardId, blocksRange.Start, blocksRange.End+1, true, requestBatchSize)
	if err != nil {
		return fmt.Errorf("error fetching blocks from shard %d: %w", shardId, err)
	}

	for _, mainShardBlock := range results {
		blockBatch, err := agg.createBlockBatch(ctx, mainShardBlock)
		if err != nil {
			return fmt.Errorf("error creating batch, mainHash=%s: %w", mainShardBlock.Hash, err)
		}

		if err := agg.handleBlockBatch(ctx, blockBatch); err != nil {
			return fmt.Errorf("error handing batch, mainHash=%s: %w", mainShardBlock.Hash, err)
		}
	}

	fetchedLen := int64(len(results))
	agg.logger.Debug().Int64("blkCount", fetchedLen).Stringer(logging.FieldShardId, shardId).Msg("fetched main shard blocks")
	agg.metrics.RecordBlockBatchSize(ctx, fetchedLen)
	return nil
}

func (agg *Aggregator) createBlockBatch(ctx context.Context, mainShardBlock *jsonrpc.RPCBlock) (*types.BlockBatch, error) {
	childIds, err := types.ChildBlockIds(mainShardBlock)
	if err != nil {
		return nil, err
	}

	childBlocks := make([]*jsonrpc.RPCBlock, 0, len(childIds))

	for _, childId := range childIds {
		childBlock, err := agg.rpcClient.GetBlock(ctx, childId.ShardId, childId.Hash, true)
		if err != nil {
			return nil, fmt.Errorf(
				"error fetching child block with id=%s, mainHash=%s: %w", childId, mainShardBlock.Hash, err,
			)
		}
		childBlocks = append(childBlocks, childBlock)
	}

	return types.NewBlockBatch(mainShardBlock, childBlocks)
}

// handleBlockBatch checks the validity of a block and stores it if valid.
func (agg *Aggregator) handleBlockBatch(ctx context.Context, batch *types.BlockBatch) error {
	latestFetched, err := agg.blockStorage.TryGetLatestFetched(ctx)
	if err != nil {
		return fmt.Errorf("error reading latest fetched block from storage: %w", err)
	}
	if err := latestFetched.ValidateChild(batch.MainShardBlock); err != nil {
		return err
	}

	if err := agg.createProofTasks(ctx, batch); err != nil {
		return fmt.Errorf("error creating proof tasks, mainHash=%s: %w", batch.MainShardBlock.Hash, err)
	}

	if err := agg.blockStorage.SetBlockBatch(ctx, batch); err != nil {
		return fmt.Errorf("error storing block batch, mainHash=%s: %w", batch.MainShardBlock.Hash, err)
	}

	agg.metrics.RecordMainBlockFetched(ctx)
	return nil
}

// createProofTask generates proof tasks for block batch
func (agg *Aggregator) createProofTasks(ctx context.Context, batch *types.BlockBatch) error {
	currentTime := agg.timer.NowTime()
	proofTasks, err := batch.CreateProofTasks(currentTime)
	if err != nil {
		return fmt.Errorf("error creating proof tasks, mainHash=%s: %w", batch.MainShardBlock.Hash, err)
	}

	if err := agg.taskStorage.AddTaskEntries(ctx, proofTasks); err != nil {
		return fmt.Errorf("error adding task entries, mainHash=%s: %w", batch.MainShardBlock.Hash, err)
	}

	agg.logger.Debug().
		Stringer(logging.FieldBatchId, batch.Id).
		Msgf("created %d proof tasks, mainHash=%s", len(proofTasks), batch.MainShardBlock.Hash)

	return nil
}
