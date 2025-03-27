package l1

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/big"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common/heap"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/storage"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

type FinalityEnsurerConfig struct {
	EthPollInterval      time.Duration
	DbPollInterval       time.Duration
	EventBufferSize      int
	BlockCacheSize       int
	EventEmitterCapacity int
}

func (cfg *FinalityEnsurerConfig) Validate() error {
	if cfg.EthPollInterval == 0 {
		return errors.New("zero L1 poll interval")
	}
	if cfg.DbPollInterval == 0 {
		return errors.New("zero storage poll interval")
	}
	if cfg.EventBufferSize == 0 {
		return errors.New("event buffer size is not set")
	}
	if cfg.BlockCacheSize == 0 {
		return errors.New("block cache size is not set")
	}
	return nil
}

func DefaultFinalityEnsurerConfig() *FinalityEnsurerConfig {
	return &FinalityEnsurerConfig{
		EthPollInterval:      5 * time.Second,
		DbPollInterval:       10 * time.Second,
		EventBufferSize:      1000,
		BlockCacheSize:       100,
		EventEmitterCapacity: 0, // recommended for production usage
	}
}

type eventProvider interface {
	EventReceived() <-chan struct{}
}

type FinalityEnsurer struct {
	ethClient EthClient

	finalizedBlock     *ProcessedBlock
	finalizedBlockLock sync.RWMutex

	// cache for fetched finalized block number
	// it must contain only blocks with number <= finalizedBlock.BlockNumber
	finBlockCache *lru.Cache[uint64, *ProcessedBlock]

	config        *FinalityEnsurerConfig
	logger        logging.Logger
	clock         clockwork.Clock
	l1Storage     *EventStorage
	l2Storage     *l2.EventStorage
	metrics       FinalityEnsurerMetrics
	eventProvider eventProvider

	emitter chan struct{}
}

func NewFinalityEnsurer(
	config *FinalityEnsurerConfig,
	ethClient EthClient,
	clock clockwork.Clock,
	logger logging.Logger,
	l1Storage *EventStorage,
	l2Storage *l2.EventStorage,
	metrics FinalityEnsurerMetrics,
	eventProvider eventProvider,
) (*FinalityEnsurer, error) {
	err := config.Validate()
	if err != nil {
		return nil, err
	}

	fe := &FinalityEnsurer{
		ethClient:     ethClient,
		config:        config,
		logger:        logger,
		clock:         clock,
		l1Storage:     l1Storage,
		l2Storage:     l2Storage,
		eventProvider: eventProvider,
		metrics:       metrics,
		emitter:       make(chan struct{}, config.EventEmitterCapacity),
	}

	fe.finBlockCache, err = lru.New[uint64, *ProcessedBlock](config.BlockCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize block cache: %w", err)
	}

	fe.logger = logger.With().Str(logging.FieldComponent, fe.Name()).Logger()
	return fe, nil
}

func (fe *FinalityEnsurer) Name() string {
	return "block-finality-ensurer"
}

func (fe *FinalityEnsurer) EventFinalized() <-chan struct{} {
	return fe.emitter
}

func (fe *FinalityEnsurer) Run(ctx context.Context, started chan<- struct{}) error {
	eg, gCtx := errgroup.WithContext(ctx)

	fe.logger.Info().Msg("initializing component")

	eg.Go(func() error {
		return fe.blockFetcher(gCtx)
	})

	eg.Go(func() error {
		return fe.pendingEventPoller(gCtx)
	})

	close(started)

	err := eg.Wait()
	fe.logger.Debug().Err(err).Msg("end processing")
	return nil
}

