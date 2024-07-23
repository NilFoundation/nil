package collate

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/rs/zerolog"
)

const (
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
	if c.params.ShardId.IsMainShard() {
		for i := 1; i < c.params.NShards; i++ {
			shardId := types.ShardId(i)
			lastBlockHash, err := db.ReadLastBlockHash(c.roTx, shardId)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}

			c.proposal.ShardHashes[shardId] = lastBlockHash
		}
	} else {
		lastBlockHash, err := db.ReadLastBlockHash(c.roTx, types.MainShardId)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		c.proposal.MainChainHash = lastBlockHash
	}
	return nil
}

func (c *collator) handleMessagesFromPool() ([]*types.Message, error) {
	poolMsgs, err := c.pool.Peek(c.ctx, c.params.MaxInMessagesInBlock-len(c.proposal.InMsgs))
	if err != nil {
		return nil, err
	}

	sa, err := execution.NewStateAccessor()
	if err != nil {
		return nil, err
	}

	es, err := execution.NewROExecutionStateForShard(c.roTx, c.params.ShardId, c.params.Timer, c.params.GasPriceScale)
	if err != nil {
		return nil, err
	}

	validate := func(msg *types.Message) (bool, error) {
		hash := msg.Hash()

		if msgData, err := sa.Access(c.roTx, c.params.ShardId).GetInMessage().ByHash(hash); err != nil &&
			!errors.Is(err, db.ErrKeyNotFound) {
			return false, err
		} else if err == nil && msgData.Message() != nil {
			c.logger.Trace().Stringer(logging.FieldMessageHash, hash).
				Msg("Message is already in the blockchain. Dropping...")
			return false, nil
		}

		if res := execution.ValidateExternalMessage(es, msg); res.FatalError != nil {
			return false, res.FatalError
		} else if res.Error != nil {
			// todo: we should run full transactions, because we cannot validate without it
			// for now, skip VM errors here, they will be caught by the generator
			if !vm.IsVMError(res.Error) {
				execution.AddFailureReceipt(hash, msg.To, res)
				return false, nil
			}
		}

		if err := es.SetExtSeqno(msg.To, msg.Seqno+1); err != nil {
			return false, err
		}

		return true, nil
	}

	nExternal := 0
	for ; c.shouldContinue() && nExternal < len(poolMsgs); nExternal++ {
		msg := poolMsgs[nExternal]
		c.proposal.RemoveFromPool = append(c.proposal.RemoveFromPool, msg)

		if ok, err := validate(msg); err != nil {
			return nil, err
		} else if ok {
			c.proposal.InMsgs = append(c.proposal.InMsgs, msg)
		}
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

		var lastBlockNumber types.BlockNumber
		lastBlock, err := db.ReadLastBlock(c.roTx, neighborId)
		if !errors.Is(err, db.ErrKeyNotFound) {
			if err != nil {
				return err
			}
			lastBlockNumber = lastBlock.Id
		}

		for c.shouldContinue() {
			// We will break the loop when lastBlockNumber is reached anyway,
			// but in case of read-through mode, we will make unnecessary requests to the server if we don't check it here.
			if lastBlockNumber < neighbor.BlockNumber {
				break
			}
			block, err := db.ReadBlockByNumber(c.roTx, neighborId, neighbor.BlockNumber)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return err
			}

			outMsgTrie := execution.NewDbMessageTrieReader(c.roTx, neighborId)
			outMsgTrie.SetRootHash(block.OutMessagesRoot)
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
