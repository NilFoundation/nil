package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

const (
	// batchesTable stores blocks batches produced by the Sync Committee.
	// Key: scTypes.BatchId, Value: batchEntry.
	batchesTable db.TableName = "batches"

	// batchParentIdxTable is used for indexing batches by their parent ids.
	// Key: scTypes.BatchId (batch's parent id), Value: scTypes.BatchId (batch's own id);
	batchParentIdxTable db.TableName = "blocks_parent_hash_idx"
)

// batchOp represents the set of operations related to batches within the storage.
type batchOp struct {
	logger zerolog.Logger
}

func newBatchOp(logger zerolog.Logger) batchOp {
	return batchOp{
		logger: logger,
	}
}

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

func (t batchOp) getBatch(tx db.RoTx, id scTypes.BatchId) (*batchEntry, error) {
	return t.getBatchBytesId(tx, id.Bytes(), true)
}

func (batchOp) getBatchBytesId(tx db.RoTx, idBytes []byte, required bool) (*batchEntry, error) {
	value, err := tx.Get(batchesTable, idBytes)

	switch {
	case err == nil:
		break

	case errors.Is(err, context.Canceled):
		return nil, err

	case errors.Is(err, db.ErrKeyNotFound) && required:
		return nil, fmt.Errorf("%w, id=%s", scTypes.ErrBatchNotFound, hex.EncodeToString(idBytes))

	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil

	default:
		return nil, fmt.Errorf("failed to get batch with id=%s: %w", hex.EncodeToString(idBytes), err)
	}

	entry, err := unmarshallEntry[batchEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// getBatchesSequence iterates through a chain of batches, starting from the batch with the given id.
// It uses batchParentIdxTable to retrieve parent-child connections between batches.
func (t batchOp) getBatchesSequence(tx db.RoTx, startingId scTypes.BatchId) iter.Seq2[*batchEntry, error] {
	return func(yield func(*batchEntry, error) bool) {
		startBatch, err := t.getBatch(tx, startingId)
		if err != nil {
			yield(nil, err)
			return
		}

		if !yield(startBatch, nil) {
			return
		}

		seenParentIds := make(map[scTypes.BatchId]bool)
		nextParentId := startBatch.Id
		for {
			if seenParentIds[nextParentId] {
				yield(nil, fmt.Errorf("cycle detected in the batch chain, parentId=%s", nextParentId))
				return
			}
			seenParentIds[nextParentId] = true

			nextIdBytes, err := tx.Get(batchParentIdxTable, nextParentId.Bytes())
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				yield(nil, fmt.Errorf("failed to get parent batch idx entry, parentId=%s: %w", nextParentId, err))
				return
			}
			if nextIdBytes == nil {
				break
			}
			nextBatchEntry, err := t.getBatchBytesId(tx, nextIdBytes, true)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(nextBatchEntry, nil) {
				return
			}
			nextParentId = nextBatchEntry.Id
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

func (batchOp) putBatchParentIndexEntry(tx db.RwTx, batch *scTypes.BlockBatch) error {
	if batch.ParentId == nil {
		return nil
	}

	err := tx.Put(batchParentIdxTable, batch.ParentId.Bytes(), batch.Id.Bytes())
	if err != nil {
		return fmt.Errorf(
			"failed to put parent batch idx entry, batchId=%s, parentId=%s,: %w", batch.Id, batch.ParentId, err,
		)
	}

	return nil
}

func (t batchOp) deleteBatch(tx db.RwTx, batch *batchEntry) error {
	if err := tx.Delete(batchesTable, batch.Id.Bytes()); err != nil {
		return fmt.Errorf("failed to delete batch with id=%s: %w", batch.Id, err)
	}

	if err := t.deleteBatchParentIndexEntry(tx, batch); err != nil {
		return err
	}

	return nil
}

func (t batchOp) deleteBatchParentIndexEntry(tx db.RwTx, batch *batchEntry) error {
	if batch.ParentId == nil {
		return nil
	}

	err := tx.Delete(batchParentIdxTable, batch.ParentId.Bytes())

	switch {
	case err == nil:
		return nil

	case errors.Is(err, context.Canceled):
		return err

	case errors.Is(err, db.ErrKeyNotFound):
		t.logger.Warn().Err(err).
			Stringer(logging.FieldBatchId, batch.Id).
			Stringer("parentBatchId", batch.ParentId).
			Msg("parent batch idx entry is not found")
		return nil

	default:
		return fmt.Errorf("failed to delete parent batch idx entry, parentId=%s: %w", batch.ParentId, err)
	}
}