func (fe *FinalityEnsurer) blockFetcher(ctx context.Context) error {
	fe.logger.Info().Msg("started finalized block fetcher")

	ticker := fe.clock.NewTicker(fe.config.EthPollInterval)

	var lastSuccessfulUpdate time.Time
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.Chan():
			header, err := fe.ethClient.HeaderByNumber(ctx, big.NewInt(rpc.FinalizedBlockNumber.Int64()))
			now := fe.clock.Now()
			if err != nil {
				log := fe.logger.Error().Err(err)
				if !lastSuccessfulUpdate.IsZero() {
					diff := now.Sub(lastSuccessfulUpdate)
					log = log.Int("time_since_last_update_sec", int(diff.Seconds()))
				}
				if fe.finalizedBlock != nil {
					log = log.Uint64("local_finalized_block_number", fe.finalizedBlock.BlockNumber)
				}

				fe.metrics.SetTimeSinceFinalizedBlockNumberUpdate(
					ctx,
					uint64(now.Sub(lastSuccessfulUpdate).Seconds()),
				)

				log.Msg("failed to fetch last finalized block number from Etherium")
				continue
			}

			if fe.finalizedBlock == nil ||
				fe.finalizedBlock.BlockNumber != header.Number.Uint64() {
				fe.finalizedBlockLock.Lock()
				fe.finalizedBlock = &ProcessedBlock{
					BlockNumber: header.Number.Uint64(),
					BlockHash:   header.Hash(),
				}
				fe.finalizedBlockLock.Unlock()
			}

			fe.logger.Info().
				Uint64("local_finalized_block_number", header.Number.Uint64()).
				Msg("refreshed actual finalized block number")
			lastSuccessfulUpdate = now

			fe.metrics.SetTimeSinceFinalizedBlockNumberUpdate(ctx, 0)
		}
	}
}

func (fe *FinalityEnsurer) pendingEventPoller(ctx context.Context) error {
	fe.logger.Info().Msg("started l1 pending event processor")

	ticker := fe.clock.NewTicker(fe.config.DbPollInterval)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.Chan():
			fe.logger.Debug().Msg("wake up by timer")
		case <-fe.eventProvider.EventReceived():
			fe.logger.Debug().Msg("wake up by event emitter")
		}
		if err := fe.forwardFinalizedEvents(ctx); err != nil {
			fe.logger.Error().Err(err).Msg("failed to process l1 pending events")
			fe.metrics.AddRelayError(ctx)
		}
	}
}

