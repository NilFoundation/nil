package storage

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
)

const (
	// storedBatchesCountTable stores the count of batches that have been persisted in the database.
	// Key: mainShardKey, Value: uint32.
	storedBatchesCountTable db.TableName = "stored_batches_count"
)

// batchCountOp represents the set of operations related to batch count in storage based on the given configuration.
type batchCountOp struct{}

func (t batchCountOp) addStoredCount(tx db.RwTx, delta int32, config BlockStorageConfig) error {
	currentBatchesCount, err := t.getBatchesCount(tx)
	if err != nil {
		return err
	}

	signed := int32(currentBatchesCount) + delta
	if signed < 0 {
		return fmt.Errorf(
			"batches count cannot be negative: delta=%d, current blocks count=%d", delta, currentBatchesCount,
		)
	}

	newBatchesCount := uint32(signed)
	if newBatchesCount > config.StoredBatchesLimit {
		return fmt.Errorf(
			"%w: delta is %d, current storage size is %d, capacity limit is %d",
			ErrCapacityLimitReached, delta, currentBatchesCount, config.StoredBatchesLimit,
		)
	}

	return t.putBatchesCount(tx, newBatchesCount)
}

func (batchCountOp) getBatchesCount(tx db.RoTx) (uint32, error) {
	bytes, err := tx.Get(storedBatchesCountTable, mainShardKey)
	switch {
	case err == nil:
	case errors.Is(err, db.ErrKeyNotFound):
		return 0, nil
	default:
		return 0, fmt.Errorf("failed to get batches count: %w", err)
	}

	count := binary.LittleEndian.Uint32(bytes)
	return count, nil
}

func (batchCountOp) putBatchesCount(tx db.RwTx, newValue uint32) error {
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, newValue)

	err := tx.Put(storedBatchesCountTable, mainShardKey, bytes)
	if err != nil {
		return fmt.Errorf("failed to put batches count: %w (newValue is %d)", err, newValue)
	}
	return nil
}
