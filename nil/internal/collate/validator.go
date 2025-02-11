package collate

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog"
)

type Validator struct {
	params Params

	txFabric       db.DB
	pool           TxnPool
	networkManager *network.Manager

	logger zerolog.Logger
}

func (s *Validator) BuildProposal(ctx context.Context) (*execution.ProposalSSZ, error) {
	proposer := newProposer(s.params, s.params.Topology, s.pool, s.logger)
	proposal, err := proposer.GenerateProposal(ctx, s.txFabric)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proposal: %w", err)
	}
	return proposal, nil
}

func (s *Validator) VerifyProposal(ctx context.Context, proposal *execution.ProposalSSZ) (*types.Block, error) {
	p, err := execution.ConvertProposal(proposal)
	if err != nil {
		return nil, err
	}

	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric, p.GetMainShardHash(s.params.ShardId))
	if err != nil {
		return nil, fmt.Errorf("failed to create block generator: %w", err)
	}
	defer gen.Rollback()

	res, err := gen.BuildBlock(p, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to generate block: %w", err)
	}
	return res, nil
}

func (s *Validator) InsertProposal(ctx context.Context, proposal *execution.ProposalSSZ, sig types.Signature) error {
	p, err := execution.ConvertProposal(proposal)
	if err != nil {
		return err
	}

	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric, p.GetMainShardHash(s.params.ShardId))
	if err != nil {
		return fmt.Errorf("failed to create block generator: %w", err)
	}
	defer gen.Rollback()

	res, err := gen.GenerateBlock(p, s.logger, sig)
	if err != nil {
		return fmt.Errorf("failed to generate block: %w", err)
	}

	if err := s.pool.OnCommitted(ctx, proposal.ExternalTxns); err != nil {
		s.logger.Warn().Err(err).
			Msgf("Failed to remove %d committed transactions from pool", len(proposal.ExternalTxns))
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, &types.BlockWithExtractedData{
		Block:           res.Block,
		InTransactions:  res.InTxns,
		OutTransactions: res.OutTxns,
		ChildBlocks:     proposal.ShardHashes,
	})
}
