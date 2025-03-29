package storage

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/NilFoundation/nil/nil/internal/db"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// batchesTable stores blocks batches produced by the Sync Committee.
	// Key: scTypes.BatchId, Value: batchEntry.
	batchesTable db.TableName = "batches"
)

// batchOp represents the set of operations related to batches within the storage.
type batchOp struct{}

func (batchOp) putBatch(tx db.RwTx, entry *batchEntry) error {
	value, err := marshallEntry(entry)
	if err != nil {
		return fmt.Errorf("%w, id=%s", err, entry.Id)
	}

	if err := tx.Put(batchesTable, entry.Id.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put batch with id=%s: %w", entry.Id, err)
	}

	return nil
}

func (batchOp) getBatch(tx db.RoTx, id scTypes.BatchId) (*batchEntry, error) {
	idBytes := id.Bytes()
	value, err := tx.Get(batchesTable, idBytes)

	switch {
	case err == nil:
		break

	case errors.Is(err, context.Canceled):
		return nil, err

	case errors.Is(err, db.ErrKeyNotFound):
		return nil, fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotFound, id)

	default:
		return nil, fmt.Errorf("failed to get batch with id=%s: %w", id, err)
	}

	entry, err := unmarshallEntry[batchEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// getBatchesSeqReversed iterates through a chain of batches between two ids (boundaries included) in reverse order.
// Batch `from` is expected to be a descendant of the batch `to`.
func (t batchOp) getBatchesSeqReversed(
	tx db.RoTx, from scTypes.BatchId, to scTypes.BatchId,
) iter.Seq2[*batchEntry, error] {
	return func(yield func(*batchEntry, error) bool) {
		startBatch, err := t.getBatch(tx, from)
		if err != nil {
			yield(nil, err)
			return
		}

		if !yield(startBatch, nil) || from == to {
			return
		}

		seenBatches := make(map[scTypes.BatchId]bool)
		nextBatchId := startBatch.ParentId
		for {
			if nextBatchId == nil {
				yield(nil, fmt.Errorf("unable to restore batch sequence [%s, %s]", from, to))
				return
			}

			if seenBatches[*nextBatchId] {
				yield(nil, fmt.Errorf("cycle detected in the batch chain, parentId=%s", nextBatchId))
				return
			}
			seenBatches[*nextBatchId] = true

			nextBatchEntry, err := t.getBatch(tx, *nextBatchId)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(nextBatchEntry, nil) || nextBatchEntry.Id == to {
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

func (t batchOp) deleteBatch(tx db.RwTx, batch *batchEntry) error {
	if err := tx.Delete(batchesTable, batch.Id.Bytes()); err != nil {
		return fmt.Errorf("failed to delete batch with id=%s: %w", batch.Id, err)
	}

	return nil
}
