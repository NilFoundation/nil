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
	"github.com/NilFoundation/nil/nil/internal/db"
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
func (agg *Aggregator) getBlock(shardId coreTypes.ShardId, blockHash common.Hash) (*jsonrpc.RPCBlock, error) {
	block, err := agg.client.GetBlock(shardId, blockHash, true)
	if err != nil && block != nil {
		return nil, err
	}
	return block, nil
}

// fetchLatestBlocks retrieves the latest block for main shard
func (agg *Aggregator) fetchLatestBlock() (*jsonrpc.RPCBlock, error) {
	block, err := agg.client.GetBlock(coreTypes.MainShardId, "latest", false)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest block from shard %d: %w", coreTypes.MainShardId, err)
	}
	return block, nil
}

// createProofTask generates proof tasks for the main shard blocks.
func (agg *Aggregator) createProofTask(ctx context.Context, blockForProof *jsonrpc.RPCBlock) error {
	mainHash := blockForProof.MainChainHash
	if mainHash == common.EmptyHash {
		mainHash = blockForProof.Hash
	}

	batchId, err := agg.blockStorage.GetBatchId(ctx, blockForProof)
	if err != nil {
		return err
	}

	taskEntries := make([]*types.TaskEntry, 1)

	aggregateProofsTask, err := agg.taskStorage.TryGetTaskEntryByHash(ctx, mainHash)
	if err != nil {
		return err
	}
	if aggregateProofsTask == nil {
		aggregateProofsTask = types.NewAggregateBlockProofsTaskEntry(batchId, coreTypes.MainShardId, 0, mainHash, 0)
		aggregateProofsTask.Status = types.Draft
		agg.logger.Debug().
			Stringer(logging.FieldBatchId, batchId).
			Int64("blkNum", int64(blockForProof.Number)).
			Stringer(logging.FieldShardId, blockForProof.ShardId).
			Msgf("create aggregate proof task %s, blkHash = %s", aggregateProofsTask.Task.Id, mainHash)
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
			Msgf("complete aggregate proof task %s, blkHash = %s, dependencies number = %d", aggregateProofsTask.Task.Id, mainHash, aggregateProofsTask.Task.DependencyNum)
	} else {
		proofProviderTask := types.NewBlockProofTaskEntry(batchId, &aggregateProofsTask.Task.Id, blockForProof.Hash)
		proofProviderTask.PendingDeps = append(proofProviderTask.PendingDeps, aggregateProofsTask.Task.Id)
		taskEntries = append(taskEntries, proofProviderTask)
		agg.logger.Debug().
			Stringer(logging.FieldBatchId, batchId).
			Int64("blkNum", int64(blockForProof.Number)).
			Stringer(logging.FieldShardId, blockForProof.ShardId).
			Msgf("create block proof task %s, blkHash = %s, mainHash = %s", proofProviderTask.Task.Id, blockForProof.Hash, mainHash)
	}

	if err := agg.taskStorage.AddTaskEntries(ctx, taskEntries); err != nil {
		return err
	}

	agg.metrics.RecordBlocksInTasks(ctx, 1)

	return nil
}

// validateAndProcessBlock checks the validity of a block and stores it if valid.
func (agg *Aggregator) validateAndProcessBlock(ctx context.Context, block *jsonrpc.RPCBlock) error {
	isBatchCompleted, err := agg.blockStorage.IsBatchCompleted(ctx, block)
	if err != nil {
		return err
	}
	if block.ShardId == coreTypes.MainShardId && !isBatchCompleted {
		return fmt.Errorf("batch for the main shard block %s is not completed", block.Hash)
	}

	if err := agg.blockStorage.SetBlock(ctx, block.ShardId, block.Number, block); err != nil {
		return err
	}

	// Start generating proof during blocks fetching
	return agg.createProofTask(ctx, block)
}

// fetchAndProcessBlocks retrieves a range of blocks for a main shard, stores them, creates proof tasks
func (agg *Aggregator) fetchAndProcessBlocks(ctx context.Context, from, to coreTypes.BlockNumber) error {
	shardId := coreTypes.MainShardId
	const batchSize = 20
	results, err := agg.client.GetBlocksRange(shardId, from, to+1, true, batchSize)
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
		if err := agg.validateAndProcessBlock(ctx, block); err != nil {
			return fmt.Errorf("error validating and storing main shard block %s: %w", block.Hash, err)
		}
	}

	return nil
}

func (agg *Aggregator) fetchChildBlocks(ctx context.Context, block *jsonrpc.RPCBlock) error {
	childBlocks := block.ChildBlocks
	for i, childBlockHash := range childBlocks {
		shardId := coreTypes.ShardId(i + 1)
		childBlock, err := agg.getBlock(shardId, childBlockHash)
		if err != nil {
			return err
		}
		if err = agg.validateAndProcessBlock(ctx, childBlock); err != nil {
			return fmt.Errorf("error validating and storing block %s for main shard block %s: %w", childBlockHash, block.Hash, err)
		}
	}
	return nil
}

// processNewBlocks fetches and processes new blocks for all shards.
// It handles the overall flow of block synchronization and proof creation.
func (agg *Aggregator) processNewBlocks(ctx context.Context) error {
	agg.metrics.StartProcessingMeasurment()
	defer agg.metrics.EndProcessingMeasurment(ctx)

	latestBlock, err := agg.fetchLatestBlock()
	if err != nil {
		agg.metrics.RecordError(ctx)
		return err
	}

	if err := agg.processShardBlocks(ctx, latestBlock.Number); err != nil {
		agg.metrics.RecordError(ctx)
		return fmt.Errorf("error processing blocks from shard %d: %w", coreTypes.MainShardId, err)
	}

	return nil
}

// processShardBlocks handles the processing of new blocks for a main shard.
// It fetches new blocks, updates the storage, and records relevant metrics.
func (agg *Aggregator) processShardBlocks(ctx context.Context, latestBlockNum coreTypes.BlockNumber) error {
	shardId := coreTypes.MainShardId
	lastFetchedBlockNum, err := agg.blockStorage.GetLastFetchedBlockNum(ctx, shardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	switch {
	case errors.Is(err, db.ErrKeyNotFound):
		// If there is no such shard info in db, we need to init it
		if err = agg.fetchAndProcessBlocks(ctx, latestBlockNum, latestBlockNum); err != nil {
			return err
		}

	case latestBlockNum > lastFetchedBlockNum:
		if err := agg.fetchAndProcessBlocks(ctx, lastFetchedBlockNum+1, latestBlockNum); err != nil {
			return err
		}

	default:
		agg.logger.Debug().Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, latestBlockNum).Msg("no new blocks to fetch")
	}

	agg.metrics.SetCurrentBlockHeight(ctx, int64(latestBlockNum), uint32(shardId))

	return nil
}
