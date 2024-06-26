package collate

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

const (
	defaultTimeout = time.Second

	maxInMessagesInBlock  = 1024
	maxOutMessagesInBlock = 1024
)

var sharedLogger = logging.NewLogger("collator")

type collator struct {
	params execution.BlockGeneratorParams

	topology ShardTopology
	pool     MsgPool

	logger zerolog.Logger

	state   types.CollatorState
	txOwner *execution.TxOwner

	inMsgs  []*types.Message
	outMsgs []*types.Message
}

func newCollator(params execution.BlockGeneratorParams, topology ShardTopology, pool MsgPool, logger zerolog.Logger) *collator {
	return &collator{
		params:   params,
		topology: topology,
		pool:     pool,
		logger:   logger,
	}
}

func (c *collator) shouldContinue() bool {
	// todo: we should break collation on some gas condition, not on the number of messages
	return len(c.inMsgs) < maxInMessagesInBlock && len(c.outMsgs) < maxOutMessagesInBlock
}

func (c *collator) GenerateZeroState(ctx context.Context, txFabric db.DB, zeroState string) error {
	var err error
	c.txOwner, err = execution.NewTxOwner(ctx, txFabric)
	if err != nil {
		return err
	}
	defer c.txOwner.Rollback()

	c.logger.Info().Msg("Generating zero-state...")

	gen, err := execution.NewBlockGenerator(c.params, c.txOwner)
	if err != nil {
		return err
	}

	if err := gen.GenerateZeroState(zeroState); err != nil {
		return err
	}

	return c.finalize()
}

func (c *collator) GenerateBlock(ctx context.Context, txFabric db.DB) error {
	var err error
	c.txOwner, err = execution.NewTxOwner(ctx, txFabric)
	if err != nil {
		return err
	}
	defer c.txOwner.Rollback()

	c.logger.Debug().Msg("Collating...")

	if err := c.handleMessagesFromNeighbors(); err != nil {
		return err
	}

	poolMsgs, err := c.handleMessagesFromPool()
	if err != nil {
		return err
	}

	blockGenerator, err := execution.NewBlockGenerator(c.params, c.txOwner)
	if err != nil {
		return err
	}

	block, err := blockGenerator.GenerateBlock(c.inMsgs, c.outMsgs, c.params.GasBasePrice)
	if err != nil {
		return err
	}

	if err := c.finalize(); err != nil {
		return err
	}

	// todo: pool should not take too much responsibility, collator must check messages for duplicates
	if err := c.pool.OnNewBlock(ctx, block, poolMsgs); err != nil {
		return err
	}

	return nil
}

func (c *collator) handleMessagesFromPool() ([]*types.Message, error) {
	// todo: take messages one by one
	poolMsgs, err := c.pool.Peek(c.txOwner.Ctx, maxInMessagesInBlock-len(c.inMsgs), 0)
	if err != nil {
		return nil, err
	}

	nExternal := 0
	for ; c.shouldContinue() && nExternal < len(poolMsgs); nExternal++ {
		c.inMsgs = append(c.inMsgs, poolMsgs[nExternal])
	}

	return poolMsgs[:nExternal], nil
}

func (c *collator) finalize() error {
	if err := db.WriteCollatorState(c.txOwner.RwTx, c.params.ShardId, c.state); err != nil {
		return err
	}

	return c.txOwner.Commit()
}

func (c *collator) handleMessagesFromNeighbors() error {
	state, err := db.ReadCollatorState(c.txOwner.RoTx, c.params.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	neighborIndexes := common.SliceToMap(state.Neighbors, func(i int, t types.Neighbor) (types.ShardId, int) {
		return t.ShardId, i
	})

	for _, neighborId := range c.topology.GetNeighbors(c.params.ShardId, c.params.NShards, true) {
		position, ok := neighborIndexes[neighborId]
		if !ok {
			position = len(neighborIndexes)
			neighborIndexes[neighborId] = position
			state.Neighbors = append(state.Neighbors, types.Neighbor{ShardId: neighborId})
		}
		neighbor := &state.Neighbors[position]

		for c.shouldContinue() {
			block, err := db.ReadBlockByNumber(c.txOwner.RoTx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return err
			}

			outMsgTrie := execution.NewMessageTrieReader(mpt.NewReaderWithRoot(c.txOwner.RoTx, neighborId, db.MessageTrieTable, block.OutMessagesRoot))
			for ; c.shouldContinue() && neighbor.MessageIndex < block.OutMessagesNum; neighbor.MessageIndex++ {
				msg, err := outMsgTrie.Fetch(neighbor.MessageIndex)
				if err != nil {
					return err
				}

				if msg.To.ShardId() == c.params.ShardId {
					c.inMsgs = append(c.inMsgs, msg)
				} else if c.params.ShardId != neighborId {
					if c.topology.ShouldPropagateMsg(neighborId, c.params.ShardId, msg.To.ShardId()) {
						c.outMsgs = append(c.outMsgs, msg)
					}
				}
			}

			neighbor.BlockNumber++
			neighbor.MessageIndex = 0
		}
	}

	c.state = state
	return nil
}
