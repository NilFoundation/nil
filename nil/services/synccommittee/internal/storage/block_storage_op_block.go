package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// blocksTable stores blocks received from the RPC.
	// Key: scTypes.BlockId (block's own id), Value: blockEntry.
	blocksTable db.TableName = "blocks"
)

// blockOp represents the set of operations related to individual blocks within the storage.
type blockOp struct{}

func (bs blockOp) getBlock(tx db.RoTx, id scTypes.BlockId, required bool) (*blockEntry, error) {
	return bs.getBlockBytesId(tx, id.Bytes(), required)
}

func (blockOp) getBlockBytesId(tx db.RoTx, idBytes []byte, required bool) (*blockEntry, error) {
	value, err := tx.Get(blocksTable, idBytes)

	switch {
	case err == nil:
	case errors.Is(err, db.ErrKeyNotFound) && required:
		return nil, fmt.Errorf("%w, id=%s", scTypes.ErrBlockNotFound, hex.EncodeToString(idBytes))
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to get block with id=%s: %w", hex.EncodeToString(idBytes), err)
	}

	entry, err := unmarshallEntry[blockEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (blockOp) putBlockTx(tx db.RwTx, entry *blockEntry) error {
	value, err := marshallEntry(entry)
	if err != nil {
		return fmt.Errorf("%w, hash=%s", err, entry.Block.Hash)
	}

	blockId := scTypes.IdFromBlock(&entry.Block)
	if err := tx.Put(blocksTable, blockId.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put block %s: %w", blockId.String(), err)
	}

	return nil
}

func (blockOp) deleteBlock(tx db.RwTx, blockId scTypes.BlockId, logger logging.Logger) error {
	err := tx.Delete(blocksTable, blockId.Bytes())

	switch {
	case err == nil:
		return nil

	case errors.Is(err, context.Canceled):
		return err

	case errors.Is(err, db.ErrKeyNotFound):
		logger.Warn().Err(err).
			Stringer(logging.FieldShardId, blockId.ShardId).
			Stringer(logging.FieldBlockHash, blockId.Hash).
			Msg("block is not found (deleteBlock)")
		return nil

	default:
		return fmt.Errorf("failed to delete block with id=%s: %w", blockId, err)
	}
}
