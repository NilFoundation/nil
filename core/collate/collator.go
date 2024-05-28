package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

const (
	defaultPeriod  = 2 * time.Second
	defaultTimeout = time.Second

	nMessagesForBlock = 10
)

type collator struct {
	shard shardchain.BlockGenerator
	pool  MsgPool

	id      types.ShardId
	nShards int

	logger *zerolog.Logger
	timer  common.Timer
}

func newCollator(shard shardchain.BlockGenerator, pool MsgPool, id types.ShardId, nShards int, logger *zerolog.Logger) *collator {
	return &collator{
		shard:   shard,
		pool:    pool,
		id:      id,
		nShards: nShards,
		logger:  logger,
		timer:   common.NewTimer(),
	}
}

func (c *collator) GenerateBlock(ctx context.Context) error {
	roTx, err := c.shard.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer roTx.Rollback()

	rwTx, err := c.shard.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer rwTx.Rollback()

	es, err := execution.NewExecutionStateForShard(rwTx, c.id, c.timer)
	if err != nil {
		return err
	}

	lastBlockHash, err := db.ReadLastBlockHash(roTx, c.id)
	if err != nil {
		return err
	}

	var msgs []*types.Message
	if lastBlockHash == common.EmptyHash {
		c.logger.Trace().Msgf("Generating zero-state on shard %s...", c.id)

		if err := c.shard.GenerateZeroState(ctx, es); err != nil {
			return err
		}
	} else {
		c.logger.Trace().Msgf("Collating on shard %s...", c.id)

		// todo: store last block id
		// todo: collect messages from neighbors first
		msgs, err = c.pool.Peek(ctx, nMessagesForBlock, 0)
		if err != nil {
			return err
		}

		if err := c.shard.HandleMessages(ctx, es, msgs); err != nil {
			return err
		}
	}

	block, err := c.finalize(es, rwTx, roTx)
	if err != nil {
		return err
	}

	// todo: pool should not take too much responsibility, collator must check messages for duplicates
	if err := c.pool.OnNewBlock(ctx, block, msgs, nil); err != nil {
		return err
	}

	return nil
}

func (c *collator) finalize(es *execution.ExecutionState, rwTx db.RwTx, roTx db.RoTx) (*types.Block, error) {
	if err := c.setLastBlockHashes(roTx, es); err != nil {
		return nil, err
	}

	blockId := types.BlockNumber(0)
	if es.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(rwTx, c.id, es.PrevBlock).Id + 1
	}

	blockHash, err := es.Commit(blockId)
	if err != nil {
		return nil, err
	}

	block, err := execution.PostprocessBlock(rwTx, c.id, blockHash)
	if err != nil {
		return nil, err
	}

	if err := rwTx.Commit(); err != nil {
		return nil, err
	}

	return block, err
}

func (c *collator) setLastBlockHashes(tx db.RoTx, es *execution.ExecutionState) error {
	if c.id == types.MasterShardId {
		for i := 1; i < c.nShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(tx, shardId)
			if err != nil {
				return err
			}
			es.SetShardHash(shardId, lastBlockHash)
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(tx, types.MasterShardId)
		if err != nil {
			return err
		}
		es.SetMasterchainHash(lastBlockHash)
	}
	return nil
}
