package l2

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/nil/common/heap"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jonboulle/clockwork"
)

type TransactionSenderConfig struct {
	DbPollInterval  time.Duration
	EventBufferSize int
}

func (cfg *TransactionSenderConfig) Validate() error {
	if cfg.DbPollInterval == 0 {
		return errors.New("no storage poll interval set")
	}
	if cfg.EventBufferSize == 0 {
		return errors.New("no event buffer size for the poll heap is set")
	}
	return nil
}

func DefaultTransactionSenderConfig() *TransactionSenderConfig {
	return &TransactionSenderConfig{
		DbPollInterval:  time.Second * 10,
		EventBufferSize: 500,
	}
}

type eventFinalizedProvider interface {
	EventFinalized() <-chan struct{}
}

type TransactionSender struct {
	config           *TransactionSenderConfig
	clock            clockwork.Clock
	logger           logging.Logger
	storage          *EventStorage
	eventFinProvider eventFinalizedProvider
	metrics          TransactionSenderMetrics
	contractBinding  L2Contract
}

func NewTransactionSender(
	config *TransactionSenderConfig,
	storage *EventStorage,
	logger logging.Logger,
	clock clockwork.Clock,
	eventFinProvider eventFinalizedProvider,
	metrics TransactionSenderMetrics,
	contractBinding L2Contract,
) (*TransactionSender, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	ts := &TransactionSender{
		config:           config,
		clock:            clock,
		storage:          storage,
		eventFinProvider: eventFinProvider,
		metrics:          metrics,
		contractBinding:  contractBinding,
	}
	ts.logger = logger.With().Str(logging.FieldComponent, ts.Name()).Logger()
	return ts, nil
}

func (ts *TransactionSender) Name() string {
	return "transaction-sender"
}

func (ts *TransactionSender) Run(ctx context.Context, started chan<- struct{}) error {
	ts.logger.Info().Msg("initializing component")

	ticker := ts.clock.NewTicker(ts.config.DbPollInterval)
	close(started)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.Chan():
			ts.logger.Debug().Msg("wake up by timer")
		case <-ts.eventFinProvider.EventFinalized():
			ts.logger.Debug().Msg("wake up by event emitter")
		}
		if err := ts.relayEvents(ctx); err != nil {
			ts.logger.Error().Err(err).Msg("error occurred during relaying events to L2")
			ts.metrics.AddRelayError(ctx)
		}
	}
}

func (ts *TransactionSender) relayEvents(ctx context.Context) error {
	eventBySeqNumber := heap.NewBoundedMaxHeap(
		ts.config.EventBufferSize,
		func(a, b *Event) int {
			return cmp.Compare(a.SequenceNumber, b.SequenceNumber)
		},
	)

	eventsIterated := 0
	if err := ts.storage.IterateEventsByBatch(ctx, 100, func(batch []*Event) error {
		for _, evt := range batch {
			eventBySeqNumber.Add(evt)
		}
		eventsIterated += len(batch)
		return nil
	}); err != nil {
		return err
	}

	events := eventBySeqNumber.PopAllSorted()

	if len(events) == 0 {
		ts.logger.Debug().Msg("no ready events to be relayed to L2")
		return nil
	}

	ts.logger.Info().
		Int("fetched_events_count", len(events)).
		Int("checked_events_count", eventsIterated).
		Msg("fetched some events ready to be relayed to L2")

	droppingEvents := make([]common.Hash, 0, len(events))

	defer func() {
		if len(droppingEvents) == 0 {
			return
		}
		ts.logger.Debug().
			Int("event_count", len(droppingEvents)).
			Msg("dropping events from L2 storage")

		if err := ts.storage.DeleteEvents(ctx, droppingEvents); err != nil {
			ts.logger.Warn().Err(err).Msg("failed to drop events from L2 storage")
		}
	}()

	for i, evt := range events {
		if _, err := ts.contractBinding.RelayMessage(ctx, evt); err != nil {
			ts.logger.Error().Err(err).
				Int("event_index", i).
				Uint64("event_seqno", evt.SequenceNumber).
				Stringer("event_hash", evt.Hash).
				Msg("failed to relay event to L2")

			return err
		}
		ts.logger.Debug().Stringer("event_hash", evt.Hash).Msg("event relayed to L2")
		droppingEvents = append(droppingEvents, evt.Hash)

		ts.metrics.AddRelayedEvents(ctx, 1)
	}

	return nil
}
