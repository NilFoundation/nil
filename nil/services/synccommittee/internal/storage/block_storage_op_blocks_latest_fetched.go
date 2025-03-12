package storage

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// latestFetchedTable stores reference to the latest main shard block.
	// Key: mainShardKey, Value: scTypes.MainBlockRef.
	latestFetchedTable db.TableName = "latest_fetched"
)

// blocksLatestFetchedOp represents the set of operations related to latest fetched blocks within the storage.
type blocksLatestFetchedOp struct{}

func newBlocksLatestFetchedOp() blocksLatestFetchedOp {
	return blocksLatestFetchedOp{}
}

func (blocksLatestFetchedOp) getLatestFetchedMain(tx db.RoTx) (*scTypes.MainBlockRef, error) {
	value, err := tx.Get(latestFetchedTable, mainShardKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var blockRef *scTypes.MainBlockRef
	err = json.Unmarshal(value, &blockRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSerializationFailed, err)
	}
	return blockRef, nil
}

func (blocksLatestFetchedOp) putLatestFetchedBlock(tx db.RwTx, shardId types.ShardId, block *scTypes.MainBlockRef) error {
	bytes, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf(
			"%w: failed to encode block ref with hash=%s: %w", ErrSerializationFailed, block.Hash.String(), err,
		)
	}
	err = tx.Put(latestFetchedTable, makeShardKey(shardId), bytes)
	if err != nil {
		return fmt.Errorf("failed to put block ref with hash=%s: %w", block.Hash.String(), err)
	}
	return nil
}

func (t blocksLatestFetchedOp) updateLatestFetched(tx db.RwTx, block *jsonrpc.RPCBlock) error {
	if block.ShardId != types.MainShardId {
		return nil
	}

	latestFetched, err := t.getLatestFetchedMain(tx)
	if err != nil {
		return err
	}

	if latestFetched.Equals(block) {
		return nil
	}

	if err := latestFetched.ValidateChild(block); err != nil {
		return fmt.Errorf("unable to update latest fetched block: %w", err)
	}

	newLatestFetched, err := scTypes.NewBlockRef(block)
	if err != nil {
		return err
	}

	return t.putLatestFetchedBlock(tx, block.ShardId, newLatestFetched)
}
