package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/rs/zerolog/log"
)

const (
	defaultPeriod  = time.Second
	defaultTimeout = time.Second

	nMessagesForBlock = 10
)

type Collator struct {
	shards []*shardchain.ShardChain
	pools  []msgpool.Pool
}

func NewCollator(shards []*shardchain.ShardChain, pools []msgpool.Pool) *Collator {
	return &Collator{
		shards: shards,
		pools:  pools,
	}
}

func (c *Collator) Run(ctx context.Context) error {
	log.Info().Msg("Starting collation...")

	funcs := make([]concurrent.Func, len(c.shards))
	for i := range c.shards {
		shard, pool := c.shards[i], c.pools[i] // we must not capture loop-variables in the func
		funcs[i] = func(ctx context.Context) error {
			// todo: remember last block (the pool removes delivered messages, but we shouldn't rely on it)
			msgs, err := pool.Peek(ctx, nMessagesForBlock, 0)
			if err != nil {
				return err
			}

			block, err := shard.GenerateBlock(ctx, msgs)
			if err != nil {
				return err
			}

			return pool.OnNewBlock(ctx, block, msgs, nil)
		}
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
