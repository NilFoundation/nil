package collate

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rollup"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

type TxnPool interface {
	Peek(n int) ([]*types.TxnWithHash, error)
	Discard(ctx context.Context, txns []common.Hash, reason txnpool.DiscardReason) error
	OnCommitted(ctx context.Context, baseFee types.Value, committed []*types.Transaction) error
}

type Consensus interface {
	RunSequence(ctx context.Context, height uint64) error
}

type Params struct {
	execution.BlockGeneratorParams

	MaxGasInBlock                 types.Gas
	MaxForwardTransactionsInBlock int

	CollatorTickPeriod time.Duration
	Timeout            time.Duration

	Topology ShardTopology

	L1Fetcher rollup.L1BlockFetcher
}

type Scheduler struct {
	consensus      Consensus
	txFabric       db.DB
	validator      *Validator
	networkManager network.Manager

	params *Params

	logger logging.Logger

	l1Fetcher rollup.L1BlockFetcher
}

func NewScheduler(
	validator *Validator,
	txFabric db.DB,
	consensus Consensus,
	networkManager network.Manager,
) *Scheduler {
	params := validator.params
	return &Scheduler{
		txFabric:       txFabric,
		validator:      validator,
		networkManager: networkManager,
		params:         params,
		logger: logging.NewLogger("collator").With().
			Stringer(logging.FieldShardId, params.ShardId).
			Logger(),
		l1Fetcher: params.L1Fetcher,
		consensus: consensus,
	}
}

func (s *Scheduler) Validator() *Validator {
	return s.validator
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.Info().Msg("Starting collation...")

	tickPeriodMs := s.params.CollatorTickPeriod.Milliseconds()
	for {
		var toRoundStartMs int64
		elapsed := time.Now().UnixMilli() % tickPeriodMs
		if elapsed > 0 {
			toRoundStartMs = tickPeriodMs - elapsed
		}

		select {
		case <-ctx.Done():
			s.logger.Info().Msg("Stopping collation...")
			return nil
		case <-time.After(time.Duration(toRoundStartMs) * time.Millisecond):
			if err := s.doCollate(ctx); err != nil {
				if ctx.Err() != nil {
					continue
				}
				s.logger.Error().Err(err).Msg("Failed to collate")
			}
		}
	}
}

func (s *Scheduler) doCollate(ctx context.Context) error {
	if s.params.DisableConsensus {
		proposal, err := s.validator.BuildProposal(ctx)
		if err != nil {
			return err
		}

		return s.validator.InsertProposal(ctx, proposal, &types.ConsensusParams{})
	}

	block, _, err := s.validator.GetLastBlock(ctx)
	if err != nil {
		return err
	}

	subId, syncCh := s.validator.Subscribe()
	defer s.validator.Unsubscribe(subId)

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	height := block.Id.Uint64() + 1
	consCh := make(chan error, 1)
	go func() {
		consCh <- s.consensus.RunSequence(ctx, height)
	}()

	for {
		select {
		case event := <-syncCh:
			if event.evType != replayType || event.blockNumber < types.BlockNumber(height) {
				continue
			}

			// We receive new block via syncer.
			// We need to interrupt current sequence and start new one.
			cancelFn()
			err := <-consCh
			s.logger.Debug().
				Uint64(logging.FieldHeight, height).
				Uint64(logging.FieldBlockNumber, uint64(event.blockNumber)).
				Err(err).
				Msg("Consensus interrupted by syncer")
			return nil
		case err := <-consCh:
			return err
		}
	}
}
