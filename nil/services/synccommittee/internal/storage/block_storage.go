package storage

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

const (
	BlockTableName       db.TableName = "blocks"
	LastFetchedTableName db.TableName = "last_fetched"
	LastProvedTableName  db.TableName = "last_proved"
)

type BlockStorage struct {
	db db.DB
}

type PrunedTransaction struct {
	flags types.MessageFlags
	seqno hexutil.Uint64
	from  types.Address
	to    types.Address
	value types.Value
	data  hexutil.Bytes
}

func NewBlockStorage(database db.DB) *BlockStorage {
	return &BlockStorage{
		db: database,
	}
}

// Returns nil block with no error in case there is no such block in database
func (bs *BlockStorage) GetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber) (*jsonrpc.RPCBlock, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	key := makeBlockKey(shardId, blockNumber)
	value, err := tx.Get(BlockTableName, key)
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

func (bs *BlockStorage) SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error {
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

	err = tx.Put(BlockTableName, key, value)
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
		if err = tx.Put(LastFetchedTableName, makeShardKey(shardId), blockNumberValue); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (bs *BlockStorage) GetLastFetchedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error) {
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

func (bs *BlockStorage) getLastFetchedBlockNumTx(tx db.RoTx, shardId types.ShardId) (types.BlockNumber, error) {
	value, err := tx.Get(LastFetchedTableName, makeShardKey(shardId))
	if err != nil {
		return 0, err
	}

	return types.BlockNumber(binary.LittleEndian.Uint64(value)), nil
}

func (bs *BlockStorage) GetLastProvedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	value, err := tx.Get(LastProvedTableName, makeShardKey(shardId))
	if err != nil {
		return 0, err
	}

	return types.BlockNumber(binary.LittleEndian.Uint64(value)), nil
}

func (bs *BlockStorage) SetLastProvedBlockNum(ctx context.Context, shardId types.ShardId, blockNum types.BlockNumber) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, uint64(blockNum))

	err = tx.Put(LastProvedTableName, makeShardKey(shardId), value)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (bs *BlockStorage) GetBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber) ([]*jsonrpc.RPCBlock, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	iter, err := tx.Range(BlockTableName, makeBlockKey(shardId, from), makeBlockKey(shardId, to-1))
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

func (bs *BlockStorage) GetTransactionsByBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber) ([]*PrunedTransaction, error) {
	blocks, err := bs.GetBlocksRange(ctx, shardId, from, to)
	if err != nil {
		return nil, err
	}

	var transactions []*PrunedTransaction
	for _, block := range blocks {
		for _, msg_any := range block.Messages {
			if msg, success := msg_any.(jsonrpc.RPCInMessage); success {
				t := &PrunedTransaction{
					flags: msg.Flags,
					seqno: msg.Seqno,
					from:  msg.From,
					to:    msg.To,
					value: msg.Value,
					data:  msg.Data,
				}
				transactions = append(transactions, t)
			}
		}
	}
	return transactions, nil
}

func (bs *BlockStorage) CleanupStorage(ctx context.Context) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	iter, err := tx.Range(BlockTableName, nil, nil)
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
			err = tx.Delete(BlockTableName, key)
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
