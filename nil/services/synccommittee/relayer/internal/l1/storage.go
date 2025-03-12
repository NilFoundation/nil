package l1

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/rs/zerolog"
)

const (
	// pendingEventsTable stores events received from L1BridgeMessenger waiting to be finalized
	// Key: Hash of the Event
	pendingEventsTable = "pending_events"

	// monotonic counter managing ordering between received events
	pendingEventsSequencer = "pending_events_sequencer"

	// lastProcessedBlockTable stores number (and some meta-info) for last block events from which were successfully stored to the local database (single value)
	// Key: lastProcessedBlockKey
	lastProcessedBlockTable = "last_processed_block"
	lastProcessedBlockKey   = "last_processed_block_key"
)

type EventStorageMetrics interface {
	// TODO(oclaw)
}

type EventStorage struct {
	storage.CommonStorage
	clock           common.Timer
	metrics         EventStorageMetrics
	eventsSequencer db.Sequence
	// TODO store events from L1
	// TODO store last processed block number
	// TODO use sequencer for event ordering
}

func NewEventStorage(
	ctx context.Context,
	database db.DB,
	clock common.Timer,
	metrics EventStorageMetrics,
	logger zerolog.Logger,
) (*EventStorage, error) {
	es := &EventStorage{
		CommonStorage: storage.NewCommonStorage(
			database, logger,
			// TODO (oclaw) add retry policies ?
		),
		clock:   clock,
		metrics: metrics,
	}
	var err error
	es.eventsSequencer, err = database.GetSequence(ctx, []byte(pendingEventsSequencer), 100)
	if err != nil {
		return nil, err
	}

	return es, nil
}

func (es *EventStorage) StoreEvent(ctx context.Context, evt *Event) error {
	// TODO(oclaw) check if hash is not empty

	var err error
	evt.SequenceNumber, err = es.eventsSequencer.Next()
	if err != nil {
		return err
	}

	writer := jsonDbWriter[*Event]{
		table:   pendingEventsTable,
		storage: es,
	}
	return writer.putTx(ctx, evt.Hash.Bytes(), evt)

	// TODO (oclaw) metrics
}

func (es *EventStorage) GetEvents(ctx context.Context) ([]*Event, error) {
	return nil, errors.New("not implemented")
}

func (es *EventStorage) DeleteEvents(ctx context.Context, hashes []common.Hash) error {
	return errors.New("not implemented")
}

func (es *EventStorage) GetLastProcessedBlock(ctx context.Context) (*ProcessedBlock, error) {
	tx, err := es.Database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}

	data, err := tx.Get(lastProcessedBlockTable, []byte(lastProcessedBlockKey))
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var blk ProcessedBlock
	if err := json.Unmarshal(data, &blk); err != nil {
		return nil, err
	}

	return &blk, nil
}

func (es *EventStorage) SetLastProcessedBlock(ctx context.Context, blk *ProcessedBlock) error {
	// TODO(oclaw) check if hash is not empty

	writer := jsonDbWriter[*ProcessedBlock]{
		table:   lastProcessedBlockTable,
		storage: es,
	}
	return writer.putTx(ctx, []byte(lastProcessedBlockKey), blk)

	// TODO(oclaw) metrics
}

type jsonDbWriter[T any] struct {
	table   string
	storage *EventStorage
}

func (jdwr *jsonDbWriter[T]) putTx(ctx context.Context, key []byte, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	tx, err := jdwr.storage.Database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := tx.Put(db.TableName(jdwr.table), key, data); err != nil {
		return err
	}

	return jdwr.storage.Commit(tx)
}
