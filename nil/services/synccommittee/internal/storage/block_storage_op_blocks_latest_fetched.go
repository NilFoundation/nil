package storage

import (
	"encoding/json"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// latestFetchedTable stores reference to the latest main shard block.
	// Key: types.ShardId, Value: scTypes.BlockRef.
	latestFetchedTable db.TableName = "latest_fetched"
)

// blockLatestFetchedOp represents the set of operations related to latest fetched blocks within the storage.
type blockLatestFetchedOp struct{}

func (blockLatestFetchedOp) getLatestFetched(tx db.RoTx) (scTypes.BlockRefs, error) {
	iter, err := tx.Range(latestFetchedTable, nil, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	refs := make(scTypes.BlockRefs)

	for iter.HasNext() {
		key, value, err := iter.Next()
		if err != nil {
			return nil, err
		}

		shardId := unmarshallShardId(key)

		var blockRef scTypes.BlockRef
		err = json.Unmarshal(value, &blockRef)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSerializationFailed, err)
		}

		refs[shardId] = blockRef
	}

	return refs, nil
}

func (blockLatestFetchedOp) putLatestFetchedRef(tx db.RwTx, shardId types.ShardId, blockRef *scTypes.BlockRef) error {
	key := marshallShardId(shardId)

	if blockRef == nil {
		if err := tx.Delete(latestFetchedTable, key); err != nil {
			return fmt.Errorf("failed to delete latest fetched ref, shardId=%d: %w", shardId, err)
		}
		return nil
	}

	bytes, err := json.Marshal(blockRef)
	if err != nil {
		return fmt.Errorf(
			"%w: failed to encode block ref with hash=%s: %w", ErrSerializationFailed, blockRef.Hash.String(), err,
		)
	}
	err = tx.Put(latestFetchedTable, key, bytes)
	if err != nil {
		return fmt.Errorf("failed to put block ref with hash=%s: %w", blockRef.Hash.String(), err)
	}
	return nil
}

func (t blockLatestFetchedOp) putLatestFetchedRefs(tx db.RwTx, refs scTypes.BlockRefs) error {
	for _, ref := range refs {
		if err := t.putLatestFetchedRef(tx, ref.ShardId, &ref); err != nil {
			return err
		}
	}
	return nil
}

func (t blockLatestFetchedOp) resetLatestFetched(tx db.RwTx) error {
	iter, err := tx.Range(latestFetchedTable, nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.HasNext() {
		key, _, err := iter.Next()
		if err != nil {
			return err
		}
		if err := tx.Delete(latestFetchedTable, key); err != nil {
			return err
		}
	}

	return nil
}
