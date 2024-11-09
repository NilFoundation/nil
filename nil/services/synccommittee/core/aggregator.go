package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type Aggregator struct {
	logger       zerolog.Logger
	client       *rpc.Client
	blockStorage storage.BlockStorage
	taskStorage  storage.TaskStorage
	metrics      *MetricsHandler
	pollingDelay time.Duration
}

func NewAggregator(
	client *rpc.Client,
	blockStorage storage.BlockStorage,
	taskStorage storage.TaskStorage,
	logger zerolog.Logger,
	metrics *MetricsHandler,
	pollingDelay time.Duration,
) (*Aggregator, error) {
	return &Aggregator{
		logger:       logger,
		client:       client,
		blockStorage: blockStorage,
		taskStorage:  taskStorage,
		metrics:      metrics,
		pollingDelay: pollingDelay,
	}, nil
}

func (agg *Aggregator) Run(ctx context.Context) error {
	agg.logger.Info().Msg("starting blocks fetching")

	concurrent.RunTickerLoop(ctx, agg.pollingDelay,
		func(ctx context.Context) {
			if err := agg.processNewBlocks(ctx); err != nil {
				agg.logger.Error().Err(err).Msg("error during processing new blocks")
				return
			}
		},
	)

	agg.logger.Info().Msg("blocks fetching stopped")
	return nil
}

// get block by hash
func (agg *Aggregator) getBlock(blockId types.BlockId) (*jsonrpc.RPCBlock, error) {
	block, err := agg.client.GetBlock(blockId.ShardId, blockId.Hash, true)
	if err != nil && block != nil {
		return nil, err
	}
	return block, nil
}

// fetchLatestBlocks retrieves the latest block for main shard
func (agg *Aggregator) fetchLatestBlockRef() (*types.MainBlockRef, error) {
	block, err := agg.client.GetBlock(coreTypes.MainShardId, "latest", false)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest block from shard %d: %w", coreTypes.MainShardId, err)
	}
	return types.NewBlockRef(block)
}

// createProofTask generates proof tasks for a block
func (agg *Aggregator) createProofTask(ctx context.Context, blockForProof *jsonrpc.RPCBlock, mainBlockHash common.Hash) error {
	batchId, err := agg.blockStorage.GetOrCreateBatchId(ctx, mainBlockHash)
	if err != nil {
		return err
	}

	taskEntries := make([]*types.TaskEntry, 1)

	aggregateProofsTask, err := agg.taskStorage.TryGetTaskEntryByHash(ctx, mainBlockHash)
	if err != nil {
		return err
	}
	if aggregateProofsTask == nil {
		aggregateProofsTask = types.NewAggregateBlockProofsTaskEntry(batchId, coreTypes.MainShardId, 0, mainBlockHash, 0)
		aggregateProofsTask.Status = types.Draft
		agg.logger.Debug().
			Stringer(logging.FieldBatchId, batchId).
			Int64("blkNum", int64(blockForProof.Number)).
			Stringer(logging.FieldShardId, blockForProof.ShardId).
			Msgf("create aggregate proof task %s, blkHash = %s", aggregateProofsTask.Task.Id, mainBlockHash)
	}
	taskEntries[0] = aggregateProofsTask
	if blockForProof.ShardId == coreTypes.MainShardId {
		aggregateProofsTask.Task.DependencyNum = uint8(len(blockForProof.ChildBlocks))
		aggregateProofsTask.Task.BlockNum = blockForProof.Number
		aggregateProofsTask.Status = types.WaitingForInput
		agg.logger.Debug().
			Stringer(logging.FieldBatchId, batchId).
			Int64("blkNum", int64(blockForProof.Number)).
			Stringer(logging.FieldShardId, blockForProof.ShardId).
			Msgf("complete aggregate proof task %s, blkHash = %s, dependencies number = %d", aggregateProofsTask.Task.Id, mainBlockHash, aggregateProofsTask.Task.DependencyNum)
	} else {
		proofProviderTask := types.NewBlockProofTaskEntry(batchId, &aggregateProofsTask.Task.Id, blockForProof.Hash)
		proofProviderTask.PendingDeps = append(proofProviderTask.PendingDeps, aggregateProofsTask.Task.Id)
		taskEntries = append(taskEntries, proofProviderTask)
		agg.logger.Debug().
			Stringer(logging.FieldBatchId, batchId).
			Int64("blkNum", int64(blockForProof.Number)).
			Stringer(logging.FieldShardId, blockForProof.ShardId).
			Msgf("create block proof task %s, blkHash = %s, mainBlockHash = %s", proofProviderTask.Task.Id, blockForProof.Hash, mainBlockHash)
	}

	if err := agg.taskStorage.AddTaskEntries(ctx, taskEntries); err != nil {
		return err
	}

	agg.metrics.RecordBlocksInTasks(ctx, 1)

	return nil
}

