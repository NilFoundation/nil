package fetching

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type LagTrackerMetrics interface {
	metrics.BasicMetrics
	RecordFetchingLag(ctx context.Context, shardId coreTypes.ShardId, blocksCount int64)
}

type LagTrackerStorage interface {
	GetLatestFetched(ctx context.Context) (types.BlockRefs, error)
}

type LagTrackerConfig struct {
	CheckInterval time.Duration
}

func NewDefaultLagTrackerConfig() LagTrackerConfig {
	return LagTrackerConfig{
		CheckInterval: 5 * time.Minute,
	}
}

type lagTracker struct {
	srv.WorkerLoop

	fetcher RpcBlockFetcher
	storage LagTrackerStorage
	metrics LagTrackerMetrics
	logger  logging.Logger
	config  LagTrackerConfig
}

func NewLagTracker(
	fetcher RpcBlockFetcher,
	storage LagTrackerStorage,
	metrics LagTrackerMetrics,
	config LagTrackerConfig,
	logger logging.Logger,
) *lagTracker {
	tracker := &lagTracker{
		fetcher: fetcher,
		storage: storage,
		metrics: metrics,
		config:  config,
	}

	tracker.WorkerLoop = srv.NewWorkerLoop("lag_tracker", tracker.config.CheckInterval, tracker.runIteration)
	tracker.logger = srv.WorkerLogger(logger, tracker)
	return tracker
}

func (t *lagTracker) runIteration(ctx context.Context) {
	t.logger.Debug().Msg("running lag tracker iteration")

	lagPerShard, err := t.getLagForAllShards(ctx)
	if err != nil {
		t.logger.Error().Err(err).Msg("failed to fetch lag per shard")
		t.metrics.RecordError(ctx, t.Name())
		return
	}

	for shardId, blocksCount := range lagPerShard {
		t.metrics.RecordFetchingLag(ctx, shardId, blocksCount)
		t.logger.Trace().Stringer(logging.FieldShardId, shardId).Msgf("lag in shard %s: %d", shardId, blocksCount)
	}

	t.logger.Debug().Msg("lag tracker iteration completed")
}

func (t *lagTracker) getLagForAllShards(ctx context.Context) (map[coreTypes.ShardId]int64, error) {
	shardIds, err := t.fetcher.GetShardIdList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shard ids: %w", err)
	}

	latestFetched, err := t.storage.GetLatestFetched(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest fetched from storage: %w", err)
	}

	lagPerShard := make(map[coreTypes.ShardId]int64)

	for _, shardId := range shardIds {
		blocksCount, err := t.getShardLag(ctx, latestFetched, shardId)
		if err != nil {
			return nil, err
		}
		lagPerShard[shardId] = blocksCount
	}

	return lagPerShard, nil
}

func (t *lagTracker) getShardLag(
	ctx context.Context,
	latestFetched types.BlockRefs,
	shardId coreTypes.ShardId,
) (blocksCount int64, err error) {
	actualLatestInShard, err := t.getLatestInShard(ctx, shardId)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch latestBlockNumber for shard %d: %w", shardId, err)
	}

	latestFetchedInShard := latestFetched.TryGet(shardId)

	if latestFetchedInShard == nil {
		return int64(*actualLatestInShard), nil
	}

	lag := int64(*actualLatestInShard - latestFetchedInShard.Number)
	return lag, nil
}

func (t *lagTracker) getLatestInShard(ctx context.Context, shardId coreTypes.ShardId) (*coreTypes.BlockNumber, error) {
	block, err := t.fetcher.GetBlock(ctx, shardId, "latest", false)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest block from shard %d: %w", shardId, err)
	}
	if block == nil {
		return nil, fmt.Errorf("latest main block not found in shard %d", shardId)
	}

	return &block.Number, nil
}
