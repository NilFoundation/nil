package badger

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
	indexertypes "github.com/NilFoundation/nil/nil/services/indexer/types"
)

const (
	blocksTable        db.TableName = "indexer_blocks"
	actionsTable       db.TableName = "indexer_actions"
	shardLatestTable   db.TableName = "indexer_shard_latest"
	shardEarliestTable db.TableName = "indexer_shard_earliest"
)

type BadgerDriver struct {
	db db.DB
}

type receiptWithSSZ struct {
	decoded    *types.Receipt
	sszEncoded sszx.SSZEncodedData
}

type blockWithSSZ struct {
	Decoded *driver.BlockWithShardId `json:"decoded"`
}

var _ driver.IndexerDriver = &BadgerDriver{}

func NewBadgerDriver(path string) (*BadgerDriver, error) {
	badgerInstance, err := db.NewBadgerDb(path)
	if err != nil {
		return nil, err
	}

	storage := &BadgerDriver{
		db: badgerInstance,
	}

	return storage, nil
}

func (b *BadgerDriver) SetupScheme(ctx context.Context, params driver.SetupParams) error {
	// no need to setup scheme
	return nil
}

func (b *BadgerDriver) IndexBlocks(ctx context.Context, blocksToIndex []*driver.BlockWithShardId) error {
	tx, err := b.db.CreateRwTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	blocks := make([]blockWithSSZ, len(blocksToIndex))
	receipts := make(map[common.Hash]receiptWithSSZ)
	shardLatest := make(map[types.ShardId]types.BlockNumber)

	for blockIndex, block := range blocksToIndex {
		sszEncodedBlock, err := block.EncodeSSZ()
		if err != nil {
			return fmt.Errorf("failed to encode block: %w", err)
		}
		blocks[blockIndex] = blockWithSSZ{Decoded: block}

		for receiptIndex, receipt := range block.Receipts {
			receipts[receipt.TxnHash] = receiptWithSSZ{
				decoded:    receipt,
				sszEncoded: sszEncodedBlock.Receipts[receiptIndex],
			}
		}

		if current, exists := shardLatest[block.ShardId]; !exists || block.Id > current {
			shardLatest[block.ShardId] = block.Id
		}

		key := makeBlockKey(block.ShardId, block.Id)
		value, err := json.Marshal(blocks[blockIndex])
		if err != nil {
			return fmt.Errorf("failed to serialize block: %w", err)
		}
		if err := tx.Put(blocksTable, key, value); err != nil {
			return fmt.Errorf("failed to store block: %w", err)
		}
	}

	for _, block := range blocksToIndex {
		if err := b.indexBlockTransactions(tx, block, receipts); err != nil {
			return fmt.Errorf("failed to index block transactions: %w", err)
		}
	}

	for shardId, latestBlock := range shardLatest {
		if err := b.updateShardLatestProcessedBlock(tx, shardId, latestBlock); err != nil {
			return fmt.Errorf("failed to update latest processed block: %w", err)
		}
		earliestAbsent, hasEarliest, err := b.getShardEarliestAbsentBlock(tx, shardId)
		if err != nil {
			return fmt.Errorf("failed to get earliest absent block: %w", err)
		}
		if !hasEarliest || earliestAbsent > latestBlock+1 {
			if err := b.updateShardEarliestAbsentBlock(tx, shardId, latestBlock+1); err != nil {
				return fmt.Errorf("failed to update earliest absent block: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (b *BadgerDriver) indexBlockTransactions(
	tx db.RwTx,
	block *driver.BlockWithShardId,
	receipts map[common.Hash]receiptWithSSZ,
) error {
	for _, txn := range block.InTransactions {
		hash := txn.Hash()
		receipt, exists := receipts[hash]
		if !exists {
			return fmt.Errorf("receipt not found for transaction %s", hash)
		}

		baseAction := indexertypes.AddressAction{
			Hash:    hash,
			From:    txn.From,
			To:      txn.To,
			Amount:  txn.Value,
			BlockId: block.Id,
			Status:  getTransactionStatus(receipt.decoded),
		}

		logger := logging.NewLogger("indexer-badger")
		logger.Debug().Msgf("indexing block transaction %s, from %s to %s", hash, txn.From, txn.To)

		fromAction := baseAction
		fromAction.Type = indexertypes.SendEth

		if err := storeAddressAction(tx, txn.From, &fromAction); err != nil {
			return fmt.Errorf("failed to store sender action: %w", err)
		}

		toAction := baseAction
		toAction.Type = indexertypes.ReceiveEth
		if err := storeAddressAction(tx, txn.To, &toAction); err != nil {
			return fmt.Errorf("failed to store receiver action: %w", err)
		}
	}

	return nil
}

func (d *BadgerDriver) IndexTxPool(ctx context.Context, txPoolStatuses []*driver.TxPoolStatus) error {
	return errors.New("not implemented")
}

func getTransactionStatus(receipt *types.Receipt) indexertypes.AddressActionStatus {
	if receipt.Success {
		return indexertypes.Success
	}
	return indexertypes.Failed
}

func storeAddressAction(tx db.RwTx, address types.Address, action *indexertypes.AddressAction) error {
	key := makeAddressActionKey(address, uint64(action.BlockId), action.Hash)
	value, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("failed to serialize address action: %w", err)
	}
	return tx.Put(actionsTable, key, value)
}

func makeAddressActionKey(address types.Address, timestamp uint64, txHash common.Hash) []byte {
	key := make([]byte, len(address)+8+len(txHash))
	copy(key[0:], address[:])
	binary.BigEndian.PutUint64(key[len(address):], timestamp)
	copy(key[len(address)+8:], txHash[:])
	return key
}

func makeAddressActionTimestampKey(address types.Address, timestamp uint64) []byte {
	key := make([]byte, len(address)+8)
	copy(key[0:], address[:])
	binary.BigEndian.PutUint64(key[len(address):], timestamp)
	return key
}

func (b *BadgerDriver) FetchAddressActions(
	ctx context.Context,
	address types.Address,
	since types.BlockNumber,
) ([]indexertypes.AddressAction, error) {
	actions := make([]indexertypes.AddressAction, 0)
	const limit = 100

	tx, err := b.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()
	startKey := makeAddressActionTimestampKey(address, uint64(since))
	iter, err := tx.Range(actionsTable, startKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get range iterator: %w", err)
	}
	defer iter.Close()

	for iter.HasNext() && len(actions) < limit {
		key, val, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("iterator error: %w", err)
		}
		if !bytes.HasPrefix(key, startKey[:len(startKey)-8]) {
			break
		}

		var action indexertypes.AddressAction
		if err := json.Unmarshal(val, &action); err != nil {
			return nil, fmt.Errorf("failed to deserialize address action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

func makeBlockKey(shardId types.ShardId, blockNumber types.BlockNumber) []byte {
	key := make([]byte, 4+8)
	binary.BigEndian.PutUint32(key[0:], uint32(shardId))
	binary.BigEndian.PutUint64(key[4:], uint64(blockNumber))
	return key
}

func (b *BadgerDriver) FetchBlock(
	ctx context.Context,
	id types.ShardId,
	number types.BlockNumber,
) (*types.Block, error) {
	var block *types.Block

	tx, err := b.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	key := makeBlockKey(id, number)
	val, err := tx.Get(blocksTable, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var blockWithSSZ blockWithSSZ
	if err := json.Unmarshal(val, &blockWithSSZ); err != nil {
		return nil, err
	}
	if blockWithSSZ.Decoded != nil {
		block = blockWithSSZ.Decoded.Block
	}

	return block, nil
}

func makeShardEarliestAbsentKey(shardId types.ShardId) []byte {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, uint32(shardId))
	return key
}

func makeShardLatestProcessedKey(shardId types.ShardId) []byte {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, uint32(shardId))
	return key
}

func (b *BadgerDriver) updateShardLatestProcessedBlock(
	tx db.RwTx,
	shardId types.ShardId,
	blockNumber types.BlockNumber,
) error {
	key := makeShardLatestProcessedKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Put(shardLatestTable, key, value)
}

func (b *BadgerDriver) updateShardEarliestAbsentBlock(
	tx db.RwTx,
	shardId types.ShardId,
	blockNumber types.BlockNumber,
) error {
	key := makeShardEarliestAbsentKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Put(shardEarliestTable, key, value)
}

func (b *BadgerDriver) getShardLatestProcessedBlock(
	tx db.RoTx,
	shardId types.ShardId,
) (types.BlockNumber, bool, error) {
	key := makeShardLatestProcessedKey(shardId)
	val, err := tx.Get(shardLatestTable, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get latest processed block from db: %w", err)
	}

	if len(val) < 8 {
		return 0, false, fmt.Errorf("invalid data length for latest processed block: got %d, want 8", len(val))
	}
	blockNumber := binary.BigEndian.Uint64(val)
	return types.BlockNumber(blockNumber), true, nil
}

func (b *BadgerDriver) getShardEarliestAbsentBlock(
	tx db.RoTx,
	shardId types.ShardId,
) (types.BlockNumber, bool, error) {
	key := makeShardEarliestAbsentKey(shardId)
	val, err := tx.Get(shardEarliestTable, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	if len(val) < 8 {
		return 0, false, err
	}
	blockNumber := binary.BigEndian.Uint64(val)
	return types.BlockNumber(blockNumber), true, nil
}

func (b *BadgerDriver) FetchLatestProcessedBlockId(ctx context.Context, id types.ShardId) (*types.BlockNumber, error) {
	result := types.InvalidBlockNumber

	tx, err := b.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create read transaction: %w", err)
	}
	defer tx.Rollback()

	latestNumber, hasLatest, err := b.getShardLatestProcessedBlock(tx, id)
	if err != nil {
		return nil, err
	}
	if !hasLatest {
		return &result, nil
	}

	result = latestNumber

	return &result, nil
}

func (b *BadgerDriver) HaveBlock(ctx context.Context, id types.ShardId, number types.BlockNumber) (bool, error) {
	tx, err := b.db.CreateRoTx(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create read transaction: %w", err)
	}
	defer tx.Rollback()

	key := makeBlockKey(id, number)
	exists, err := tx.Exists(blocksTable, key)
	if err != nil {
		return false, fmt.Errorf("failed to check block existence in db: %w", err)
	}

	return exists, nil
}

func (b *BadgerDriver) FetchEarliestAbsentBlockId(ctx context.Context, id types.ShardId) (types.BlockNumber, error) {
	var earliestAbsent types.BlockNumber

	tx, err := b.db.CreateRoTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create read transaction: %w", err)
	}
	defer tx.Rollback()

	earliest, hasEarliest, err := b.getShardEarliestAbsentBlock(tx, id)
	if err != nil {
		return 0, err
	}
	if hasEarliest {
		earliestAbsent = earliest
	}

	return earliestAbsent, nil
}

func (b *BadgerDriver) FetchNextPresentBlockId(
	ctx context.Context,
	id types.ShardId,
	number types.BlockNumber,
) (types.BlockNumber, error) {
	var nextPresent types.BlockNumber

	tx, err := b.db.CreateRoTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create read transaction: %w", err)
	}
	defer tx.Rollback()

	earliestAbsent, hasEarliest, err := b.getShardEarliestAbsentBlock(tx, id)
	if err != nil {
		return 0, err
	}
	if hasEarliest && number < earliestAbsent {
		if earliestAbsent > 0 {
			nextPresent = earliestAbsent - 1
		} else {
			nextPresent = 0
		}
	}

	return nextPresent, nil
}