// validateAndProcessBlock checks the validity of a block and stores it if valid.
func (agg *Aggregator) validateAndProcessBlock(ctx context.Context, block *jsonrpc.RPCBlock, mainBlockHash common.Hash) error {
	if block == nil {
		return errors.New("block cannot be nil")
	}

	if block.ShardId == coreTypes.MainShardId {
		if mainBlockHash != block.Hash {
			return fmt.Errorf("wrong mainBlockHash %s, expected %s", mainBlockHash, block.Hash)
		}

		latestFetched, err := agg.blockStorage.TryGetLatestFetched(ctx)
		if err != nil {
			return fmt.Errorf("error reading latest fetched block for shard %d: %w", block.ShardId, err)
		}
		if err := latestFetched.ValidateChild(block); err != nil {
			return err
		}

		isBatchCompleted, err := agg.blockStorage.IsBatchCompleted(ctx, block)
		if err != nil {
			return err
		}
		if !isBatchCompleted {
			return fmt.Errorf("batch for the main shard block %s is not completed", block.Hash)
		}
	}

	if err := agg.createProofTask(ctx, block, mainBlockHash); err != nil {
		return fmt.Errorf("error creating proof tasks for block %s: %w", block.Hash, err)
	}

	if err := agg.blockStorage.SetBlock(ctx, block, mainBlockHash); err != nil {
		return fmt.Errorf("error storing block %s: %w", block.Hash, err)
	}

	return nil
}

// fetchAndProcessBlocks retrieves a range of blocks for a main shard, stores them, creates proof tasks
func (agg *Aggregator) fetchAndProcessBlocks(ctx context.Context, blocksRange types.BlocksRange) error {
	shardId := coreTypes.MainShardId
	const batchSize = 20
	results, err := agg.client.GetBlocksRange(shardId, blocksRange.Start, blocksRange.End+1, true, batchSize)
	if err != nil {
		return fmt.Errorf("error fetching blocks from shard %d: %w", shardId, err)
	}

	fetchedBlocksLen := len(results)
	agg.logger.Debug().Int64("blkCount", int64(fetchedBlocksLen)).Stringer(logging.FieldShardId, shardId).Msg("fetched blocks range")
	agg.metrics.RecordBlocksFetched(ctx, int64(fetchedBlocksLen))

	for _, block := range results {
		err := agg.fetchChildBlocks(ctx, block)
		if err != nil {
			return err
		}
		if err := agg.validateAndProcessBlock(ctx, block, block.Hash); err != nil {
			return fmt.Errorf("error validating and storing main shard block %s: %w", block.Hash, err)
		}
	}

	return nil
}

func (agg *Aggregator) fetchChildBlocks(ctx context.Context, block *jsonrpc.RPCBlock) error {
	childIds, err := types.ChildBlockIds(block)
	if err != nil {
		return err
	}

	for _, childId := range childIds {
		childBlock, err := agg.getBlock(childId)
		if err != nil {
			return fmt.Errorf("error fetching child block with id=%s: %w", childId, err)
		}
		if err = agg.validateAndProcessBlock(ctx, childBlock, block.Hash); err != nil {
			return fmt.Errorf(
				"error validating and storing child block with id=%s for main shard block with hash=%s: %w",
				childId, block.Hash, err,
			)
		}
	}

	return nil
}

// processNewBlocks fetches and processes new blocks for all shards.
// It handles the overall flow of block synchronization and proof creation.
func (agg *Aggregator) processNewBlocks(ctx context.Context) error {
	agg.metrics.StartProcessingMeasurement()
	defer agg.metrics.EndProcessingMeasurement(ctx)

	latestBlock, err := agg.fetchLatestBlockRef()
	if err != nil {
		agg.metrics.RecordError(ctx)
		return err
	}

	if err := agg.processShardBlocks(ctx, *latestBlock); err != nil {
		// todo: launch block re-fetching in case of ErrBlockMismatch
		agg.metrics.RecordError(ctx)
		return fmt.Errorf("error processing blocks from shard %d: %w", coreTypes.MainShardId, err)
	}

	return nil
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

	agg.metrics.SetCurrentBlockHeight(ctx, int64(actualLatest.Number), coreTypes.MainShardId)
	return nil
}
