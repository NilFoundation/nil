package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/rs/zerolog/log"
)

const (
	defaultPeriod  = 2 * time.Second
	defaultTimeout = time.Second

	nMessagesForBlock = 10
)

type Collator struct {
	shard *shardchain.ShardChain
	pool  msgpool.Pool
}

func NewCollator(shard *shardchain.ShardChain, pool msgpool.Pool) *Collator {
	return &Collator{
		shard: shard,
		pool:  pool,
	}
}

func (c *Collator) Run(ctx context.Context) error {
	log.Info().Msgf("Starting collation on shard %s...", c.shard.Id)

	// Run shard collations once immediately, then run by timer.
	if err := c.genZerostateIfRequired(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to generate zerostate")
	}

	ticker := time.NewTicker(defaultPeriod)
	for {
		select {
		case <-ticker.C:
			if err := c.doCollate(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			log.Info().Msgf("Stopping collation on shard %s...", c.shard.Id)
			return nil
		}
	}
}

func (c *Collator) genZerostateIfRequired(ctx context.Context) error {
	return c.shard.GenerateZerostate(ctx)
}

func (c *Collator) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	msgs, err := c.pool.Peek(ctx, nMessagesForBlock, 0)
	if err != nil {
		return err
	}

	block, err := c.shard.GenerateBlock(ctx, msgs)
	if err != nil {
		return err
	}

	return c.pool.OnNewBlock(ctx, block, msgs, nil)
}
