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
	// batchesTable stores blocks batches produced by the Sync Committee.
	// Key: types.BatchId, Value: batchEntry.
	batchesTable db.TableName = "batches"
)

// batchOp represents the set of operations related to batches within the storage.
type batchOp struct{}

func (batchOp) putBatchEntry(tx db.RwTx, entry *batchEntry) error {
	value, err := marshallEntry(entry)
	if err != nil {
		return fmt.Errorf("%w, id=%s", err, entry.Id)
	}

	if err := tx.Put(batchesTable, entry.Id.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put batch with id=%s: %w", entry.Id, err)
	}

	return nil
}

func (batchOp) batchExists(tx db.RoTx, batchId types.BatchId) (bool, error) {
	key := batchId.Bytes()
	exists, err := tx.Exists(batchesTable, key)
	if err != nil {
		return false, fmt.Errorf("failed to check if batch with id=%s exists: %w", batchId, err)
	}
	return exists, nil
}

func (o batchOp) getBatchEntry(tx db.RoTx, id types.BatchId) (*batchEntry, error) {
	idBytes := id.Bytes()
	value, err := tx.Get(batchesTable, idBytes)

	switch {
	case err == nil:

	case errors.Is(err, context.Canceled):
		return nil, err

	case errors.Is(err, db.ErrKeyNotFound):
		return nil, o.errBatchNotFound(id)

	default:
		return nil, fmt.Errorf("failed to get batch with id=%s: %w", id, err)
	}

	entry, err := unmarshallEntry[batchEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (o batchOp) errBatchNotFound(id types.BatchId) error {
	return fmt.Errorf("%w, id=%s", types.ErrBatchNotFound, id)
}

// getBatchesSeqReversed iterates through a chain of batches between two ids (boundaries included) in reverse order.
// Batch `from` is expected to be a descendant of the batch `to`.
//
// When `to` is `nil`, batches are traversed down to the first created batch.
func (o batchOp) getBatchesSeqReversed(
	tx db.RoTx, from types.BatchId, to *types.BatchId,
) iter.Seq2[*batchEntry, error] {
	return func(yield func(*batchEntry, error) bool) {
		startBatch, err := o.getBatchEntry(tx, from)
		if err != nil {
			yield(nil, err)
			return
		}
		if to != nil {
			exists, err := o.batchExists(tx, *to)
			if err != nil {
				yield(nil, err)
				return
			}
			if !exists {
				yield(nil, o.errBatchNotFound(*to))
			}
		}

		if !yield(startBatch, nil) || to != nil && from == *to {
			return
		}

		seenBatches := make(map[types.BatchId]bool)
		nextBatchId := startBatch.ParentId
		for {
			if nextBatchId == nil {
				if to != nil {
					yield(nil, fmt.Errorf("unable to restore batch sequence [%s, %s]", from, to))
				}
				return
			}

			if seenBatches[*nextBatchId] {
				yield(nil, fmt.Errorf("cycle detected in the batch chain, parentId=%s", nextBatchId))
				return
			}
			seenBatches[*nextBatchId] = true

			nextBatchExists, err := o.batchExists(tx, *nextBatchId)
			if err != nil {
				yield(nil, err)
				return
			}
			switch {
			case nextBatchExists:
				// Batch exists, continue traversing
			case to == nil:
				// We've reached the earliest batch in the storage, and its parent has already been removed
				return
			case to != nil:
				traverseErr := fmt.Errorf(
					"failed to traverse batch sequence from %s to %s: %w", from, to, o.errBatchNotFound(*nextBatchId),
				)
				yield(nil, traverseErr)
				return
			}

			nextBatchEntry, err := o.getBatchEntry(tx, *nextBatchId)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(nextBatchEntry, nil) || to != nil && nextBatchEntry.Id == *to {
				return
			}

			nextBatchId = nextBatchEntry.ParentId
		}
	}
}

// getStoredBatchesSeq returns a sequence of stored batches in an arbitrary order.
func (batchOp) getStoredBatchesSeq(tx db.RoTx) iter.Seq2[*batchEntry, error] {
	return func(yield func(*batchEntry, error) bool) {
		txIter, err := tx.Range(batchesTable, nil, nil)
		if err != nil {
			yield(nil, err)
			return
		}
		defer txIter.Close()

		for txIter.HasNext() {
			key, val, err := txIter.Next()
			if err != nil {
				yield(nil, err)
				return
			}
			entry, err := unmarshallEntry[batchEntry](key, val)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (o batchOp) deleteBatch(tx db.RwTx, batch *batchEntry) error {
	if err := tx.Delete(batchesTable, batch.Id.Bytes()); err != nil {
		return fmt.Errorf("failed to delete batch with id=%s: %w", batch.Id, err)
	}

	return nil
}
