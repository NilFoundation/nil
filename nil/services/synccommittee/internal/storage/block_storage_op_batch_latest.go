package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// latestBatchIdTable stores identifier of the latest saved batch.
	// Key: mainShardKey, Value: scTypes.BatchId.
	latestBatchIdTable db.TableName = "latest_batch_id"
)

// batchLatestOp represents the set of operations to manage the latest batch ID state in the storage.
type batchLatestOp struct{}

func (t batchLatestOp) updateLatestBatchId(tx db.RwTx, batch *scTypes.BlockBatch) error {
	latestBatchId, err := t.getLatestBatchId(tx)
	if err != nil {
		return err
	}
	if err := t.validateLatestBatchId(batch, latestBatchId); err != nil {
		return err
	}

	return t.putLatestBatchId(tx, &batch.Id)
}

func (batchLatestOp) getLatestBatchId(tx db.RoTx) (*scTypes.BatchId, error) {
	bytes, err := tx.Get(latestBatchIdTable, mainShardKey)

	switch {
	case err == nil:
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	case errors.Is(err, context.Canceled):
		return nil, err
	default:
		return nil, fmt.Errorf("failed to get latest batch id: %w", err)
	}

	if bytes == nil {
		return nil, nil
	}

	var batchId scTypes.BatchId
	if err := batchId.UnmarshalText(bytes); err != nil {
		return nil, err
	}
	return &batchId, nil
}

func (batchLatestOp) validateLatestBatchId(batch *scTypes.BlockBatch, latestBatchId *scTypes.BatchId) error {
	var isValid bool
	switch {
	case latestBatchId == nil:
		isValid = batch.ParentId == nil
	case batch.ParentId == nil:
		isValid = false
	default:
		isValid = *latestBatchId == *batch.ParentId
	}

	if isValid {
		return nil
	}

	return fmt.Errorf(
		"%w: got batch with parentId=%s, latest batch id is %s",
		scTypes.ErrBatchMismatch, batch.ParentId, latestBatchId,
	)
}

func (batchLatestOp) putLatestBatchId(tx db.RwTx, batchId *scTypes.BatchId) error {
	var bytes []byte

	if batchId != nil {
		var err error
		bytes, err = batchId.MarshalText()
		if err != nil {
			return err
		}
	}

	err := tx.Put(latestBatchIdTable, mainShardKey, bytes)

	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return err
	default:
		return fmt.Errorf("failed to put latest batch id: %w", err)
	}
}
