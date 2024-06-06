package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

const (
	defaultPeriod  = 2 * time.Second
	defaultTimeout = time.Second

	nMessagesForBlock = 10
)

var sharedLogger = common.NewLogger("collator")

type collator struct {
	shard shardchain.BlockGenerator
	pool  MsgPool

	id      types.ShardId
	nShards int

	logger *zerolog.Logger
	timer  common.Timer

	neighborIds          []types.ShardId
	neighborBlockNumbers types.BlockNumberList
	topology             ShardTopology
}

func newCollator(shard shardchain.BlockGenerator, pool MsgPool, id types.ShardId, nShards int, logger *zerolog.Logger, topology ShardTopology) *collator {
	neighbors := topology.GetNeighbours(id, nShards, true /* includeSelf */)
	return &collator{
		shard:                shard,
		pool:                 pool,
		id:                   id,
		nShards:              nShards,
		logger:               logger,
		timer:                common.NewTimer(),
		neighborIds:          neighbors,
		neighborBlockNumbers: types.BlockNumberList{List: make([]uint64, len(neighbors))},
		topology:             topology,
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

	var poolMsgs []*types.Message
	if lastBlockHash == common.EmptyHash {
		c.logger.Trace().Msgf("Generating zero-state on shard %s...", c.id)

		if err := shardchain.GenerateZeroState(ctx, es); err != nil {
			return err
		}
	} else {
		c.logger.Trace().Msgf("Collating on shard %s...", c.id)

		// todo: store last block id
		inMsgs, outMsgs, err := c.collectFromNeighbours(roTx)
		if err != nil {
			return err
		}

		poolMsgs, err = c.pool.Peek(ctx, nMessagesForBlock, 0)
		if err != nil {
			return err
		}

		if err := shardchain.HandleMessages(ctx, es, append(inMsgs, poolMsgs...)); err != nil {
			return err
		}
		for _, msg := range outMsgs {
			es.AddOutMessage(msg.inMsgHash, msg.msg)
		}
	}

	block, err := c.finalize(es, rwTx, roTx)
	if err != nil {
		return err
	}

	// todo: pool should not take too much responsibility, collator must check messages for duplicates
	if err := c.pool.OnNewBlock(ctx, block, poolMsgs, nil); err != nil {
		return err
	}

	return nil
}

func (c *collator) finalize(es *execution.ExecutionState, rwTx db.RwTx, roTx db.RoTx) (*types.Block, error) {
	if err := c.setLastBlockHashes(roTx, es); err != nil {
		return nil, err
	}

	if err := c.setLastBlockNumbers(rwTx); err != nil {
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
	if types.IsMasterShard(c.id) {
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

func (c *collator) setLastBlockNumbers(tx db.RoTx) error {
	value, err := c.neighborBlockNumbers.MarshalSSZ()
	if err != nil {
		return err
	}
	return tx.Put(db.NeighbourBlockNumber, c.id.Bytes(), value)
}

type OutMessage struct {
	inMsgHash common.Hash
	msg       *types.Message
}

func (c *collator) collectFromNeighbours(roTx db.RoTx) ([]*types.Message, []*OutMessage, error) {
	numbers, err := db.ReadNbBlockNumbers(roTx, c.id, len(c.neighborIds))
	if err != nil {
		return nil, nil, err
	}

	var inMsgs []*types.Message
	var outMsgs []*OutMessage
	for i, srcShardId := range c.neighborIds {
		for {
			block, err := db.ReadBlockByNumber(roTx, srcShardId, types.BlockNumber(numbers.List[i]))
			if err != nil {
				return nil, nil, err
			}
			if block == nil {
				break
			}

			outMsgTrie := execution.NewMessageTrie(mpt.NewMerklePatriciaTrieWithRoot(roTx, srcShardId, db.MessageTrieTable, block.OutMessagesRoot))
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
