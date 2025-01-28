package collate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
	"github.com/rs/zerolog"
)

const enableConsensus = true

type TxnPool interface {
	Peek(ctx context.Context, n int) ([]*types.Transaction, error)
	OnCommitted(ctx context.Context, committed []*types.Transaction) error
}

type Consensus interface {
	RunSequence(ctx context.Context, height uint64) error
}

type Params struct {
	execution.BlockGeneratorParams

	MaxInternalGasInBlock          types.Gas
	MaxGasInBlock                  types.Gas
	MaxInternalTransactionsInBlock int
	MaxForwardTransactionsInBlock  int

	CollatorTickPeriod time.Duration
	Timeout            time.Duration

	ZeroState       string
	ZeroStateConfig *execution.ZeroStateConfig
	MainKeysOutPath string

	Topology ShardTopology
}

type Scheduler struct {
	consensus      Consensus
	txFabric       db.DB
	pool           txnpool.Pool
	networkManager *network.Manager

	params Params

	measurer *telemetry.Measurer
	logger   zerolog.Logger
}

func NewScheduler(txFabric db.DB, pool txnpool.Pool, params Params, networkManager *network.Manager) (*Scheduler, error) {
	const name = "github.com/NilFoundation/nil/nil/internal/collate"
	measurer, err := telemetry.NewMeasurer(telemetry.NewMeter(name), "collations",
		telattr.ShardId(params.ShardId))
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		txFabric:       txFabric,
		pool:           pool,
		networkManager: networkManager,
		params:         params,
		measurer:       measurer,
		logger: logging.NewLogger("collator").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
	}, nil
}

func (s *Scheduler) Run(ctx context.Context, consensus Consensus) error {
	s.logger.Info().Msg("Starting collation...")
	s.consensus = consensus

	// At first generate zero-state if needed
	if err := s.generateZeroState(ctx); err != nil {
		return err
	}

	// Enable handler for snapshot relaying
	SetBootstrapHandler(ctx, s.networkManager, s.params.ShardId, s.txFabric)

	// Enable handler for blocks relaying
	SetRequestHandler(ctx, s.networkManager, s.params.ShardId, s.txFabric)

	ticker := time.NewTicker(s.params.CollatorTickPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.doCollate(ctx); err != nil {
				if ctx.Err() != nil {
					s.logger.Info().Msg("Stopping collation...")
					return nil
				}
				s.logger.Error().Err(err).Msg("Failed to collate")
			}
		case <-ctx.Done():
			s.logger.Info().Msg("Stopping collation...")
			return nil
		}
	}
}

func (s *Scheduler) generateZeroState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.params.Timeout)
	defer cancel()

	roTx, err := s.txFabric.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer roTx.Rollback()

	if _, err := db.ReadLastBlockHash(roTx, s.params.ShardId); !errors.Is(err, db.ErrKeyNotFound) {
		// error or nil if last block found
		return err
	}

	if len(s.params.MainKeysOutPath) != 0 && s.params.ShardId == types.BaseShardId {
		if err := execution.DumpMainKeys(s.params.MainKeysOutPath); err != nil {
			return err
		}
	}

	s.logger.Info().Msg("Generating zero-state...")

	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	block, err := gen.GenerateZeroState(s.params.ZeroState, s.params.ZeroStateConfig)
	if err != nil {
		return err
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, &types.BlockWithExtractedData{Block: block})
}

func (s *Scheduler) BuildProposal(ctx context.Context) (*execution.Proposal, error) {
	collator := newCollator(s.params, s.params.Topology, s.pool, s.logger)
	proposal, err := collator.GenerateProposal(ctx, s.txFabric)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proposal: %w", err)
	}
	return proposal, nil
}

func (s *Scheduler) VerifyProposal(ctx context.Context, proposal *execution.Proposal) (*types.Block, error) {
	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
	if err != nil {
		return nil, fmt.Errorf("failed to create block generator: %w", err)
	}
	defer gen.Rollback()

	res, err := gen.BuildBlock(proposal, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to generate block: %w", err)
	}
	return res, nil
}

func (s *Scheduler) InsertProposal(ctx context.Context, proposal *execution.Proposal, sig types.Signature) error {
	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
	if err != nil {
		return fmt.Errorf("failed to create block generator: %w", err)
	}
	defer gen.Rollback()

	res, err := gen.GenerateBlock(proposal, s.logger, sig)
	if err != nil {
		return fmt.Errorf("failed to generate block: %w", err)
	}

	if err := s.pool.OnCommitted(ctx, proposal.RemoveFromPool); err != nil {
		s.logger.Warn().Err(err).Msgf("Failed to remove %d committed transactions from pool", len(proposal.RemoveFromPool))
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, &types.BlockWithExtractedData{
		Block:           res.Block,
		InTransactions:  res.InTxns,
		OutTransactions: res.OutTxns,
		ChildBlocks:     proposal.ShardHashes,
	})
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	if enableConsensus {
		roTx, err := s.txFabric.CreateRoTx(ctx)
		if err != nil {
			return err
		}
		defer roTx.Rollback()

		block, _, err := db.ReadLastBlock(roTx, s.params.ShardId)
		if err != nil {
			return err
		}

		return s.consensus.RunSequence(ctx, block.Id.Uint64()+1)
	} else {
		ctx, cancel := context.WithTimeout(ctx, s.params.Timeout)
		defer cancel()

		proposal, err := s.BuildProposal(ctx)
		if err != nil {
			return err
		}
		return s.InsertProposal(ctx, proposal, nil)
	}
}
