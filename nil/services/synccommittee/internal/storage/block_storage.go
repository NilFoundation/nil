package storage

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

const (
	blockTableName       db.TableName = "blocks"
	lastFetchedTableName db.TableName = "last_fetched"
	lastProvedTableName  db.TableName = "last_proved"
)

type BlockStorage interface {
	GetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber) (*jsonrpc.RPCBlock, error)

	SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error

	GetLastFetchedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error)

	GetLastProvedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error)

	SetLastProvedBlockNum(ctx context.Context, shardId types.ShardId, blockNum types.BlockNumber) error

	CleanupStorage(ctx context.Context) error
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

	var block jsonrpc.RPCBlock
	err = json.Unmarshal(value, &block)
	if err != nil {
		return nil, err
	}

	return &block, nil
}

func (bs *blockStorage) SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := makeBlockKey(shardId, blockNumber)
	value, err := json.Marshal(block)
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

func (bs *blockStorage) GetLastProvedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	value, err := tx.Get(lastProvedTableName, makeShardKey(shardId))
	if err != nil {
		return 0, err
	}

	return types.BlockNumber(binary.LittleEndian.Uint64(value)), nil
}

func (bs *blockStorage) SetLastProvedBlockNum(ctx context.Context, shardId types.ShardId, blockNum types.BlockNumber) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, uint64(blockNum))

	err = tx.Put(lastProvedTableName, makeShardKey(shardId), value)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (bs *blockStorage) getBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber) ([]*jsonrpc.RPCBlock, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	iter, err := tx.Range(blockTableName, makeBlockKey(shardId, from), makeBlockKey(shardId, to-1))
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var blocks []*jsonrpc.RPCBlock
	for iter.HasNext() {
		_, value, err := iter.Next()
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				// Some blocks could be missed
				continue
			}
			return nil, err
		}

		var block jsonrpc.RPCBlock
		err = json.Unmarshal(value, &block)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, &block)
	}

	return blocks, nil
}

func (bs *blockStorage) CleanupStorage(ctx context.Context) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	iter, err := tx.Range(blockTableName, nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.HasNext() {
		key, _, err := iter.Next()
		if err != nil {
			return err
		}

		shardId, blockNum := parseBlockKey(key)
		lastProvedNum, err := bs.GetLastProvedBlockNum(ctx, shardId)
		if err != nil {
			return err
		}

		if blockNum < lastProvedNum {
			err = tx.Delete(blockTableName, key)
			if err != nil {
				return err
			}
		}
	}

	iter.Close()
	return tx.Commit()
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

func parseBlockKey(key []byte) (types.ShardId, types.BlockNumber) {
	shardId := types.ShardId(binary.LittleEndian.Uint64(key[:8]))
	blockNum := types.BlockNumber(binary.LittleEndian.Uint64(key[8:]))
	return shardId, blockNum
}
