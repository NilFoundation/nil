package collate

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
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
	ZeroStateConfig *execution.ZeroStateConfig
	MainKeysOutPath string

	Topology ShardTopology
}

type Scheduler struct {
	txFabric       db.DB
	pool           msgpool.Pool
	networkManager *network.Manager

	params Params

	measurer *telemetry.Measurer
	logger   zerolog.Logger
}

func NewScheduler(txFabric db.DB, pool msgpool.Pool, params Params, networkManager *network.Manager) (*Scheduler, error) {
	const name = "github.com/NilFoundation/nil/nil/internal/collate"
	measurer, err := telemetry.NewMeasurer(telemetry.NewMeter(name), "collate")
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

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info().Msg("Starting collation...")

	// At first generate zero-state if needed
	if err := s.generateZeroState(ctx); err != nil {
		return err
	}

	// Enable handler for snapshot relaying
	if err := SetBootstrapHandler(ctx, s.networkManager, s.params.ShardId, s.txFabric); err != nil {
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

	block, err := gen.GenerateZeroState(s.params.ZeroState, s.params.ZeroStateConfig)
	if err != nil {
		return err
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, &Block{Block: block})
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.params.Timeout)
	defer cancel()

	s.measurer.Restart()
	defer s.measurer.Measure(ctx)

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

	block, outs, err := gen.GenerateBlock(proposal)
	if err != nil {
		return err
	}

	if err := s.pool.OnCommitted(ctx, proposal.RemoveFromPool); err != nil {
		s.logger.Warn().Err(err).Msgf("Failed to remove %d committed messages from pool", len(proposal.RemoveFromPool))
	}

	return PublishBlock(ctx, s.networkManager, s.params.ShardId, &Block{Block: block, OutMessages: outs})
}

func (s *Scheduler) GetMsgPool() msgpool.Pool {
	return s.pool
}
