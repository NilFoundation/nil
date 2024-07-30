package collate

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/network"
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

	CollatorTickPeriod time.Duration
	Timeout            time.Duration

	ZeroState       string
	MainKeysOutPath string

	Topology ShardTopology
}

type Scheduler struct {
	txFabric       db.DB
	pool           MsgPool
	networkManager *network.Manager

	params Params

	logger zerolog.Logger
}

func NewScheduler(txFabric db.DB, pool MsgPool, params Params, networkManager *network.Manager) *Scheduler {
	return &Scheduler{
		txFabric:       txFabric,
		pool:           pool,
		networkManager: networkManager,
		params:         params,
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

	ticker := time.NewTicker(s.params.CollatorTickPeriod)
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

	block, err := gen.GenerateZeroState(s.params.ZeroState)
	if err != nil {
		return err
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, block)
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.params.Timeout)
	defer cancel()

	collator := newCollator(s.params, s.params.Topology, s.pool, s.logger)
	proposal, err := collator.GenerateProposal(ctx, s.txFabric)
	if err != nil {
		return err
	}

	gen, err := execution.NewBlockGenerator(ctx, s.params.BlockGeneratorParams, s.txFabric)
	if err != nil {
		return err
	}
	defer gen.Rollback()

	block, err := gen.GenerateBlock(proposal)
	if err != nil {
		return err
	}

	if err := s.pool.OnCommitted(ctx, proposal.RemoveFromPool); err != nil {
		s.logger.Warn().Err(err).Msgf("Failed to remove %d committed messages from pool", len(proposal.RemoveFromPool))
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, block)
}

func (s *Scheduler) GetMsgPool() msgpool.Pool {
	pool, ok := s.pool.(msgpool.Pool)
	check.PanicIfNotf(ok, "scheduler pool is not a msgpool.Pool")
	return pool
}
