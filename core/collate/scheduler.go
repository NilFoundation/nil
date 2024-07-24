package collate

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/rs/zerolog"
)

type MsgPool interface {
	Peek(ctx context.Context, n int) ([]*types.Message, error)
	OnCommitted(ctx context.Context, committed []*types.Message) error
}

type Params struct {
	execution.BlockGeneratorParams

	MaxInMessagesInBlock  int
	MaxOutMessagesInBlock int
}

type Scheduler struct {
	txFabric db.DB
	pool     MsgPool

	params             Params
	topology           ShardTopology
	collatorTickPeriod time.Duration
	timeout            time.Duration

	ZeroState       string
	MainKeysOutPath string

	logger zerolog.Logger
}

func NewScheduler(txFabric db.DB, pool MsgPool, params Params, topology ShardTopology, collatorTickPeriod time.Duration) *Scheduler {
	return &Scheduler{
		txFabric:           txFabric,
		pool:               pool,
		params:             params,
		topology:           topology,
		collatorTickPeriod: collatorTickPeriod,
		timeout:            collatorTickPeriod,
		logger: logging.NewLogger("collator").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info().Msg("Starting collation...")

	// At first generate zero-state if needed
	if err := s.generateZeroState(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(s.collatorTickPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.doCollate(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			s.logger.Info().Msg("Stopping collation...")
			return nil
		}
	}
}

func (s *Scheduler) generateZeroState(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	roTx, err := s.txFabric.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer roTx.Rollback()

	lastBlockHash, err := db.ReadLastBlockHash(roTx, s.params.ShardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if lastBlockHash == common.EmptyHash {
		if len(s.MainKeysOutPath) != 0 && s.params.ShardId == types.MainShardId {
			if err := execution.DumpMainKeys(s.MainKeysOutPath); err != nil {
				return err
			}
		}

		s.logger.Info().Msg("Generating zero-state...")

		gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
		if err != nil {
			return err
		}
		defer gen.Rollback()

		return gen.GenerateZeroState(s.ZeroState)
	}
	return nil
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	collator := newCollator(s.params, s.topology, s.pool, s.logger)
	proposal, err := collator.GenerateProposal(ctx, s.txFabric)
	if err != nil {
		return err
	}

	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	if err := gen.GenerateBlock(proposal, s.params.GasBasePrice); err != nil {
		return err
	}

	if err := s.pool.OnCommitted(ctx, proposal.RemoveFromPool); err != nil {
		s.logger.Warn().Err(err).Msgf("Failed to remove %d committed messages from pool", len(proposal.RemoveFromPool))
	}

	return nil
}

func (s *Scheduler) GetMsgPool() msgpool.Pool {
	pool, ok := s.pool.(msgpool.Pool)
	check.PanicIfNotf(ok, "scheduler pool is not a msgpool.Pool")
	return pool
}
