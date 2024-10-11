package storage

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/rs/zerolog"
)

const (
	blockTableName         db.TableName = "blocks"
	lastFetchedTableName   db.TableName = "last_fetched"
	stateRootTableName     db.TableName = "state_root"
	nextToProposeTableName db.TableName = "next_to_propose_parent_hash"
)

var mainShardKey = makeShardKey(types.MainShardId)

type blockEntry struct {
	Block    jsonrpc.RPCBlock `json:"block"`
	IsProved bool             `json:"isProved"`
}

type PrunedTransaction struct {
	Flags types.MessageFlags
	Seqno hexutil.Uint64
	From  types.Address
	To    types.Address
	Value types.Value
	Data  hexutil.Bytes
}

type ProposalData struct {
	MainShardBlockHash common.Hash
	Transactions       []PrunedTransaction
	OldProvedStateRoot common.Hash
	NewProvedStateRoot common.Hash
}

type BlockStorage interface {
	ProvedStateRootIsInitialized(ctx context.Context) (*bool, error)

	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error

	GetLastFetchedBlockNum(ctx context.Context, shardId types.ShardId) (types.BlockNumber, error)

	GetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber) (*jsonrpc.RPCBlock, error)

	SetBlock(ctx context.Context, shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) error

	SetBlockAsProved(ctx context.Context, blockHash common.Hash) error

	SetBlockAsProposed(ctx context.Context, blockHash common.Hash) error

	TryGetNextProposalData(ctx context.Context) (*ProposalData, error)
}

type blockStorage struct {
	db          db.DB
	retryRunner common.RetryRunner
	logger      zerolog.Logger
}

func NewBlockStorage(database db.DB, logger zerolog.Logger) BlockStorage {
	return &blockStorage{
		db:          database,
		retryRunner: badgerRetryRunner(logger),
		logger:      logger,
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
	if stateRoot.Empty() {
		return errors.New("state root hash is empty")
	}

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

	err = bs.setProposeParentHash(tx, block)
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

func (bs *blockStorage) setProposeParentHash(tx db.RwTx, block *jsonrpc.RPCBlock) error {
	if block.ShardId != types.MainShardId {
		return nil
	}

	parentHash, err := bs.getParentOfNextToPropose(tx)
	if err != nil {
		return err
	}
	if parentHash != nil && !parentHash.Empty() {
		return nil
	}

	if block.Number > 0 && block.ParentHash.Empty() {
		return fmt.Errorf("block with hash=%s has empty parent hash", block.Hash.String())
	}

	bs.logger.Debug().
		Str("blockHash", block.Hash.String()).
		Str("parentHash", block.ParentHash.String()).
		Msg("block parent hash is not set, updating it")

	return bs.setParentOfNextToPropose(tx, block.ParentHash)
}

func (bs *blockStorage) SetBlockAsProved(ctx context.Context, blockHash common.Hash) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.setBlockAsProvedImpl(ctx, blockHash)
	})
}