func (fe *FinalityEnsurer) forwardFinalizedEvents(ctx context.Context) error {
	finalizedBlock, hasFinalizedBlock := fe.getLatestFinalizedBlock()
	if !hasFinalizedBlock {
		fe.logger.Debug().Msg("no finalized block number received from L1")
		return nil
	}

	// limited size storage to fetch events with min sequence number
	eventBySeqNo := heap.NewBoundedMaxHeap(fe.config.EventBufferSize, func(a, b *Event) int {
		return cmp.Compare(a.SequenceNumber, b.SequenceNumber)
	})

	checkedEvents := 0
	if err := fe.l1Storage.IterateEventsByBatch(ctx, 100, func(eventBatch []*Event) error {
		checkedEvents += len(eventBatch)
		for _, evt := range eventBatch {
			if evt.BlockNumber <= finalizedBlock.BlockNumber {
				eventBySeqNo.Add(evt)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	events := eventBySeqNo.PopAllSorted()

	fe.logger.Info().
		Int("total_events_in_storage", checkedEvents).
		Int("events_to_check", len(events)).
		Msg("scanned pending L1 events")

	if len(events) == 0 {
		fe.logger.Debug().Msg("no events to forward")
		return nil
	}

	eventByBlock := make(map[ProcessedBlock][]*Event)
	for _, evt := range events {
		blkInfo := ProcessedBlock{
			BlockNumber: evt.BlockNumber,
			BlockHash:   evt.BlockHash,
		}
		eventByBlock[blkInfo] = append(eventByBlock[blkInfo], evt)
	}

	finalized, orphaned, err := fe.checkBlocksFinality(ctx, &finalizedBlock, maps.Keys(eventByBlock))
	if err != nil {
		return fmt.Errorf("failed to check blocks finality: %w", err)
	}

	fe.logger.Info().
		Int("finalized_blocks_count", len(finalized)).
		Int("orphaned_blocks_count", len(orphaned)).
		Msg("checked blocks finality")

	var (
		finalizedEventCount int
		orphanedEventCount  = len(events)
	)

	if len(finalized) > 0 {
		var l2Events []*l2.Event
		for _, finblk := range finalized {
			for _, finEvt := range eventByBlock[finblk] {
				l2Events = append(l2Events, fe.convertEvent(finEvt))
			}
		}

		fe.logger.Info().
			Int("finalized_event_count", len(l2Events)).
			Msg("saving messages to L2 event storage")

		finalizedEventCount = len(l2Events)
		orphanedEventCount = len(events) - finalizedEventCount

		err := fe.l2Storage.StoreEvents(ctx, l2Events)
		if ignoreErrors(err, storage.ErrKeyExists) != nil {
			return fmt.Errorf("failed to forward events to L2 storage: %w", err)
		}

		// non-blocking notifier to let received know that it is time to fetch data
		select {
		case fe.emitter <- struct{}{}:
		default:
		}
	}

	fe.metrics.AddFinalizedEvents(ctx, uint64(finalizedEventCount))
	fe.metrics.AddOrphanedEvents(ctx, uint64(orphanedEventCount))

	fe.logger.Info().
		Int("dropping_events_count", len(events)).
		Msg("dropping pending events from L1 storage")

	droppingEvents := make([]ethcommon.Hash, 0, len(events))
	for _, evt := range events {
		droppingEvents = append(droppingEvents, evt.Hash)
	}

	if err := fe.l1Storage.DeleteEvents(ctx, droppingEvents); err != nil {
		return fmt.Errorf("failed to cleanup events from l1 storage: %w", err)
	}

	return nil
}

func (fe *FinalityEnsurer) checkBlocksFinality(
	ctx context.Context,
	base *ProcessedBlock,
	toCheck iter.Seq[ProcessedBlock],
) ([]ProcessedBlock, []ProcessedBlock, error) {
	var finalized, orphaned []ProcessedBlock

	for blk := range toCheck {
		if base.BlockNumber < blk.BlockNumber {
			fe.logger.Fatal().
				Uint64("latest_block_num", base.BlockNumber).
				Uint64("check_block_num", blk.BlockNumber).
				Msg("attempt to relay non-finalized event!")
		}

		finalizedBlock, err := fe.getBlockHeader(ctx, blk.BlockNumber)
		if err != nil {
			return nil, nil, err
		}

		if finalizedBlock.BlockHash != blk.BlockHash {
			fe.logger.Debug().
				Uint64("orphaned_block_num", blk.BlockNumber).
				Msg("found orphaned block")

			orphaned = append(orphaned, blk)
		} else {
			finalized = append(finalized, blk)
		}
	}

	return finalized, orphaned, nil
}

func (fe *FinalityEnsurer) getBlockHeader(ctx context.Context, blkNum uint64) (*ProcessedBlock, error) {
	// fast path
	if blk, ok := fe.finBlockCache.Get(blkNum); ok {
		return blk, nil
	}

	blkHdr, err := fe.ethClient.HeaderByNumber(ctx, big.NewInt(int64(blkNum)))
	if err != nil {
		return nil, err
	}

	ret := &ProcessedBlock{
		BlockNumber: blkHdr.Number.Uint64(),
		BlockHash:   blkHdr.Hash(),
	}
	fe.finBlockCache.Add(blkHdr.Number.Uint64(), ret)

	return ret, nil
}

func (fe *FinalityEnsurer) getLatestFinalizedBlock() (ProcessedBlock, bool) {
	fe.finalizedBlockLock.RLock()
	defer fe.finalizedBlockLock.RUnlock()
	if fe.finalizedBlock == nil {
		return ProcessedBlock{}, false
	}
	return *fe.finalizedBlock, true
}

func (fe *FinalityEnsurer) convertEvent(in *Event) (out *l2.Event) {
	return &l2.Event{
		BlockNumber:    in.BlockNumber,
		Hash:           in.Hash,
		SequenceNumber: in.SequenceNumber,
		FeePack: types.FeePack{
			FeeCredit:            types.NewValueFromBigMust(in.FeeCreditData.FeeCredit),
			MaxFeePerGas:         types.NewValueFromBigMust(in.FeeCreditData.MaxFeePerGas),
			MaxPriorityFeePerGas: types.NewValueFromBigMust(in.FeeCreditData.MaxPriorityFeePerGas),
		},
		L2Limit: types.NewValueFromBigMust(in.FeeCreditData.NilGasLimit),
		Sender:  in.Sender,
		Target:  in.Target,
		Value:   in.Value,
		Nonce:   in.Nonce,
		Type:    in.Type,
		Message: in.Message,
	}
}
