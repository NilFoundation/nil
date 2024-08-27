package synccommittee

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type Aggregator struct {
	logger   zerolog.Logger
	client   *rpc.Client
	storage  *BlockStorage
	proposer *Proposer
	metrics  *MetricsHandler
}

func NewAggregator(client *rpc.Client, logger zerolog.Logger) (*Aggregator, error) {
	metrics, err := NewMetricsHandler("github.com/NilFoundation/nil/nil/services/sync_committee")
	if err != nil {
		return nil, err
	}

	return &Aggregator{
		logger:   logger,
		client:   client,
		storage:  NewBlockStorage(),
		proposer: NewProposer("", logger),
		metrics:  metrics,
	}, nil
}

// getShardIdList retrieves the list of all shard IDs, including the main shard.
func (agg *Aggregator) getShardIdList() ([]types.ShardId, error) {
	shardIdList, err := agg.client.GetShardIdList()
	if err != nil {
		return nil, err
	}
	return append(shardIdList, types.MainShardId), nil
}

// fetchLatestBlocks retrieves the latest block for each shard in the provided list.
func (agg *Aggregator) fetchLatestBlocks(shardIdList []types.ShardId) (map[types.ShardId]*jsonrpc.RPCBlock, error) {
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

	latestBlocks := make(map[types.ShardId]*jsonrpc.RPCBlock)
	for i, result := range results {
		shardId := shardIdList[i]
		block, ok := result.(*jsonrpc.RPCBlock)
		if !ok {
			return nil, fmt.Errorf("invalid type for BatchCall result, expected *jsonrpc.RPCBlock for block: %d on shard %d", i, shardId)
		}
		latestBlocks[shardId] = block
	}

	return latestBlocks, nil
}

func (agg *Aggregator) proofThresholdMet() bool {
	lastProvedBlkNum := agg.storage.GetLastProvedBlockNum(types.MainShardId)
	lastFetchedBlockNum := agg.storage.GetLastFetchedBlockNum(types.MainShardId)
	return lastProvedBlkNum != lastFetchedBlockNum
}

// updateLastProvedBlockNumForAllShards updates the last proved block number for all shards to their respective last fetched block number
// so they will be cleaned from the storage afterwards. This is temp solution while we have dummy proof creation.
func (agg *Aggregator) updateLastProvedBlockNumForAllShards() error {
	shardIdList, err := agg.getShardIdList()
	if err != nil {
		return fmt.Errorf("failed to get shard list for updating last proved block numbers: %w", err)
	}

	for _, shardId := range shardIdList {
		lastFetchedBlockNum := agg.storage.GetLastFetchedBlockNum(shardId)
		if lastFetchedBlockNum != 0 {
			agg.storage.SetLastProvedBlockNum(shardId, lastFetchedBlockNum)
		} else {
			agg.logger.Warn().
				Stringer(logging.FieldShardId, shardId).
				Msg("no last fetched block found for shard")
		}
	}
	return nil
}

// Ask proposual send proof to L1
func (agg *Aggregator) sendProof() error {
	lastProvedBlockNum := agg.storage.GetLastProvedBlockNum(types.MainShardId)
	lastFetchedBlockNum := agg.storage.GetLastFetchedBlockNum(types.MainShardId)
	lastProvedBlock := agg.storage.GetBlock(types.MainShardId, lastProvedBlockNum)
	if lastProvedBlock == nil {
		return fmt.Errorf("failed to get last proved block: %d", lastProvedBlockNum)
	}
	lastFetchedBlock := agg.storage.GetBlock(types.MainShardId, lastFetchedBlockNum)
	if lastFetchedBlock == nil {
		return fmt.Errorf("failed to get last fetched block: %d", lastFetchedBlockNum)
	}
	provedStateRoot := lastProvedBlock.ChildBlocksRootHash
	newStateRoot := lastFetchedBlock.ChildBlocksRootHash
	transactions := agg.storage.GetTransactionsByBlocksRange(types.MainShardId, lastProvedBlockNum, lastFetchedBlockNum)
	agg.logger.Info().
		Stringer("provedStateRoot", provedStateRoot).
		Stringer("newStateRoot", newStateRoot).
		Int64("blkCount", int64(lastFetchedBlockNum-lastProvedBlockNum)).
		Int64("transactionsCount", int64(len(transactions))).Msg("send proof")
	err := agg.proposer.sendProof(provedStateRoot, newStateRoot, transactions)
	if err != nil {
		return fmt.Errorf("failed send proof: %w", err)
	}
	return nil
}

