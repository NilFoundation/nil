package synccommittee

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

const (
	MaxBloksPerShard = uint64(10)
)

type Aggregator struct {
	logger       zerolog.Logger
	client       *rpc.Client
	blockStorage *BlockStorage
	taskStorage  ProverTaskStorage
	proposer     *Proposer
	metrics      *MetricsHandler
}

func NewAggregator(client *rpc.Client, proposer *Proposer, database db.DB, logger zerolog.Logger, metrics *MetricsHandler) (*Aggregator, error) {
	return &Aggregator{
		logger:       logger,
		client:       client,
		blockStorage: NewBlockStorage(database),
		taskStorage:  NewTaskStorage(database),
		proposer:     proposer,
		metrics:      metrics,
	}, nil
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
		shardId := shardIdList[i]
		block, ok := result.(*jsonrpc.RPCBlock)
		if !ok {
			return nil, fmt.Errorf("invalid type for BatchCall result, expected *jsonrpc.RPCBlock for block: %d on shard %d", i, shardId)
		}
		latestBlocks[shardId] = block
	}

	return latestBlocks, nil
}

func (agg *Aggregator) proofThresholdMet(ctx context.Context) (bool, error) {
	lastProvedBlockNum, err := agg.blockStorage.GetLastProvedBlockNum(ctx, coreTypes.MainShardId)
	if err != nil {
		return false, err
	}
	lastFetchedBlockNum, err := agg.blockStorage.GetLastFetchedBlockNum(ctx, coreTypes.MainShardId)
	if err != nil {
		return false, err
	}
	return lastProvedBlockNum < lastFetchedBlockNum, nil
}

// updateLastProvedBlockNumForAllShards updates the last proved block number for all shards to their respective last fetched block number
// so they will be cleaned from the storage afterwards. This is temp solution while we have dummy proof creation.
func (agg *Aggregator) updateLastProvedBlockNumForAllShards(ctx context.Context) error {
	shardIdList, err := agg.getShardIdList()
	if err != nil {
		return fmt.Errorf("failed to get shard list for updating last proved block numbers: %w", err)
	}

	for _, shardId := range shardIdList {
		lastFetchedBlockNum, err := agg.blockStorage.GetLastFetchedBlockNum(ctx, shardId)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				agg.logger.Info().Stringer(logging.FieldShardId, shardId).Msg("there are no fetched blocks yet, no last proved num will be set")
				continue
			}
			return err
		}
		if err := agg.blockStorage.SetLastProvedBlockNum(ctx, shardId, lastFetchedBlockNum); err != nil {
			return err
		}
	}
	return nil
}

// Ask proposer to send proof to L1
func (agg *Aggregator) sendProof(ctx context.Context) error {
	lastProvedBlockNum, err := agg.blockStorage.GetLastProvedBlockNum(ctx, coreTypes.MainShardId)
	if err != nil {
		return err
	}
	lastFetchedBlockNum, err := agg.blockStorage.GetLastFetchedBlockNum(ctx, coreTypes.MainShardId)
	if err != nil {
		return err
	}
	lastProvedBlock, err := agg.blockStorage.GetBlock(ctx, coreTypes.MainShardId, lastProvedBlockNum)
	if err != nil {
		return err
	}
	lastFetchedBlock, err := agg.blockStorage.GetBlock(ctx, coreTypes.MainShardId, lastFetchedBlockNum)
	if err != nil {
		return err
	}
	var provedStateRoot common.Hash
	if lastProvedBlock != nil {
		provedStateRoot = lastProvedBlock.ChildBlocksRootHash
	}
	newStateRoot := lastFetchedBlock.ChildBlocksRootHash
	transactions, err := agg.blockStorage.GetTransactionsByBlocksRange(ctx, coreTypes.MainShardId, lastProvedBlockNum, lastFetchedBlockNum)
	if err != nil {
		return err
	}

	agg.logger.Info().
		Stringer("provedStateRoot", provedStateRoot).
		Stringer("newStateRoot", newStateRoot).
		Int64("blkCount", int64(lastFetchedBlockNum-lastProvedBlockNum)).
		Int64("transactionsCount", int64(len(transactions))).Msg("send proof")
	// temporary soluton for check Proposer, actually should be called from TaskScheduler ater generate proof
	err = agg.proposer.SendProof(provedStateRoot, newStateRoot, transactions)
	if err != nil {
		return fmt.Errorf("failed send proof: %w", err)
	}
	return nil
}

var circuitTypes [4]types.CircuitType = [...]types.CircuitType{types.Bytecode, types.MPT, types.ReadWrite, types.ZKEVM}

