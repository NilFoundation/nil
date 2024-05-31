package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type MsgPool interface {
	Peek(ctx context.Context, n int, onTopOf uint64) ([]*types.Message, error)
	OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message, tx db.Tx) error
}

type Scheduler struct {
	shard *shardchain.ShardChain
	pool  MsgPool

	id       types.ShardId
	nShards  int
	topology ShardTopology

	logger *zerolog.Logger
}

func NewScheduler(shard *shardchain.ShardChain, pool MsgPool, id types.ShardId, nShards int, topology ShardTopology) *Scheduler {
	return &Scheduler{
		shard:    shard,
		pool:     pool,
		id:       id,
		nShards:  nShards,
		topology: topology,
		logger:   common.NewLogger("collator-" + id.String()),
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	sharedLogger.Info().Msgf("Starting collation on shard %s...", s.shard.Id)

	// Run shard collations once immediately, then run by timer.
	if err := s.doCollate(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(defaultPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.doCollate(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			sharedLogger.Info().Msgf("Stopping collation on shard %s...", s.shard.Id)
			return nil
		}
	}
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	collator := newCollator(s.shard, s.pool, s.id, s.nShards, s.logger, s.topology)
	return collator.GenerateBlock(ctx)
}
