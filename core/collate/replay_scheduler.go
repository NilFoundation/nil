package collate

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type ReplayParams struct {
	execution.BlockGeneratorParams

	Timeout time.Duration

	ReplayBlockNumber types.BlockNumber
}

type ReplayScheduler struct {
	txFabric db.DB

	params ReplayParams

	logger zerolog.Logger
}

func NewReplayScheduler(txFabric db.DB, params ReplayParams) *ReplayScheduler {
	return &ReplayScheduler{
		txFabric: txFabric,
		params:   params,
		logger: logging.NewLogger("block-replayer").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
	}
}

func (s *ReplayScheduler) Run(ctx context.Context) error {
	s.logger.Info().Msg("Starting block replay...")

	if err := s.doReplay(ctx); err != nil {
		return err
	}

	<-ctx.Done()
	s.logger.Info().Msg("Stopping block replay...")
	return nil
}

func (s *ReplayScheduler) doReplay(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.params.Timeout)
	defer cancel()

	proposal, err := s.buildProposalFromPrevBlock(ctx)
	if err != nil {
		return err
	}

	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	if _, err := gen.GenerateBlock(proposal); err != nil {
		return err
	}

	return nil
}

func (s *ReplayScheduler) switchLastBlock(ctx context.Context, blockId types.BlockNumber) (common.Hash, error) {
	rwTx, err := s.txFabric.CreateRwTx(ctx)
	if err != nil {
		return common.EmptyHash, err
	}
	defer rwTx.Rollback()

	blockHash, err := db.ReadBlockHashByNumber(rwTx, s.params.ShardId, blockId)
	if err != nil {
		return common.EmptyHash, err
	}

	s.logger.Debug().Msgf("Switching last block to %s", blockHash)
	if err = db.WriteLastBlockHash(rwTx, s.params.ShardId, blockHash); err != nil {
		return common.EmptyHash, err
	}

	return blockHash, rwTx.Commit()
}

func (s *ReplayScheduler) buildProposalFromPrevBlock(ctx context.Context) (*execution.Proposal, error) {
	if s.params.ShardId == types.MainShardId {
		return nil, errors.New("replay for masterchain is not supported")
	}
	if s.params.ReplayBlockNumber == types.BlockNumber(0) {
		return nil, errors.New("replay for zerostate-block is not supported")
	}

	proposal := &execution.Proposal{PrevBlockId: s.params.ReplayBlockNumber - 1}

	// NOTE: masterchain last block isn't switched now
	if hash, err := s.switchLastBlock(ctx, proposal.PrevBlockId); err != nil {
		return nil, err
	} else {
		proposal.PrevBlockHash = hash
	}

	roTx, err := s.txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer roTx.Rollback()

	prevBlock, err := db.ReadBlock(roTx, s.params.ShardId, proposal.PrevBlockHash)
	if err != nil {
		return nil, err
	}
	blockToReplay, err := db.ReadBlockByNumber(roTx, s.params.ShardId, s.params.ReplayBlockNumber)
	if err != nil {
		return nil, err
	}

	proposal.MainChainHash = prevBlock.MainChainHash
	s.logger.Trace().Msgf("Last block is %s, last MC block is %s", proposal.PrevBlockHash, proposal.MainChainHash)

	// we could also consider option with fairly collecting these messages
	// from neighbor shards and running collator
	// however it's not a purpose of replay mode (at least now)
	inMsgsReader := execution.NewDbMessageTrieReader(roTx, s.params.ShardId)
	inMsgsReader.SetRootHash(blockToReplay.InMessagesRoot)
	entries, err := inMsgsReader.Entries()
	if err != nil {
		return nil, err
	}
	proposal.InMsgs = make([]*types.Message, len(entries))
	for _, inMsg := range entries {
		proposal.InMsgs[inMsg.Key] = inMsg.Val
	}

	outMsgsReader := execution.NewDbMessageTrieReader(roTx, s.params.ShardId)
	outMsgsReader.SetRootHash(blockToReplay.OutMessagesRoot)
	entries, err = outMsgsReader.Entries()
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	proposal.OutMsgs = make([]*types.Message, 0)
	for _, outMsg := range entries {
		if outMsg.Val.From.ShardId() != s.params.ShardId {
			proposal.OutMsgs = append(proposal.OutMsgs, outMsg.Val)
		}
	}

	return proposal, nil
}
