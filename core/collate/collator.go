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

	defaultMaxInMessagesInBlock  = 200
	defaultMaxOutMessagesInBlock = 200
)

var sharedLogger = logging.NewLogger("collator")

type collator struct {
	params Params

	topology ShardTopology
	pool     MsgPool

	logger zerolog.Logger

	proposal *execution.Proposal

	ctx  context.Context
	roTx db.RoTx
}

func newCollator(params Params, topology ShardTopology, pool MsgPool, logger zerolog.Logger) *collator {
	if params.MaxInMessagesInBlock == 0 {
		params.MaxInMessagesInBlock = defaultMaxInMessagesInBlock
	}
	if params.MaxOutMessagesInBlock == 0 {
		params.MaxOutMessagesInBlock = defaultMaxOutMessagesInBlock
	}
	return &collator{
		params:   params,
		topology: topology,
		pool:     pool,
		logger:   logger,
	}
}

func (c *collator) shouldContinue() bool {
	// todo: we should break collation on some gas condition, not on the number of messages
	return len(c.proposal.InMsgs) < c.params.MaxInMessagesInBlock && len(c.proposal.OutMsgs) < c.params.MaxOutMessagesInBlock
}

func (c *collator) GenerateProposal(ctx context.Context, txFabric db.DB) (*execution.Proposal, error) {
	c.proposal = execution.NewEmptyProposal()

	var err error
	c.roTx, err = txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer c.roTx.Rollback()

	c.logger.Trace().Msg("Collating...")

	if err := c.fetchPrevBlock(); err != nil {
		return nil, err
	}

	if err := c.fetchLastBlockHashes(); err != nil {
		return nil, err
	}

	if err := c.handleMessagesFromNeighbors(); err != nil {
		return nil, err
	}

	if _, err := c.handleMessagesFromPool(); err != nil {
		return nil, err
	}

	c.logger.Trace().Msgf("Collected %d in messages and %d out messages", len(c.proposal.InMsgs), len(c.proposal.OutMsgs))

	return c.proposal, nil
}

func (c *collator) fetchPrevBlock() error {
	b, err := db.ReadLastBlock(c.roTx, c.params.ShardId)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	c.proposal.PrevBlockId = b.Id
	c.proposal.PrevBlockHash = b.Hash()
	return nil
}

func (c *collator) fetchLastBlockHashes() error {
	if types.IsMasterShard(c.params.ShardId) {
		for i := 1; i < c.params.NShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(c.roTx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			c.proposal.ShardHashes[shardId] = lastBlockHash
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(c.roTx, types.MasterShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		c.proposal.MainChainHash = lastBlockHash
	}
	return nil
}

func (c *collator) handleMessagesFromPool() ([]*types.Message, error) {
	// todo: take messages one by one
	poolMsgs, err := c.pool.Peek(c.ctx, c.params.MaxInMessagesInBlock-len(c.proposal.InMsgs), 0)
	if err != nil {
		return nil, err
	}

	nExternal := 0
	for ; c.shouldContinue() && nExternal < len(poolMsgs); nExternal++ {
		c.proposal.InMsgs = append(c.proposal.InMsgs, poolMsgs[nExternal])
	}

	return poolMsgs[:nExternal], nil
}

func (c *collator) handleMessagesFromNeighbors() error {
	state, err := db.ReadCollatorState(c.roTx, c.params.ShardId)
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
			block, err := db.ReadBlockByNumber(c.roTx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return err
			}

			outMsgTrie := execution.NewMessageTrieReader(mpt.NewReaderWithRoot(c.roTx, neighborId, db.MessageTrieTable, block.OutMessagesRoot))
			for ; c.shouldContinue() && neighbor.MessageIndex < block.OutMessagesNum; neighbor.MessageIndex++ {
				msg, err := outMsgTrie.Fetch(neighbor.MessageIndex)
				if err != nil {
					return err
				}

				if msg.To.ShardId() == c.params.ShardId {
					c.proposal.InMsgs = append(c.proposal.InMsgs, msg)
				} else if c.params.ShardId != neighborId {
					if c.topology.ShouldPropagateMsg(neighborId, c.params.ShardId, msg.To.ShardId()) {
						c.proposal.OutMsgs = append(c.proposal.OutMsgs, msg)
					}
				}
			}

			if neighbor.MessageIndex == block.OutMessagesNum {
				neighbor.BlockNumber++
				neighbor.MessageIndex = 0
			}
		}
	}

	c.proposal.CollatorState = state
	return nil
}
