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
	"golang.org/x/sync/errgroup"
)

type BlockObserver struct {
	client  *rpc.Client
	storage *BlockStorage
	metrics *MetricsHandler
	logger  zerolog.Logger
}

func NewBlockObserver(client *rpc.Client, storage *BlockStorage, metrics *MetricsHandler, logger zerolog.Logger) *BlockObserver {
	return &BlockObserver{
		client:  client,
		storage: storage,
		metrics: metrics,
		logger:  logger,
	}
}

func (bo *BlockObserver) ProcessNewBlocks(ctx context.Context) error {
	bo.metrics.StartProcessingMeasurment()
	defer bo.metrics.EndProcessingMeasurment(ctx)

	shardIdList, err := bo.getShardIdList()
	if err != nil {
		bo.metrics.RecordError(ctx)
		return err
	}

	latestBlocks, err := bo.fetchLatestBlocks(shardIdList)
	if err != nil {
		bo.metrics.RecordError(ctx)
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	for shardId, latestBlock := range latestBlocks {
		g.Go(func() error {
			if err := bo.processShardBlocks(ctx, shardId, latestBlock); err != nil {
				bo.metrics.RecordError(ctx)
				return err
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error processing shard blocks: %w", err)
	}

	return nil
}

func (bo *BlockObserver) getShardIdList() ([]types.ShardId, error) {
	shardIdList, err := bo.client.GetShardIdList()
	if err != nil {
		return nil, fmt.Errorf("failed to get shards list: %w", err)
	}
	return append(shardIdList, types.MainShardId), nil
}

func (bo *BlockObserver) fetchLatestBlocks(shardIdList []types.ShardId) (map[types.ShardId]*jsonrpc.RPCBlock, error) {
	batch := bo.client.CreateBatchRequest()
	for _, shardId := range shardIdList {
		if _, err := batch.GetBlock(shardId, "latest", false); err != nil {
			bo.logger.Panic().Err(err).Stringer(logging.FieldShardId, shardId).Msg("error creating latest block request")
		}
	}
	results, err := bo.client.BatchCall(batch)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest blocks from shards: %w", err)
	}

	latestBlocks := make(map[types.ShardId]*jsonrpc.RPCBlock)
	for i, result := range results {
		shardId := shardIdList[i]
		block, ok := result.(*jsonrpc.RPCBlock)
		if !ok {
			bo.logger.Panic().Err(err).Stringer(logging.FieldShardId, shardId).Msg("invalid type for BatchCall result, expected *jsonrpc.RPCBlock")
		}
		latestBlocks[shardId] = block
	}

	return latestBlocks, nil
}

func (bo *BlockObserver) processShardBlocks(ctx context.Context, shardId types.ShardId, latestBlock *jsonrpc.RPCBlock) error {
	lastFetchedBlock := bo.storage.GetLastFetchedBlock(shardId)
	switch {
	case lastFetchedBlock == nil:
		bo.storage.SetLastProvedBlockNum(shardId, latestBlock.Number-1)
		if err := bo.fetchAndStoreBlocks(shardId, latestBlock.Number, latestBlock.Number); err != nil {
			return err
		}
		bo.metrics.RecordBlocksFetched(ctx, 1)

	case latestBlock.Number > lastFetchedBlock.Number:
		blocksFetched := latestBlock.Number - lastFetchedBlock.Number
		if err := bo.fetchAndStoreBlocks(shardId, lastFetchedBlock.Number+1, latestBlock.Number); err != nil {
			return err
		}
		bo.metrics.RecordBlocksFetched(ctx, int64(blocksFetched))

	default:
		bo.logger.Debug().Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, latestBlock.Number).Msg("no new blocks to fetch")
	}

	bo.metrics.SetCurrentBlockHeight(ctx, int64(latestBlock.Number), uint32(shardId))

	return nil
}

func (bo *BlockObserver) fetchAndStoreBlocks(shardId types.ShardId, from, to types.BlockNumber) error {
	bo.logger.Debug().Stringer(logging.FieldShardId, shardId).Msg("fetching blocks range")

	batch := bo.client.CreateBatchRequest()
	for i := from; i <= to; i++ {
		if _, err := batch.GetBlock(shardId, transport.BlockNumber(i), true); err != nil {
			bo.logger.Panic().Err(err).Stringer(logging.FieldShardId, shardId).Msg("error creating block request")
		}
	}

	results, err := bo.client.BatchCall(batch)
	if err != nil {
		return fmt.Errorf("error fetching blocks from shard %d: %w", shardId, err)
	}

	for _, result := range results {
		block, ok := result.(*jsonrpc.RPCBlock)
		if !ok {
			bo.logger.Panic().Err(err).Stringer(logging.FieldShardId, shardId).Msg("invalid type for BatchCall result, expected *jsonrpc.RPCBlock")
		}
		if err := bo.validateAndStoreBlock(shardId, block); err != nil {
			bo.logger.Warn().Err(err).Stringer(logging.FieldShardId, shardId).Stringer(logging.FieldBlockNumber, block.Number).Msg("error validating and storing block")
		}
	}

	return nil
}

func (bo *BlockObserver) validateAndStoreBlock(shardId types.ShardId, block *jsonrpc.RPCBlock) error {
	prevBlock := bo.storage.GetBlock(shardId, block.Number-1)
	if prevBlock != nil && prevBlock.Hash != block.ParentHash {
		return fmt.Errorf("block hash mismatch for block %d", block.Number)
	}

	bo.storage.SetBlock(shardId, block.Number, block)
	return nil
}
