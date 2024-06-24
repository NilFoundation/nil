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
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

const (
	defaultTimeout = time.Second

	maxMessagesInBlock = 100
)

var sharedLogger = logging.NewLogger("collator")

type collator struct {
	id      types.ShardId
	nShards int

	pool MsgPool

	topology    ShardTopology
	neighborIds []types.ShardId
	state       types.CollatorState
	traceEVM    bool

	logger zerolog.Logger

	executionState *execution.ExecutionState
	roTx           db.RoTx
	rwTx           db.RwTx
}

func newCollator(id types.ShardId, nShards int, topology ShardTopology, traceEVM bool, pool MsgPool, logger zerolog.Logger) *collator {
	return &collator{
		pool:        pool,
		id:          id,
		nShards:     nShards,
		logger:      logger,
		neighborIds: topology.GetNeighbors(id, nShards, true),
		topology:    topology,
		traceEVM:    traceEVM,
	}
}

func (c *collator) GenerateZeroState(ctx context.Context, txFabric db.DB, zerostate string) error {
	if err := c.init(ctx, txFabric); err != nil {
		return err
	}
	defer c.clear()

	c.logger.Trace().Stringer("shard", c.id).Msg("Generating zero-state...")

	if err := c.executionState.GenerateZeroState(zerostate); err != nil {
		return err
	}

	if _, err := c.finalize(); err != nil {
		return err
	}

	return nil
}

// TODO: Make this dynamically calculated based on the network conditions and current shard gas price
var ForwardFee = uint256.NewInt(100)

func (c *collator) GenerateBlock(ctx context.Context, txFabric db.DB) error {
	if err := c.init(ctx, txFabric); err != nil {
		return err
	}
	defer c.clear()

	var poolMsgs []*types.Message

	c.logger.Trace().Msg("Collating...")

	inMsgs, outMsgs, err := c.collectFromNeighbors()
	if err != nil {
		return err
	}

	if nPooled := maxMessagesInBlock - len(inMsgs) - len(outMsgs); nPooled != 0 {
		poolMsgs, err = c.pool.Peek(ctx, nPooled, 0)
		if err != nil {
			return err
		}
	}

	if err := HandleMessages(ctx, c.roTx, c.executionState, append(inMsgs, poolMsgs...)); err != nil {
		return err
	}
	for _, msg := range outMsgs {
		if msg.msg.Value.Cmp(ForwardFee) < 0 {
			sharedLogger.Warn().Err(errors.New("message can't pay forward fee")).Msgf("discarding message %v", msg.msg)
			continue
		}
		msg.msg.Value.Sub(&msg.msg.Value.Int, ForwardFee)
		c.executionState.AddOutMessageForTx(msg.inMsgHash, msg.msg)
	}

	block, err := c.finalize()
	if err != nil {
		return err
	}

	// todo: pool should not take too much responsibility, collator must check messages for duplicates
	if err := c.pool.OnNewBlock(ctx, block, poolMsgs); err != nil {
		return err
	}

	return nil
}

func (c *collator) init(ctx context.Context, txFabric db.DB) error {
	var err error

	c.roTx, err = txFabric.CreateRoTx(ctx)
	if err != nil {
		return err
	}

	c.rwTx, err = txFabric.CreateRwTx(ctx)
	if err != nil {
		return err
	}

	c.executionState, err = execution.NewExecutionStateForShard(c.rwTx, c.id, common.NewTimer())
	c.executionState.TraceVm = c.traceEVM
	if err != nil {
		return err
	}

	return nil
}

func (c *collator) clear() {
	if c.roTx != nil {
		c.roTx.Rollback()
		c.roTx = nil
	}
	if c.rwTx != nil {
		c.rwTx.Rollback()
		c.rwTx = nil
	}
	c.executionState = nil
}

func (c *collator) finalize() (*types.Block, error) {
	if err := c.setLastBlockHashes(); err != nil {
		return nil, err
	}

	if err := db.WriteCollatorState(c.rwTx, c.id, c.state); err != nil {
		return nil, err
	}

	blockId := types.BlockNumber(0)
	if !c.executionState.PrevBlock.Empty() {
		b, err := db.ReadBlock(c.rwTx, c.id, c.executionState.PrevBlock)
		if err != nil {
			return nil, err
		}
		blockId = b.Id + 1
	}

	blockHash, err := c.executionState.Commit(blockId)
	if err != nil {
		return nil, err
	}

	block, err := execution.PostprocessBlock(c.rwTx, c.id, blockHash)
	if err != nil {
		return nil, err
	}

	if err := c.rwTx.Commit(); err != nil {
		return nil, err
	}

	return block, nil
}

func (c *collator) setLastBlockHashes() error {
	if types.IsMasterShard(c.id) {
		for i := 1; i < c.nShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(c.roTx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			c.executionState.SetShardHash(shardId, lastBlockHash)
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(c.roTx, types.MasterShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		c.executionState.SetMasterchainHash(lastBlockHash)
	}
	return nil
}

type OutMessage struct {
	inMsgHash common.Hash
	msg       *types.Message
}

func (c *collator) collectFromNeighbors() ([]*types.Message, []*OutMessage, error) {
	state, err := db.ReadCollatorState(c.roTx, c.id)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil, err
	}

	neighborIndexes := common.SliceToMap(state.Neighbors, func(i int, t types.Neighbor) (types.ShardId, int) {
		return t.ShardId, i
	})

	var inMsgs []*types.Message
	var outMsgs []*OutMessage

SHARDS:
	for _, neighborId := range c.neighborIds {
		position, ok := neighborIndexes[neighborId]
		if !ok {
			position = len(neighborIndexes)
			neighborIndexes[neighborId] = position
			state.Neighbors = append(state.Neighbors, types.Neighbor{ShardId: neighborId})
		}
		neighbor := &state.Neighbors[position]

	BLOCKS:
		for {
			block, err := db.ReadBlockByNumber(c.roTx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break BLOCKS
			}
			if err != nil {
				return nil, nil, err
			}

			outMsgTrie := execution.NewMessageTrieReader(mpt.NewReaderWithRoot(c.roTx, neighborId, db.MessageTrieTable, block.OutMessagesRoot))
			for ; neighbor.MessageIndex < block.OutMessagesNum; neighbor.MessageIndex++ {
				msg, err := outMsgTrie.Fetch(neighbor.MessageIndex)
				if err != nil {
					return nil, nil, err
				}

				dstShardId := msg.To.ShardId()
				if dstShardId == c.id {
					sharedLogger.Debug().Msgf("Adding %s message to %v to shard %v", msg.Kind, msg.To, c.id)
					inMsgs = append(inMsgs, msg)
				} else if c.id != neighborId && c.topology.ShouldPropagateMsg(neighborId, c.id, dstShardId) {
					// TODO: add inMsgHash support (do we even need it?)
					outMsgs = append(outMsgs, &OutMessage{msg: msg})
				}

				if len(inMsgs)+len(outMsgs) >= maxMessagesInBlock {
					break SHARDS
				}
			}
			neighbor.BlockNumber++
			neighbor.MessageIndex = 0
		}
	}

	c.state = state
	return inMsgs, outMsgs, nil
}
