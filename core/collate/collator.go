package collate

import (
	"context"
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
	defaultPeriod  = 2 * time.Second
	defaultTimeout = time.Second

	nMessagesForBlock = 10
)

var sharedLogger = logging.NewLogger("collator")

type collator struct {
	id      types.ShardId
	nShards int

	pool MsgPool

	topology             ShardTopology
	neighborIds          []types.ShardId
	neighborBlockNumbers types.BlockNumberList

	logger zerolog.Logger

	state *execution.ExecutionState
	roTx  db.RoTx
	rwTx  db.RwTx
}

func newCollator(id types.ShardId, nShards int, topology ShardTopology, pool MsgPool, logger zerolog.Logger) *collator {
	neighbors := topology.GetNeighbours(id, nShards, true)
	return &collator{
		pool:                 pool,
		id:                   id,
		nShards:              nShards,
		logger:               logger,
		neighborIds:          neighbors,
		neighborBlockNumbers: types.BlockNumberList{List: make([]uint64, len(neighbors))},
		topology:             topology,
	}
}

func (c *collator) GenerateBlock(ctx context.Context, txFabric db.DB) error {
	if err := c.init(ctx, txFabric); err != nil {
		return err
	}
	defer c.clear()

	lastBlockHash, err := db.ReadLastBlockHash(c.roTx, c.id)
	if err != nil {
		return err
	}

	var poolMsgs []*types.Message
	if lastBlockHash == common.EmptyHash {
		c.logger.Trace().Msg("Generating zero-state...")

		if err := c.state.GenerateZeroState(ctx); err != nil {
			return err
		}
	} else {
		c.logger.Trace().Msg("Collating...")

		// todo: store last block id
		inMsgs, outMsgs, err := c.collectFromNeighbours()
		if err != nil {
			return err
		}

		poolMsgs, err = c.pool.Peek(ctx, nMessagesForBlock, 0)
		if err != nil {
			return err
		}

		if err := HandleMessages(ctx, c.roTx, c.state, append(inMsgs, poolMsgs...)); err != nil {
			return err
		}
		for _, msg := range outMsgs {
			c.state.AddOutMessage(msg.inMsgHash, msg.msg)
		}
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

	c.state, err = execution.NewExecutionStateForShard(c.rwTx, c.id, common.NewTimer())
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
	c.state = nil
}

func (c *collator) finalize() (*types.Block, error) {
	if err := c.setLastBlockHashes(); err != nil {
		return nil, err
	}

	if err := c.setLastBlockNumbers(); err != nil {
		return nil, err
	}

	blockId := types.BlockNumber(0)
	if c.state.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(c.rwTx, c.id, c.state.PrevBlock).Id + 1
	}

	blockHash, err := c.state.Commit(blockId)
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
			if err != nil {
				return err
			}
			c.state.SetShardHash(shardId, lastBlockHash)
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(c.roTx, types.MasterShardId)
		if err != nil {
			return err
		}
		c.state.SetMasterchainHash(lastBlockHash)
	}
	return nil
}

func (c *collator) setLastBlockNumbers() error {
	value, err := c.neighborBlockNumbers.MarshalSSZ()
	if err != nil {
		return err
	}
	return c.rwTx.Put(db.NeighbourBlockNumber, c.id.Bytes(), value)
}

type OutMessage struct {
	inMsgHash common.Hash
	msg       *types.Message
}

func (c *collator) collectFromNeighbours() ([]*types.Message, []*OutMessage, error) {
	numbers, err := db.ReadNbBlockNumbers(c.roTx, c.id, len(c.neighborIds))
	if err != nil {
		return nil, nil, err
	}

	var inMsgs []*types.Message
	var outMsgs []*OutMessage
	for i, srcShardId := range c.neighborIds {
		for {
			block, err := db.ReadBlockByNumber(c.roTx, srcShardId, types.BlockNumber(numbers.List[i]))
			if err != nil {
				return nil, nil, err
			}
			if block == nil {
				break
			}

			outMsgTrie := execution.NewMessageTrieReader(mpt.NewReaderWithRoot(c.roTx, srcShardId, db.MessageTrieTable, block.OutMessagesRoot))
			for msgIndex := range block.OutMessagesNum {
				msg, err := outMsgTrie.Fetch(msgIndex)
				if err != nil {
					return nil, nil, err
				}

				dstShardId := msg.To.ShardId()
				if dstShardId == c.id {
					inMsgs = append(inMsgs, msg)
				} else if c.id != srcShardId && c.topology.ShouldPropagateMsg(srcShardId, c.id, dstShardId) {
					// TODO: add inMsgHash support (do we even need it?)
					outMsgs = append(outMsgs, &OutMessage{msg: msg})
				}
			}
			numbers.List[i]++
		}
	}

	c.neighborBlockNumbers = numbers
	return inMsgs, outMsgs, nil
}