// createProofTasks generates proof tasks for the main shard blocks.
func (agg *Aggregator) createProofTasks(ctx context.Context, blockForProof *jsonrpc.RPCBlock) {
	if blockForProof.ShardId != types.MainShardId {
		agg.logger.Debug().Stringer(logging.FieldShardId, blockForProof.ShardId).Msg("skip create proof tasks for not main shard")
		return
	}
	// For testnet we create proofs only for main shard blocks
	lastProvedBlockNum := agg.storage.GetLastProvedBlockNum(types.MainShardId)
	lastFetchedBlockNum := agg.storage.GetLastFetchedBlockNum(types.MainShardId)
	if lastProvedBlockNum >= blockForProof.Number || lastFetchedBlockNum >= blockForProof.Number {
		agg.logger.Debug().Stringer(logging.FieldShardId, types.MainShardId).Int64("targetBlockNum", int64(blockForProof.Number)).Int64("lastFetchedBlockNum", int64(lastFetchedBlockNum)).Int64("lastProvedBlockNum", int64(lastProvedBlockNum)).Msg("skip create proof tasks, because the last fetched block already proved")
		return
	}

	// TODO: add actual creation logic here

	agg.logger.Info().Stringer(logging.FieldShardId, types.MainShardId).Int64("blkNum", int64(blockForProof.Number)).Msg("proof tasks created")
	agg.metrics.RecordBlocksInTasks(ctx, 1)
}

// validateAndStoreBlock checks the validity of a block and stores it if valid.
func (agg *Aggregator) validateAndStoreBlock(ctx context.Context, block *jsonrpc.RPCBlock) error {
	prevBlock := agg.storage.GetBlock(block.ShardId, block.Number-1)
	if prevBlock != nil && prevBlock.Hash != block.ParentHash {
		return fmt.Errorf("block hash mismatch for block %d", block.Number)
	}

	// Start genetating proof during collection blocks
	agg.createProofTasks(ctx, block)

	agg.storage.SetBlock(block)
	return nil
}

// fetchAndStoreBlocks retrieves a range of blocks for a specific shard and stores them.
func (agg *Aggregator) fetchAndStoreBlocks(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber) error {
	batch := agg.client.CreateBatchRequest()
	for i := from; i <= to; i++ {
		if _, err := batch.GetBlock(shardId, transport.BlockNumber(i), true); err != nil {
			return fmt.Errorf("error creating block request for shard %d, block %d: %w", shardId, i, err)
		}
	}

	results, err := agg.client.BatchCall(batch)
	if err != nil {
		return fmt.Errorf("error fetching blocks from shard %d: %w", shardId, err)
	}

	agg.logger.Debug().Int64("blkCount", int64(len(results))).Stringer(logging.FieldShardId, shardId).Msg("fetching blocks range")

	for _, result := range results {
		block, ok := result.(*jsonrpc.RPCBlock)
		if !ok {
			return fmt.Errorf("invalid type for BatchCall result, expected *jsonrpc.RPCBlock. Shard %d", shardId)
		}
		if err := agg.validateAndStoreBlock(ctx, block); err != nil {
			agg.logger.Warn().Err(err).Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, block.Number).Msg("error validating and storing block")
		}
	}

	return nil
}

// processNewBlocks fetches and processes new blocks for all shards.
// It handles the overall flow of block synchronization and proof creation.
func (agg *Aggregator) ProcessNewBlocks(ctx context.Context) error {
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

	if agg.proofThresholdMet() {
		// Should be called from TaskScheduler, now added here just for check Proposual
		err = agg.sendProof()
		if err != nil {
			agg.metrics.RecordError(ctx)
			return err
		}
		// Update last proved block number for all shards and clean the storage
		err = agg.updateLastProvedBlockNumForAllShards()
		if err != nil {
			agg.metrics.RecordError(ctx)
			return err
		}
		agg.storage.CleanupStorage()
	}

	return nil
}

// processShardBlocks handles the processing of new blocks for a specific shard.
// It fetches new blocks, updates the storage, and records relevant metrics.
func (agg *Aggregator) processShardBlocks(ctx context.Context, shardId types.ShardId, latestBlockNum types.BlockNumber) error {
	lastFetchedBlockNum := agg.storage.GetLastFetchedBlockNum(shardId)
	switch {
	case lastFetchedBlockNum == 0:
		agg.storage.SetLastProvedBlockNum(shardId, latestBlockNum)
		if err := agg.fetchAndStoreBlocks(ctx, shardId, latestBlockNum, latestBlockNum); err != nil {
			return err
		}
		agg.metrics.RecordBlocksFetched(ctx, 1)

	case latestBlockNum > lastFetchedBlockNum:
		blocksFetched := latestBlockNum - lastFetchedBlockNum
		if err := agg.fetchAndStoreBlocks(ctx, shardId, lastFetchedBlockNum+1, latestBlockNum); err != nil {
			return err
		}
		agg.metrics.RecordBlocksFetched(ctx, int64(blocksFetched))

	default:
		agg.logger.Debug().Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, latestBlockNum).Msg("no new blocks to fetch")
	}

	agg.metrics.SetCurrentBlockHeight(ctx, int64(latestBlockNum), uint32(shardId))

	return nil
}
