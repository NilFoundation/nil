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
	ssz "github.com/ferranbt/fastssz"
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

	nbId          []types.ShardId
	nbBlockNumber types.BlockNumberList
	topology      ShardTopology
}

func newCollator(shard shardchain.BlockGenerator, pool MsgPool, id types.ShardId, nShards int, logger *zerolog.Logger, topology ShardTopology) *collator {
	nbId := topology.GetNeighbours(id, nShards)
	nbBlockNumber := types.BlockNumberList{List: make([]uint64, len(nbId))}
	return &collator{
		shard:         shard,
		pool:          pool,
		id:            id,
		nShards:       nShards,
		logger:        logger,
		timer:         common.NewTimer(),
		nbId:          nbId,
		nbBlockNumber: nbBlockNumber,
		topology:      topology,
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

		if err := shardchain.GenerateZeroState(ctx, es); err != nil {
			return err
		}
	} else {
		c.logger.Trace().Msgf("Collating on shard %s...", c.id)

		// todo: store last block id
		inmsgs, outmsgs, err := c.collectFromNeighbours(roTx)
		if err != nil {
			return err
		}
		msgs, err = c.pool.Peek(ctx, nMessagesForBlock, 0)
		if err != nil {
			return err
		}
		inmsgs = append(inmsgs, msgs...)

		if err := shardchain.HandleMessages(ctx, es, inmsgs); err != nil {
			return err
		}
		for _, msg := range outmsgs {
			es.AddOutMessage(msg.inMsgHash, msg.msg)
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
	value, err := c.nbBlockNumber.MarshalSSZ()
	if err != nil {
		return err
	}
	return tx.Put(db.NeighbourBlockNumber, c.id.Bytes(), value)
}

type OutMessage struct {
	inMsgHash common.Hash
	msg       *types.Message
}

func (c *collator) collectFromNeighbours(roTx db.RoTx) (inmsgs []*types.Message, outmsgs []*OutMessage, err error) {
	c.nbBlockNumber, err = db.ReadNbBlockNumbers(roTx, c.id, len(c.nbId))

	process := func(id types.ShardId, blockNumber *uint64) {
		for {
			var block *types.Block
			block, err = db.ReadBlockByNumber(roTx, id, types.BlockNumber(*blockNumber))
			if block == nil || err != nil {
				break
			}
			outMsgTrie := mpt.NewMerklePatriciaTrieWithRoot(roTx, id, db.MessageTrieTable, block.OutMessagesRoot)
			for msgIndex := range block.OutMessagesNum {
				var msgRaw []byte
				msgRaw, err = outMsgTrie.Get(ssz.MarshalUint32(nil, msgIndex))
				if err != nil {
					return
				}
				msg := new(types.Message)
				if err = msg.UnmarshalSSZ(msgRaw); err != nil {
					return
				}
				msgShardId := msg.To.ShardId()
				if msgShardId == c.id {
					inmsgs = append(inmsgs, msg)
				} else if c.topology.ShouldPropagateMsg(id, c.id, msgShardId) {
					// TODO: add inMsgHash support (do we even need it?)
					outmsgs = append(outmsgs, &OutMessage{inMsgHash: common.EmptyHash, msg: msg})
				}
			}
			*blockNumber += 1
		}
	}
	for i, id := range c.nbId {
		process(id, &c.nbBlockNumber.List[i])
		if err != nil {
			return nil, nil, err
		}
	}
	return inmsgs, outmsgs, err
}
