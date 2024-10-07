package storage

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

const (
	blockTableName       db.TableName = "blocks"
	lastFetchedTableName db.TableName = "last_fetched"
)

type blockEntry struct {
	Block    jsonrpc.RPCBlock
	IsProved bool
}

type BlockStorage interface {
	GetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber) (*jsonrpc.RPCBlock, error)

	SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error

	SetBlockAsProved(ctx context.Context, blockHash common.Hash) error

	GetLastFetchedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error)
}

type blockStorage struct {
	db db.DB
}

func NewBlockStorage(database db.DB) BlockStorage {
	return &blockStorage{
		db: database,
	}
}

func (bs *blockStorage) GetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber) (*jsonrpc.RPCBlock, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	key := makeBlockKey(shardId, blockNumber)
	value, err := tx.Get(blockTableName, key)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	entry, err := unmarshallEntry(&key, &value)
	if err != nil {
		return nil, err
	}

	return &entry.Block, nil
}

func (bs *blockStorage) SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := makeBlockKey(shardId, blockNumber)
	entry := blockEntry{Block: *block}
	value, err := encodeEntry(&entry)
	if err != nil {
		return err
	}

	err = tx.Put(blockTableName, key, value)
	if err != nil {
		return err
	}

	// Update last fetched block if necessary
	lastFetchedBlockNum, err := bs.getLastFetchedBlockNumTx(tx, shardId)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}
	if errors.Is(err, db.ErrKeyNotFound) || block.Number > lastFetchedBlockNum {
		blockNumberValue := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumberValue, uint64(blockNumber))
		if err = tx.Put(lastFetchedTableName, makeShardKey(shardId), blockNumberValue); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (bs *blockStorage) SetBlockAsProved(ctx context.Context, blockHash common.Hash) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// todo: refactor after switching to hash-based keys
	// https://www.notion.so/nilfoundation/Out-of-order-block-number-f549ca82b2db4a0d9ef71bdde5c878b0?pvs=4

	var target *blockEntry
	err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
		if entry.Block.Hash != blockHash {
			return true, nil
		}

		target = entry
		return false, nil
	})

	if err != nil {
		return err
	}

	if target == nil {
		return fmt.Errorf("block with hash=%s is not found", blockHash.String())
	}

	target.IsProved = true
	key := makeBlockKey(target.Block.ShardId, target.Block.Number)
	value, err := encodeEntry(target)
	return tx.Put(blockTableName, key, value)
}

func (bs *blockStorage) GetLastFetchedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	lastFetchedBlockNum, err := bs.getLastFetchedBlockNumTx(tx, shardId)
	if err != nil {
		return 0, err
	}

	return lastFetchedBlockNum, nil
}

func (bs *blockStorage) getLastFetchedBlockNumTx(tx db.RoTx, shardId types.ShardId) (types.BlockNumber, error) {
	value, err := tx.Get(lastFetchedTableName, makeShardKey(shardId))
	if err != nil {
		return 0, err
	}

	return types.BlockNumber(binary.LittleEndian.Uint64(value)), nil
}

func makeBlockKey(shardId types.ShardId, blockNumber types.BlockNumber) []byte {
	key := make([]byte, 16)
	binary.LittleEndian.PutUint64(key[:8], uint64(shardId))
	binary.LittleEndian.PutUint64(key[8:], uint64(blockNumber))
	return key
}

func makeShardKey(shardId types.ShardId) []byte {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, uint64(shardId))
	return key
}

func iterateOverEntries(tx db.RoTx, action func(entry *blockEntry) (shouldContinue bool, err error)) error {
	iter, err := tx.Range(blockTableName, nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.HasNext() {
		key, val, err := iter.Next()
		if err != nil {
			return err
		}
		entry, err := unmarshallEntry(&key, &val)
		if err != nil {
			return err
		}
		shouldContinue, err := action(entry)
		if err != nil {
			return err
		}
		if !shouldContinue {
			return nil
		}
	}

	return nil
}

func encodeEntry(entry *blockEntry) ([]byte, error) {
	bytes, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to encode block with hash %s: %w", entry.Block.Hash.String(), err)
	}
	return bytes, nil
}

func unmarshallEntry(key *[]byte, val *[]byte) (*blockEntry, error) {
	entry := &blockEntry{}
	if err := json.Unmarshal(*val, entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshall block entry with id %v: %w", string(*key), err)
	}
	return entry, nil
}
