package storage

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// batchEventsTable stores batch events.
	// Key: types.BatchEventId, Value: types.BatchEvent.
	batchEventsTable db.TableName = "batches"
)

// batchEventOp represents the set of operations related to batch events within the storage.
type batchEventOp struct{}

func (batchEventOp) putEvent(tx db.RwTx, event *types.BatchEvent) error {
	value, err := marshallEntry(event)
	if err != nil {
		return fmt.Errorf("%w, id=%s", err, event.Id)
	}

	key := event.Id.Bytes()

	if err := tx.Put(batchEventsTable, key, value); err != nil {
		return fmt.Errorf("failed to put batch event, batchId=%s, eventId=%s: %w", event.BatchId, event.Id, err)
	}

	return nil
}

func (batchEventOp) deleteEvent(tx db.RwTx, eventId types.BatchEventId) error {
	key := eventId.Bytes()
	err := tx.Delete(batchEventsTable, key)

	switch {
	case err == nil || errors.Is(err, db.ErrKeyNotFound):
		return nil
	case errors.Is(err, context.Canceled):
		return err
	default:
		return fmt.Errorf("failed to delete batch event, eventId=%s: %w", eventId, err)
	}
}

// getStoredEventsSeq returns a sequence of stored events in an arbitrary order.
func (batchEventOp) getStoredEventsSeq(tx db.RoTx) iter.Seq2[*types.BatchEvent, error] {
	return tableIter[types.BatchEvent](tx, batchEventsTable)
}
