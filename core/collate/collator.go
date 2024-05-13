package collate

import (
	"context"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/rs/zerolog/log"
	"time"
)

const (
	defaultPeriod  = time.Second
	defaultTimeout = time.Second
)

type Collator struct {
	shards []*shardchain.ShardChain
}

func NewCollator(shards []*shardchain.ShardChain) *Collator {
	return &Collator{
		shards: shards,
	}
}

func (c *Collator) Run(ctx context.Context) error {
	log.Info().Msg("Starting collation...")

	funcs := make([]concurrent.Func, len(c.shards))
	for i := range c.shards {
		shard := c.shards[i] // we must not capture loop-variables in the func
		funcs[i] = func(ctx context.Context) error { return shard.Collate(ctx) }
	}

	// Run shard collations once immediately, then run by timer.
	if err := concurrent.RunWithTimeout(ctx, defaultTimeout, funcs...); err != nil {
		return err
	}

	ticker := time.NewTicker(defaultPeriod)
	for {
		select {
		case <-ticker.C:
			if err := concurrent.RunWithTimeout(ctx, defaultTimeout, funcs...); err != nil {
				return err
			}
		case <-ctx.Done():
			log.Info().Msg("Stopping collation...")
			return nil
		}
	}
}
