package l1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/storage"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/jonboulle/clockwork"
)

const (
	// pendingEventsTable stores events received from L1BridgeMessenger waiting to be finalized
	// Key: Hash of the Event
	pendingEventsTable = "pending_events"

	// monotonic counter managing ordering between received events
	pendingEventsSequencer = "pending_events_sequencer"

	// lastProcessedBlockTable stores number (and some meta-info)
	// for last block events from which were successfully stored to the local database (single value)
	// Key: lastProcessedBlockKey
	lastProcessedBlockTable = "last_processed_block"
	lastProcessedBlockKey   = "last_processed_block_key"
)

type EventStorageMetrics interface {
	// TODO(oclaw)
}

type EventStorage struct {
	*storage.BaseStorage
	metrics         EventStorageMetrics
	eventsSequencer db.Sequence
}

func NewEventStorage(
	ctx context.Context,
	database db.DB,
	clock clockwork.Clock,
	metrics EventStorageMetrics,
	logger logging.Logger,
) (*EventStorage, error) {
	es := &EventStorage{
		BaseStorage: storage.NewBaseStorage(ctx, database, clock, logger),
		metrics:     metrics,
	}
	var err error
	es.eventsSequencer, err = database.GetSequence(ctx, []byte(pendingEventsSequencer), 100)
	if err != nil {
		return nil, err
	}

	return es, nil
}

func (es *EventStorage) StoreEvent(ctx context.Context, evt *Event) error {
	var emptyHash ethcommon.Hash
	if evt.Hash == emptyHash {
		return errors.New("cannot store event without hash")
	}

	return es.RetryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		evt.SequenceNumber, err = es.eventsSequencer.Next()
		if err != nil {
			return err
		}

		writer := storage.NewJSONWriter[*Event](pendingEventsTable, es.BaseStorage, false)
		return writer.PutTx(ctx, evt.Hash.Bytes(), evt)

		// TODO (oclaw) metrics
	})
}

func (es *EventStorage) IterateEventsByBatch(
	ctx context.Context,
	batchSize int,
	callback func([]*Event) error,
) error {
	return es.RetryRunner.Do(ctx, func(ctx context.Context) error {
		tx, err := es.Database.CreateRoTx(ctx)
		if err != nil {
			return err
		}

		iter, err := tx.Range(pendingEventsTable, nil, nil)
		if err != nil {
			return err
		}

		batch := make([]*Event, batchSize)
		idx := 0
		for iter.HasNext() {
			_, val, err := iter.Next()
			if err != nil {
				return err
			}
			if err := json.Unmarshal(val, &batch[idx]); err != nil {
				return fmt.Errorf("%w: %w", storage.ErrSerializationFailed, err)
			}

			idx++
			if idx >= batchSize {
				if err := callback(batch); err != nil {
					return err
				}
				idx = 0
			}
		}
		if idx > 0 {
			return callback(batch[:idx])
		}

		return nil
	})
}

func (es *EventStorage) DeleteEvents(ctx context.Context, hashes []ethcommon.Hash) error {
	return es.RetryRunner.Do(ctx, func(ctx context.Context) error {
		tx, err := es.Database.CreateRwTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		for _, hash := range hashes {
			if err := tx.Delete(pendingEventsTable, hash.Bytes()); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}
		}

		return es.Commit(tx)
	})
}

func (es *EventStorage) GetLastProcessedBlock(ctx context.Context) (*ProcessedBlock, error) {
	var ret *ProcessedBlock
	err := es.RetryRunner.Do(ctx, func(ctx context.Context) error {
		tx, err := es.Database.CreateRoTx(ctx)
		if err != nil {
			return err
		}

		data, err := tx.Get(lastProcessedBlockTable, []byte(lastProcessedBlockKey))
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		var blk ProcessedBlock
		if err := json.Unmarshal(data, &blk); err != nil {
			return fmt.Errorf("%w: %w", storage.ErrSerializationFailed, err)
		}

		ret = &blk

		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (es *EventStorage) SetLastProcessedBlock(ctx context.Context, blk *ProcessedBlock) error {
	var emptyHash ethcommon.Hash
	if blk.BlockHash == emptyHash {
		return errors.New("empty last processed block hash")
	}
	if blk.BlockNumber == 0 {
		return errors.New("empty last processed block number")
	}

	return es.RetryRunner.Do(ctx, func(ctx context.Context) error {
		writer := storage.NewJSONWriter[*ProcessedBlock](lastProcessedBlockTable, es.BaseStorage, true)
		return writer.PutTx(ctx, []byte(lastProcessedBlockKey), blk)

		// TODO(oclaw) metrics
	})
}
