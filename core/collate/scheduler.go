package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type MsgPool interface {
	Peek(ctx context.Context, n int, onTopOf uint64) ([]*types.Message, error)
	OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message, tx db.Tx) error
}

type Scheduler struct {
	txFabric db.DB
	pool     MsgPool

	id       types.ShardId
	nShards  int
	topology ShardTopology

	logger *zerolog.Logger
}

func NewScheduler(txFabric db.DB, pool MsgPool, id types.ShardId, nShards int, topology ShardTopology) *Scheduler {
	return &Scheduler{
		txFabric: txFabric,
		pool:     pool,
		id:       id,
		nShards:  nShards,
		topology: topology,
		logger:   common.NewLogger("collator-" + id.String()),
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	sharedLogger.Info().Msgf("Starting collation on shard %s...", s.id)

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
			sharedLogger.Info().Msgf("Stopping collation on shard %s...", s.id)
			return nil
		}
	}
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	collator := newCollator(s.id, s.nShards, s.topology, s.pool, s.logger)
	return collator.GenerateBlock(ctx, s.txFabric)
}
