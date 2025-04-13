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
)

// stateRootOp represents the set of operations related to latest fetched blocks within the storage.
type stateRootOp struct{}

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
