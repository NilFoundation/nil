package collate

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type MsgPool interface {
	Peek(ctx context.Context, n int, onTopOf uint64) ([]*types.Message, error)
	OnNewBlock(ctx context.Context, block *types.Block, committed []*types.Message) error
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
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
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
		if len(s.MainKeysOutPath) != 0 && s.params.ShardId == types.MasterShardId {
			if err := execution.DumpMainKeys(s.MainKeysOutPath); err != nil {
				return err
			}
		}

		collator := newCollator(s.params, s.topology, s.pool, s.logger)
		return collator.GenerateZeroState(ctx, s.txFabric, s.ZeroState)
	}
	return nil
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	collator := newCollator(s.params, s.topology, s.pool, s.logger)
	return collator.GenerateBlock(ctx, s.txFabric)
}