func prepareTasksForBlock(blockNumber coreTypes.BlockNumber) []*types.ProverTaskEntry {
	taskEntries := make(map[types.ProverTaskId]*types.ProverTaskEntry)

	// Create partial proof tasks (top level, no dependencies)
	partialProofTasks := make(map[types.CircuitType]types.ProverTaskId)
	for _, ct := range circuitTypes {
		partialProveTaskEntry := types.NewPartialProveTaskEntry(0, blockNumber, ct)
		taskEntries[partialProveTaskEntry.Task.Id] = partialProveTaskEntry
		partialProofTasks[ct] = partialProveTaskEntry.Task.Id
	}

	// aggregate FRI task depends on all the previous tasks
	aggFRITaskEntry := types.NewAggregateFRITaskEntry(0, blockNumber)
	aggFRITaskID := aggFRITaskEntry.Task.Id
	taskEntries[aggFRITaskID] = aggFRITaskEntry

	// Second level of circuit-dependent tasks
	consistencyCheckTasks := make(map[types.CircuitType]types.ProverTaskId)
	for _, ct := range circuitTypes {
		taskEntry := types.NewFRIConsistencyCheckTaskEntry(0, blockNumber, ct)
		consistencyCheckTasks[ct] = taskEntry.Task.Id
		taskEntries[taskEntry.Task.Id] = taskEntry
	}

	// Final task, depends on all the previous ones
	mergeProofTaskEntry := types.NewMergeProofTaskEntry(0, blockNumber)
	mergeProofTaskId := mergeProofTaskEntry.Task.Id
	taskEntries[mergeProofTaskId] = mergeProofTaskEntry

	// Set pending dependencies

	// Partial proof results go to all other levels of tasks
	for ct, id := range partialProofTasks {
		ppEntry := taskEntries[id]
		ppEntry.PendingDeps = append(ppEntry.PendingDeps, aggFRITaskID, consistencyCheckTasks[ct], mergeProofTaskId)
	}

	for _, id := range consistencyCheckTasks {
		ccEntry := taskEntries[id]
		// consistency check task result goes to merge proof task
		ccEntry.PendingDeps = append(ccEntry.PendingDeps, mergeProofTaskId)
		// aggregate FRI task result goes to all consistency check tasks
		aggFRITaskEntry.PendingDeps = append(aggFRITaskEntry.PendingDeps, id)
	}

	// Also aggregate FRI task result must be forwarded to merge proof task
	aggFRITaskEntry.PendingDeps = append(aggFRITaskEntry.PendingDeps, mergeProofTaskId)
	return slices.Collect(maps.Values(taskEntries))
}

// createProofTasks generates proof tasks for the main shard blocks.
func (agg *Aggregator) createProofTasks(ctx context.Context, blockForProof *jsonrpc.RPCBlock) error {
	if blockForProof.ShardId != coreTypes.MainShardId {
		agg.logger.Debug().Stringer(logging.FieldShardId, blockForProof.ShardId).Msg("skip create proof tasks for not main shard")
		return nil
	}
	// For testnet we create proofs only for main shard blocks
	lastProvedBlockNum, err := agg.blockStorage.GetLastProvedBlockNum(ctx, coreTypes.MainShardId)
	if err != nil {
		return err
	}
	if blockForProof.Number <= lastProvedBlockNum {
		agg.logger.Debug().
			Stringer(logging.FieldShardId, coreTypes.MainShardId).
			Int64("targetBlockNum", int64(blockForProof.Number)).
			Int64("lastProvedBlockNum", int64(lastProvedBlockNum)).
			Msg("skip create proof tasks, because the last fetched block already proved")
		return nil
	}

	blockTasks := prepareTasksForBlock(blockForProof.Number)
	if err := agg.taskStorage.AddTaskEntries(ctx, blockTasks); err != nil {
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

	// Start genetating proof during blocks fetching
	return agg.createProofTasks(ctx, block)
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
	proofThresholdMet, err := agg.proofThresholdMet(ctx)
	if err != nil {
		return err
	}
	if proofThresholdMet {
		// Should be called from TaskScheduler, now added here just for check Proposal
		if err = agg.sendProof(ctx); err != nil {
			agg.metrics.RecordError(ctx)
			return err
		}
		// Update last proved block number for all shards and clean the storage
		if err = agg.updateLastProvedBlockNumForAllShards(ctx); err != nil {
			agg.metrics.RecordError(ctx)
			return err
		}
		if err = agg.blockStorage.CleanupStorage(ctx); err != nil {
			agg.metrics.RecordError(ctx)
			return err
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
		if err = agg.blockStorage.SetLastProvedBlockNum(ctx, shardId, latestBlockNum-1); err != nil {
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
