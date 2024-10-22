package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
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

// getShardIdList retrieves the list of all shard IDs, including the main shard.
func (agg *Aggregator) getShardIdList() ([]coreTypes.ShardId, error) {
	shardIdList, err := agg.client.GetShardIdList()
	if err != nil {
		return nil, err
	}
	return append(shardIdList, coreTypes.MainShardId), nil
}

// fetchLatestBlocks retrieves the latest block for each shard in the provided list.
func (agg *Aggregator) fetchLatestBlocks(shardIdList []coreTypes.ShardId) (map[coreTypes.ShardId]*jsonrpc.RPCBlock, error) {
	batch := agg.client.CreateBatchRequest()
	for _, shardId := range shardIdList {
		if _, err := batch.GetBlock(shardId, "latest", false); err != nil {
			return nil, fmt.Errorf("error fetching latest block from shard %d: %w", shardId, err)
		}
	}
	results, err := agg.client.BatchCall(batch)
	if err != nil {
		return nil, fmt.Errorf("failed fetching latest blocks: %w", err)
	}

	latestBlocks := make(map[coreTypes.ShardId]*jsonrpc.RPCBlock)
	for i, result := range results {
		if result == nil {
			return nil, fmt.Errorf("no latest block found for shard %d", shardIdList[i])
		}
		shardId := shardIdList[i]
		block, ok := result.(*jsonrpc.RPCBlock)
		if !ok {
			return nil, fmt.Errorf("invalid type for BatchCall result, expected *jsonrpc.RPCBlock for block: %d on shard %d", i, shardId)
		}
		latestBlocks[shardId] = block
	}

	return latestBlocks, nil
}

// createProofTask generates proof tasks for the main shard blocks.
func (agg *Aggregator) createProofTask(ctx context.Context, blockForProof *jsonrpc.RPCBlock) error {
	if blockForProof.ShardId != coreTypes.MainShardId {
		agg.logger.Debug().Stringer(logging.FieldShardId, blockForProof.ShardId).Msg("skip create proof tasks for not main shard")
		return nil
	}

	proofProviderTask := types.NewBlockProofTaskEntry(blockForProof.ShardId, blockForProof.Number, blockForProof.Hash)
	if err := agg.taskStorage.AddSingleTaskEntry(ctx, *proofProviderTask); err != nil {
		return err
	}

	agg.logger.Info().Stringer(logging.FieldShardId, coreTypes.MainShardId).Int64("blkNum", int64(blockForProof.Number)).Msg("proof tasks created")
	agg.metrics.RecordBlocksInTasks(ctx, 1)
	return nil
}

// validateAndProcessBlock checks the validity of a block and stores it if valid.
func (agg *Aggregator) validateAndProcessBlock(ctx context.Context, block *jsonrpc.RPCBlock) error {
	prevBlock, err := agg.blockStorage.GetBlock(ctx, block.ShardId, block.Number-1)
	if err != nil {
		return err
	}
	if prevBlock != nil && prevBlock.Hash != block.ParentHash {
		return &BlockHashMismatchError{
			Expected: prevBlock.Hash,
			Got:      block.ParentHash,
		}
	}

	if err = agg.blockStorage.SetBlock(ctx, block.ShardId, block.Number, block); err != nil {
		return err
	}

	// Start generating proof during blocks fetching
	return agg.createProofTask(ctx, block)
}

// fetchAndProcessBlocks retrieves a range of blocks for a specific shard, stores them, creates proof tasks
func (agg *Aggregator) fetchAndProcessBlocks(ctx context.Context, shardId coreTypes.ShardId, from, to coreTypes.BlockNumber) error {
	const batchSize = 20
	results, err := agg.client.GetBlocksRange(shardId, from, to+1, true, batchSize)
	if err != nil {
		return fmt.Errorf("error fetching blocks from shard %d: %w", shardId, err)
	}

	fetchedBlocksLen := len(results)
	agg.logger.Debug().Int64("blkCount", int64(fetchedBlocksLen)).Stringer(logging.FieldShardId, shardId).Msg("fetched blocks range")
	agg.metrics.RecordBlocksFetched(ctx, int64(fetchedBlocksLen))

	for _, block := range results {
		if err := agg.validateAndProcessBlock(ctx, block); err != nil {
			// TODO: add reorg handling
			agg.logger.Warn().Err(err).Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, block.Number).Msg("error validating and storing block")
		}
	}

	return nil
}

// processNewBlocks fetches and processes new blocks for all shards.
// It handles the overall flow of block synchronization and proof creation.
func (agg *Aggregator) processNewBlocks(ctx context.Context) error {
	agg.metrics.StartProcessingMeasurment()
	defer agg.metrics.EndProcessingMeasurment(ctx)

	shardIdList, err := agg.getShardIdList()
	if err != nil {
		agg.metrics.RecordError(ctx)
		return err
	}

	latestBlocks, err := agg.fetchLatestBlocks(shardIdList)
	if err != nil {
		agg.metrics.RecordError(ctx)
		return err
	}

	for shardId, latestBlock := range latestBlocks {
		if err := agg.processShardBlocks(ctx, shardId, latestBlock.Number); err != nil {
			agg.metrics.RecordError(ctx)
			return fmt.Errorf("error processing blocks from shard %d: %w", shardId, err)
		}
	}

	return nil
}

// processShardBlocks handles the processing of new blocks for a specific shard.
// It fetches new blocks, updates the storage, and records relevant metrics.
func (agg *Aggregator) processShardBlocks(ctx context.Context, shardId coreTypes.ShardId, latestBlockNum coreTypes.BlockNumber) error {
	lastFetchedBlockNum, err := agg.blockStorage.GetLastFetchedBlockNum(ctx, shardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	switch {
	case errors.Is(err, db.ErrKeyNotFound):
		// If there is no such shard info in db, we need to init it
		block, err := agg.client.GetBlock(shardId, int(latestBlockNum-1), true)
		if err != nil {
			return fmt.Errorf("error fetching block %d from shard %d: %w", latestBlockNum-1, shardId, err)
		}
		if err = agg.blockStorage.SetBlock(ctx, shardId, block.Number, block); err != nil {
			return err
		}
		if err = agg.fetchAndProcessBlocks(ctx, shardId, latestBlockNum, latestBlockNum); err != nil {
			return err
		}

	case latestBlockNum > lastFetchedBlockNum:
		if err := agg.fetchAndProcessBlocks(ctx, shardId, lastFetchedBlockNum+1, latestBlockNum); err != nil {
			return err
		}

	default:
		agg.logger.Debug().Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, latestBlockNum).Msg("no new blocks to fetch")
	}

	agg.metrics.SetCurrentBlockHeight(ctx, int64(latestBlockNum), uint32(shardId))

	return nil
}
