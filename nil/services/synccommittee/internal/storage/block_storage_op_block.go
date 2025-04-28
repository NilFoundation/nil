package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// blocksTable stores blocks received from the RPC.
	// Key: scTypes.BlockId (block's own id), Value: blockEntry.
	blocksTable db.TableName = "blocks"
)

// blockOp represents the set of operations related to individual blocks within the storage.
type blockOp struct{}

func (o blockOp) getBlocksAsSegments(tx db.RoTx, ids []scTypes.BlockId) (scTypes.ChainSegments, error) {
	blocks := make(map[types.ShardId][]*scTypes.Block)
	for _, blockId := range ids {
		bEntry, err := o.getBlock(tx, blockId, true)
		if err != nil {
			return nil, err
		}

		shardBlocks := blocks[blockId.ShardId]
		blocks[blockId.ShardId] = append(shardBlocks, &bEntry.Block)
	}

	return scTypes.NewChainSegments(blocks)
}

func (o blockOp) getBlock(tx db.RoTx, id scTypes.BlockId, required bool) (*blockEntry, error) {
	return o.getBlockBytesId(tx, id.Bytes(), required)
}

func (blockOp) blockExists(tx db.RoTx, id scTypes.BlockId) (bool, error) {
	key := id.Bytes()
	exists, err := tx.Exists(blocksTable, key)
	if err != nil {
		return false, fmt.Errorf("failed to check if block with id=%s exists: %w", id, err)
	}
	return exists, nil
}

func (blockOp) getBlockBytesId(tx db.RoTx, idBytes []byte, required bool) (*blockEntry, error) {
	value, err := tx.Get(blocksTable, idBytes)

	switch {
	case errors.Is(err, db.ErrKeyNotFound) && required:
		return nil, fmt.Errorf("%w, id=%s", scTypes.ErrBlockNotFound, hex.EncodeToString(idBytes))
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("failed to get block with id=%s: %w", hex.EncodeToString(idBytes), err)
	}

	entry, err := unmarshallEntry[blockEntry](idBytes, value)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (o blockOp) putBlockIfNotExist(tx db.RwTx, entry *blockEntry, logger logging.Logger) error {
	blockId := scTypes.IdFromBlock(&entry.Block)

	exists, err := o.blockExists(tx, blockId)
	if err != nil {
		return fmt.Errorf("failed to check if block exists, blockId=%s: %w", blockId, err)
	}
	if exists {
		logger.Trace().
			Stringer(logging.FieldShardId, blockId.ShardId).
			Stringer(logging.FieldBlockHash, blockId.Hash).
			Msg("Block already exists, skipping (putBlockIfNotExist)")
		return nil
	}

	value, err := marshallEntry(entry)
	if err != nil {
		return fmt.Errorf("%w, blockId=%s", err, blockId)
	}

	if err := tx.Put(blocksTable, blockId.Bytes(), value); err != nil {
		return fmt.Errorf("failed to put block, blockId=%s: %w", blockId, err)
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
			Msg("Block is not found (deleteBlock)")
		return nil

	default:
		return fmt.Errorf("failed to delete block with id=%s: %w", blockId, err)
	}
}