func (bs *blockStorage) setBlockAsProvedImpl(ctx context.Context, blockHash common.Hash) error {
	if blockHash.Empty() {
		return errors.New("block hash is empty")
	}

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
	if err != nil {
		return err
	}

	err = tx.Put(blockTableName, key, value)
	if err != nil {
		return err
	}
	return tx.Commit()
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

	parentHash, err := bs.getParentOfNextToPropose(tx)
	if err != nil {
		return nil, err
	}
	if parentHash == nil || parentHash.Empty() {
		bs.logger.Debug().Msg("block parent hash is not set")
		return nil, nil
	}

	var mainShardEntry *blockEntry
	err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
		if isValidProposalCandidate(entry, parentHash) {
			mainShardEntry = entry
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	if mainShardEntry == nil {
		bs.logger.Debug().Str("parentHash", parentHash.String()).Msg("no proved main shard block found")
		return nil, nil
	}

	transactions, err := extractTransactions(mainShardEntry.Block)
	if err != nil {
		return nil, fmt.Errorf("failed to extract transactions: %w", err)
	}

	err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
		if isExecutionShardBlock(entry, mainShardEntry.Block.Hash) {
			blockTransactions, err := extractTransactions(entry.Block)
			if err != nil {
				return false, fmt.Errorf("failed to extract transactions: %w", err)
			}
			transactions = append(transactions, blockTransactions...)
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return &ProposalData{
		MainShardBlockHash: mainShardEntry.Block.Hash,
		Transactions:       transactions,
		OldProvedStateRoot: *currentProvedStateRoot,
		NewProvedStateRoot: mainShardEntry.Block.ChildBlocksRootHash,
	}, nil
}

func (bs *blockStorage) SetBlockAsProposed(ctx context.Context, blockHash common.Hash) error {
	return bs.retryRunner.Do(ctx, func(ctx context.Context) error {
		return bs.setBlockAsProposedImpl(ctx, blockHash)
	})
}

func (bs *blockStorage) setBlockAsProposedImpl(ctx context.Context, blockHash common.Hash) error {
	if blockHash.Empty() {
		return errors.New("block hash is empty")
	}

	tx, err := bs.db.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	mainShardEntry, err := bs.getBlockByHash(tx, blockHash)
	if err != nil {
		return err
	}

	if err := bs.validateMainShardEntry(tx, mainShardEntry, blockHash); err != nil {
		return err
	}

	err = iterateOverEntries(tx, func(entry *blockEntry) (bool, error) {
		if !isExecutionShardBlock(entry, blockHash) {
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
		return fmt.Errorf("failed to put state root: %w", err)
	}

	err = bs.setParentOfNextToPropose(tx, blockHash)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func extractTransactions(block jsonrpc.RPCBlock) ([]PrunedTransaction, error) {
	transactions := make([]PrunedTransaction, len(block.Messages))

	for idx, message := range block.Messages {
		rpcInMessage, ok := message.(map[string]any)
		if !ok {
			return nil, errors.New("failed to cast message to map")
		}

		seqnoStr, ok := rpcInMessage["seqno"].(string)
		if !ok {
			return nil, errors.New("failed to cast seqno to string")
		}

		var seqno hexutil.Uint64
		err := seqno.UnmarshalText([]byte(seqnoStr))
		if err != nil {
			return nil, err
		}

		valueStr, ok := rpcInMessage["value"].(string)
		if !ok {
			return nil, errors.New("failed to cast value to string")
		}

		var value types.Value
		err = value.Set(valueStr)
		if err != nil {
			return nil, err
		}

		dataStr, ok := rpcInMessage["data"].(string)
		if !ok {
			return nil, errors.New("failed to cast data to string")
		}

		var data hexutil.Bytes
		err = data.Set(dataStr)
		if err != nil {
			return nil, err
		}

		fromStr, ok := rpcInMessage["from"].(string)
		if !ok {
			return nil, errors.New("failed to cast from to string")
		}

		toStr, ok := rpcInMessage["to"].(string)
		if !ok {
			return nil, errors.New("failed to cast to to string")
		}

		transactions[idx] = PrunedTransaction{
			Seqno: seqno,
			From:  types.HexToAddress(fromStr),
			To:    types.HexToAddress(toStr),
			Value: value,
			Data:  data,
		}
	}
	return transactions, nil
}

func isValidProposalCandidate(entry *blockEntry, parentHash *common.Hash) bool {
	return entry.Block.ShardId == types.MainShardId &&
		entry.IsProved &&
		entry.Block.ParentHash == *parentHash
}

func (bs *blockStorage) getCurrentProvedStateRoot(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(stateRootTableName, mainShardKey)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, errors.New("proved state root was not initialized")
	}
	if err != nil {
		return nil, err
	}

	hash := common.BytesToHash(hashBytes)
	return &hash, nil
}

// getParentOfNextToPropose retrieves parent's hash of the next block to propose
func (bs *blockStorage) getParentOfNextToPropose(tx db.RoTx) (*common.Hash, error) {
	hashBytes, err := tx.Get(nextToProposeTableName, mainShardKey)

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
func (bs *blockStorage) setParentOfNextToPropose(tx db.RwTx, hash common.Hash) error {
	err := tx.Put(nextToProposeTableName, mainShardKey, hash.Bytes())
	if err != nil {
		return fmt.Errorf("failed to put next to propose parent hash: %w", err)
	}
	return nil
}

func isExecutionShardBlock(entry *blockEntry, mainShardBlockHash common.Hash) bool {
	return entry.Block.ShardId != types.MainShardId && entry.Block.MainChainHash == mainShardBlockHash
}

func (bs *blockStorage) validateMainShardEntry(tx db.RoTx, entry *blockEntry, blockHash common.Hash) error {
	if entry == nil {
		return fmt.Errorf("block with hash=%s is not found", blockHash.String())
	}

	if entry.Block.ShardId != types.MainShardId {
		return fmt.Errorf("block with hash=%s is not from main shard", blockHash.String())
	}

	if !entry.IsProved {
		return fmt.Errorf("block with hash=%s is not proved", blockHash.String())
	}

	parentHash, err := bs.getParentOfNextToPropose(tx)
	if err != nil {
		return err
	}
	if parentHash == nil {
		return errors.New("next to propose parent hash is not set")
	}

	if *parentHash != entry.Block.ParentHash {
		return fmt.Errorf(
			"parent's block hash=%s is not equal to the stored value=%s",
			entry.Block.ParentHash.String(),
			parentHash.String(),
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
