package collate

import (
	"context"
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

type Scheduler struct {
	txFabric db.DB
	pool     MsgPool

	id       types.ShardId
	nShards  int
	topology ShardTopology

	ZeroState       string
	MainKeysOutPath string

	logger zerolog.Logger
}

func NewScheduler(txFabric db.DB, pool MsgPool, id types.ShardId, nShards int, topology ShardTopology) *Scheduler {
	return &Scheduler{
		txFabric: txFabric,
		pool:     pool,
		id:       id,
		nShards:  nShards,
		topology: topology,
		logger: logging.NewLogger("collator").With().
			Str(logging.FieldShardId, id.String()).
			Logger(),
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	sharedLogger.Info().Msg("Starting collation...")

	// At first generate zerostate if needed
	if err := s.generateZeroState(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(defaultPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.doCollate(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			sharedLogger.Info().Msg("Stopping collation...")
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

	lastBlockHash, err := db.ReadLastBlockHash(roTx, s.id)
	if err != nil {
		return err
	}
	if lastBlockHash == common.EmptyHash {
		if len(s.MainKeysOutPath) != 0 && s.id == types.MasterShardId {
			if err := execution.DumpMainKeys(s.MainKeysOutPath); err != nil {
				return err
			}
		}

		collator := newCollator(s.id, s.nShards, s.topology, s.pool, s.logger)
		return collator.GenerateZeroState(ctx, s.txFabric, s.ZeroState)
	}
	return nil
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	collator := newCollator(s.id, s.nShards, s.topology, s.pool, s.logger)
	return collator.GenerateBlock(ctx, s.txFabric)
}
