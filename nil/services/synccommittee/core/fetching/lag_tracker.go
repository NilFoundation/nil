package fetching

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type LagTrackerMetrics interface {
	srv.WorkerMetrics
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

	fetcher *Fetcher
	storage LagTrackerStorage
	metrics LagTrackerMetrics
	config  LagTrackerConfig
}

func NewLagTracker(
	fetcher *Fetcher,
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

	loopConfig := srv.NewWorkerLoopConfig("lag_tracker", tracker.config.CheckInterval, tracker.runIteration)
	tracker.WorkerLoop = srv.NewWorkerLoop(loopConfig, metrics, logger)
	return tracker
}

func (t *lagTracker) runIteration(ctx context.Context) error {
	t.Logger.Debug().Msg("running lag tracker iteration")

	lagPerShard, err := t.getLagForAllShards(ctx)
	switch {
	case errors.Is(err, context.Canceled):
		return err
	case err != nil:
		return fmt.Errorf("failed to fetch lag per shard: %w", err)
	}

	for shardId, blocksCount := range lagPerShard {
		t.metrics.RecordFetchingLag(ctx, shardId, blocksCount)
		t.Logger.Trace().Stringer(logging.FieldShardId, shardId).Msgf("lag in shard %s: %d", shardId, blocksCount)
	}

	t.Logger.Debug().Msg("lag tracker iteration completed")
	return nil
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
	block, err := t.fetcher.GetLatestBlockRef(ctx, shardId)
	if err != nil {
		return nil, err
	}
	return &block.Number, nil
}
