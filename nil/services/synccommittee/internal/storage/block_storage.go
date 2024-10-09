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
	blockTableName        db.TableName = "blocks"
	lastFetchedTableName  db.TableName = "last_fetched"
	stateRootTableName    db.TableName = "state_root"
	lastProposedTableName db.TableName = "last_proposed"
)

var mainShardKey = makeShardKey(types.MainShardId)

type blockEntry struct {
	Block    jsonrpc.RPCBlock
	IsProved bool
}

type ProposalData struct {
	MainShardBlock         jsonrpc.RPCBlock
	ExecutionShardBlocks   []jsonrpc.RPCBlock
	CurrentProvedStateRoot common.Hash
}

type BlockStorage interface {
	ProvedStateRootIsInitialized(ctx context.Context) (*bool, error)

	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error

	GetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber) (*jsonrpc.RPCBlock, error)

	SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error

	SetBlockAsProved(ctx context.Context, blockHash common.Hash) error

	GetLastFetchedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error)

	TryGetNextProposalData(ctx context.Context) (*ProposalData, error)

	SetBlockAsProposed(ctx context.Context, mainShardBlockHash common.Hash) error
}

type blockStorage struct {
	db db.DB
}

func NewBlockStorage(database db.DB) BlockStorage {
	return &blockStorage{
		db: database,
	}
}

func (bs *blockStorage) ProvedStateRootIsInitialized(ctx context.Context) (*bool, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	exists, err := tx.Exists(stateRootTableName, mainShardKey)
	if err != nil {
		return nil, err
	}

	return &exists, nil
}

func (bs *blockStorage) SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.Put(stateRootTableName, mainShardKey, stateRoot.Bytes())
	if err != nil {
		return err
	}

	return tx.Commit()
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

	entry, err := bs.getBlockByHash(tx, blockHash)
	if err != nil {
		return err
	}

	if entry == nil {
		return fmt.Errorf("block with hash=%s is not found", blockHash.String())
	}

	entry.IsProved = true
	key := makeBlockKey(entry.Block.ShardId, entry.Block.Number)
	value, err := encodeEntry(entry)
	err = tx.Put(blockTableName, key, value)
	if err != nil {
		return err
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

func (bs *blockStorage) TryGetNextProposalData(ctx context.Context) (*ProposalData, error) {
	tx, err := bs.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentProvedStateRoot, err := bs.getCurrentProvedStateRoot(tx)
	if err != nil {
		return nil, err
	}

	lastProposedBlockHash, err := bs.getLastProposedBlockHash(tx)
	if err != nil {
		return nil, err
	}

	if lastProposedBlockHash != nil {
		var mainShardEntry *blockEntry
		err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
			if isValidProposalCandidate(entry, lastProposedBlockHash) {
				mainShardEntry = entry
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return nil, err
		}

		var executionShardBlocks []jsonrpc.RPCBlock
		err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
			if isExecutionShardBlock(entry, *lastProposedBlockHash) {
				executionShardBlocks = append(executionShardBlocks, entry.Block)
			}
			return true, nil
		})
		if err != nil {
			return nil, err
		}

		return &ProposalData{
			MainShardBlock:         mainShardEntry.Block,
			ExecutionShardBlocks:   executionShardBlocks,
			CurrentProvedStateRoot: *currentProvedStateRoot,
		}, nil
	}

	// todo: scan proved blocks
	return nil, nil
}

func isValidProposalCandidate(entry *blockEntry, lastProposedBlockHash *common.Hash) bool {
	return entry.Block.ShardId == types.MainShardId &&
		entry.IsProved &&
		entry.Block.ParentHash == *lastProposedBlockHash
}

func (bs *blockStorage) getCurrentProvedStateRoot(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(stateRootTableName, mainShardKey)
	if errors.Is(db.ErrKeyNotFound, err) {
		return nil, errors.New("proved state root was not initialized")
	}
	if err != nil {
		return nil, err
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
}

func (bs *blockStorage) getLastProposedBlockHash(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(lastProposedTableName, mainShardKey)

	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
}

func (bs *blockStorage) SetBlockAsProposed(ctx context.Context, mainShardBlockHash common.Hash) error {
	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	mainShardEntry, err := bs.getBlockByHash(tx, mainShardBlockHash)
	if err != nil {
		return err
	}

	if err := bs.validateMainShardEntry(tx, mainShardEntry, mainShardBlockHash); err != nil {
		return err
	}

	err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
		if !isExecutionShardBlock(entry, mainShardBlockHash) {
			return true, nil
		}

		err := tx.Delete(blockTableName, makeBlockKey(entry.Block.ShardId, entry.Block.Number))
		return true, err
	})
	if err != nil {
		return err
	}

	err = tx.Delete(blockTableName, makeBlockKey(mainShardEntry.Block.ShardId, mainShardEntry.Block.Number))
	if err != nil {
		return err
	}

	err = tx.Put(stateRootTableName, mainShardKey, mainShardEntry.Block.ChildBlocksRootHash.Bytes())
	if err != nil {
		return err
	}

	err = tx.Put(lastProposedTableName, mainShardKey, mainShardBlockHash.Bytes())
	if err != nil {
		return err
	}

	return tx.Commit()
}

func isExecutionShardBlock(entry *blockEntry, mainShardBlockHash common.Hash) bool {
	return entry.Block.ShardId != types.MainShardId && entry.Block.ParentHash == mainShardBlockHash
}

func (bs *blockStorage) validateMainShardEntry(tx db.RwTx, entry *blockEntry, blockHash common.Hash) error {
	if entry == nil {
		return fmt.Errorf("block with hash=%s is not found", blockHash.String())
	}

	if entry.Block.ShardId != types.MainShardId {
		return fmt.Errorf("block with hash=%s is not from main shard", blockHash.String())
	}

	if !entry.IsProved {
		return fmt.Errorf("block with hash=%s is not proved", blockHash.String())
	}

	lastProposedBlockHash, err := bs.getLastProposedBlockHash(tx)
	if err != nil {
		return err
	}
	if lastProposedBlockHash != nil && *lastProposedBlockHash != entry.Block.ParentHash {
		return fmt.Errorf(
			"last proposed block hash=%s is not equal to the parent's block hash=%s",
			lastProposedBlockHash.String(),
			entry.Block.ParentHash.String(),
		)
	}
	return nil
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

func (bs *blockStorage) getBlockByHash(tx db.RoTx, blockHash common.Hash) (*blockEntry, error) {
	// todo: refactor after switching to hash-based keys
	// https://www.notion.so/nilfoundation/Out-of-order-block-number-f549ca82b2db4a0d9ef71bdde5c878b0?pvs=4

	var target *blockEntry
	err := iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
		if entry.Block.Hash != blockHash {
			return true, nil
		}

		target = entry
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return target, nil
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
