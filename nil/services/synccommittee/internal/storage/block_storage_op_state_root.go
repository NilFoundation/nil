package storage

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
)

const (
	// stateRootTable stores the latest ProvedStateRoot (single value).
	// Key: mainShardKey, Value: common.Hash.
	stateRootTable db.TableName = "state_root"

	// nextToProposeTable stores parent's hash of the next block to propose (single value).
	// Key: mainShardKey, Value: common.Hash.
	nextToProposeTable db.TableName = "next_to_propose_parent_hash"
)

// stateRootOp represents the set of operations related to latest fetched blocks within the storage.
type stateRootOp struct{}

func newStateRootOp() stateRootOp {
	return stateRootOp{}
}

func (stateRootOp) putProvedStateRoot(tx db.RwTx, stateRoot common.Hash) error {
	err := tx.Put(stateRootTable, mainShardKey, stateRoot.Bytes())
	if err != nil {
		return fmt.Errorf("failed to put proved state root: %w, value=%s", err, stateRoot)
	}
	return nil
}

func (stateRootOp) getProvedStateRoot(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(stateRootTable, mainShardKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
}

// getParentOfNextToPropose retrieves parent's hash of the next block to propose
func (stateRootOp) getParentOfNextToPropose(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(nextToProposeTable, mainShardKey)

	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get next to propose parent hash: %w", err)
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
}

// setParentOfNextToPropose sets parent's hash of the next block to propose
func (stateRootOp) setParentOfNextToPropose(tx db.RwTx, hash common.Hash) error {
	err := tx.Put(nextToProposeTable, mainShardKey, hash.Bytes())
	if err != nil {
		return fmt.Errorf("failed to put next to propose parent hash: %w", err)
	}
	return nil
}
