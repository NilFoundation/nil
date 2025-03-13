package badger

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
	types2 "github.com/NilFoundation/nil/nil/services/indexer/types"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/dgraph-io/badger/v4"
)

type BadgerDriver struct {
	db *badger.DB
}

type receiptWithSSZ struct {
	decoded    *types.Receipt
	sszEncoded sszx.SSZEncodedData
}

type blockWithSSZ struct {
	decoded    *driver.BlockWithShardId
	sszEncoded *types.RawBlockWithExtractedData
}

var _ driver.IndexerDriver = &BadgerDriver{}

func NewBadgerDriver(path string) (*BadgerDriver, error) {
	opts := badger.DefaultOptions(path).WithLogger(nil)
	badgerInstance, err := badger.Open(opts)
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

func (b *BadgerDriver) IndexBlocks(_ context.Context, blocksToIndex []*driver.BlockWithShardId) error {
	tx := b.createRwTx()
	defer tx.Discard()

	blocks := make([]blockWithSSZ, len(blocksToIndex))
	receipts := make(map[common.Hash]receiptWithSSZ)

	shardLatest := make(map[types.ShardId]types.BlockNumber)

	for blockIndex, block := range blocksToIndex {
		sszEncodedBlock, err := block.EncodeSSZ()
		if err != nil {
			return fmt.Errorf("failed to encode block: %w", err)
		}
		blocks[blockIndex] = blockWithSSZ{decoded: block, sszEncoded: sszEncodedBlock}

		for receiptIndex, receipt := range block.Receipts {
			receipts[receipt.TxnHash] = receiptWithSSZ{
				decoded:    receipt,
				sszEncoded: sszEncodedBlock.Receipts[receiptIndex],
			}
		}

		if current, exists := shardLatest[block.ShardId]; !exists || block.Block.Id > current {
			shardLatest[block.ShardId] = block.Block.Id
		}

		key := makeBlockKey(block.ShardId, block.Block.Id)
		value, err := json.Marshal(blocks[blockIndex])
		if err != nil {
			return fmt.Errorf("failed to serialize block: %w", err)
		}
		if err := tx.Set(key, value); err != nil {
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

func (b *BadgerDriver) indexBlockTransactions(tx *badger.Txn, block *driver.BlockWithShardId, receipts map[common.Hash]receiptWithSSZ) error {
	for _, txn := range block.InTransactions {
		receipt, exists := receipts[txn.Hash()]
		if !exists {
			return fmt.Errorf("receipt not found for transaction %s", txn.Hash())
		}

		baseAction := types2.AddressAction{
			Hash:      txn.Hash(),
			From:      txn.From,
			To:        txn.To,
			Amount:    txn.Value,
			Timestamp: db.Timestamp(block.Block.Timestamp),
			BlockId:   block.Block.Id,
			Status:    getTransactionStatus(receipt.decoded),
		}

		fromAction := baseAction
		fromAction.Type = types2.SendEth
		if err := storeAddressAction(tx, txn.From, &fromAction); err != nil {
			return fmt.Errorf("failed to store sender action: %w", err)
		}

		toAction := baseAction
		toAction.Type = types2.ReceiveEth
		if err := storeAddressAction(tx, txn.To, &toAction); err != nil {
			return fmt.Errorf("failed to store receiver action: %w", err)
		}
	}

	return nil
}

func getTransactionStatus(receipt *types.Receipt) types2.AddressActionStatus {
	if receipt.Success {
		return types2.Success
	}
	return types2.Failed
}

func storeAddressAction(tx *badger.Txn, address types.Address, action *types2.AddressAction) error {
	key := makeAddressActionKey(address, uint64(action.Timestamp), action.Hash)
	value, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("failed to serialize address action: %w", err)
	}
	return tx.Set(key, value)
}

func makeAddressActionKey(address types.Address, timestamp uint64, txHash common.Hash) []byte {
	key := make([]byte, len("actions:")+len(address)+8+len(txHash))
	copy(key[0:], "actions:")
	copy(key[len("actions:"):], address[:])
	binary.BigEndian.PutUint64(key[len("actions:")+len(address):], timestamp)
	copy(key[len("actions:")+len(address)+8:], txHash[:])
	return key
}

func makeAddressActionPrefix(address types.Address) []byte {
	prefix := make([]byte, len("actions:")+len(address))
	copy(prefix[0:], "actions:")
	copy(prefix[len("actions:"):], address[:])
	return prefix
}

func makeAddressActionTimestampKey(address types.Address, timestamp uint64) []byte {
	key := make([]byte, len("actions:")+len(address)+8)
	copy(key[0:], "actions:")
	copy(key[len("actions:"):], address[:])
	binary.BigEndian.PutUint64(key[len("actions:")+len(address):], timestamp)
	return key
}

func (b *BadgerDriver) FetchAddressActions(address types.Address, since db.Timestamp) ([]types2.AddressAction, error) {
	actions := make([]types2.AddressAction, 0)
	const limit = 100

	err := b.db.View(func(txn *badger.Txn) error {
		prefix := makeAddressActionPrefix(address)
		startKey := makeAddressActionTimestampKey(address, uint64(since))

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(startKey)
		for it.Valid() && len(actions) < limit {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var action types2.AddressAction
				if err := json.Unmarshal(val, &action); err != nil {
					return fmt.Errorf("failed to deserialize address action: %w", err)
				}
				actions = append(actions, action)
				return nil
			})
			if err != nil {
				return err
			}
			it.Next()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get address actions: %w", err)
	}

	return actions, nil
}

func makeShardCurrentKey(shardId types.ShardId) []byte {
	key := make([]byte, len("shard:")+4+len(":current"))
	copy(key[0:], "shard:")
	binary.BigEndian.PutUint32(key[len("shard:"):], uint32(shardId))
	copy(key[len("shard:")+4:], ":current")
	return key
}

func (b *BadgerDriver) updateShardCurrentBlock(tx *badger.Txn, shardId types.ShardId, blockNumber types.BlockNumber) error {
	key := makeShardCurrentKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Set(key, value)
}

func (b *BadgerDriver) getShardCurrentBlock(tx *badger.Txn, shardId types.ShardId) (types.BlockNumber, bool, error) {
	key := makeShardCurrentKey(shardId)
	item, err := tx.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get shard current block: %w", err)
	}

	var blockNumber uint64
	err = item.Value(func(val []byte) error {
		blockNumber = binary.BigEndian.Uint64(val)
		return nil
	})
	if err != nil {
		return 0, false, fmt.Errorf("failed to read shard current block value: %w", err)
	}

	return types.BlockNumber(blockNumber), true, nil
}

func makeBlockKey(shardId types.ShardId, blockNumber types.BlockNumber) []byte {
	key := make([]byte, len("block:")+4+8)
	copy(key[0:], "block:")
	binary.BigEndian.PutUint32(key[len("block:"):], uint32(shardId))
	binary.BigEndian.PutUint64(key[len("block:")+4:], uint64(blockNumber))
	return key
}

func (b *BadgerDriver) FetchBlock(_ context.Context, id types.ShardId, number types.BlockNumber) (*types.Block, error) {
	var block *types.Block

	err := b.db.View(func(txn *badger.Txn) error {
		key := makeBlockKey(id, number)
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get block: %w", err)
		}

		err = item.Value(func(val []byte) error {
			var blockWithSSZ blockWithSSZ
			if err := json.Unmarshal(val, &blockWithSSZ); err != nil {
				return fmt.Errorf("failed to deserialize block: %w", err)
			}
			block = blockWithSSZ.decoded.Block
			return nil
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block: %w", err)
	}

	return block, nil
}

func makeShardEarliestAbsentKey(shardId types.ShardId) []byte {
	key := make([]byte, len("shard:")+4+len(":earliest_absent"))
	copy(key[0:], "shard:")
	binary.BigEndian.PutUint32(key[len("shard:"):], uint32(shardId))
	copy(key[len("shard:")+4:], ":earliest_absent")
	return key
}

func makeShardLatestProcessedKey(shardId types.ShardId) []byte {
	key := make([]byte, len("shard:")+4+len(":latest_processed"))
	copy(key[0:], "shard:")
	binary.BigEndian.PutUint32(key[len("shard:"):], uint32(shardId))
	copy(key[len("shard:")+4:], ":latest_processed")
	return key
}

func (b *BadgerDriver) updateShardLatestProcessedBlock(tx *badger.Txn, shardId types.ShardId, blockNumber types.BlockNumber) error {
	key := makeShardLatestProcessedKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Set(key, value)
}

func (b *BadgerDriver) updateShardEarliestAbsentBlock(tx *badger.Txn, shardId types.ShardId, blockNumber types.BlockNumber) error {
	key := makeShardEarliestAbsentKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Set(key, value)
}

func (b *BadgerDriver) getShardLatestProcessedBlock(tx *badger.Txn, shardId types.ShardId) (types.BlockNumber, bool, error) {
	key := makeShardLatestProcessedKey(shardId)
	item, err := tx.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get latest processed block: %w", err)
	}

	var blockNumber uint64
	err = item.Value(func(val []byte) error {
		blockNumber = binary.BigEndian.Uint64(val)
		return nil
	})
	if err != nil {
		return 0, false, fmt.Errorf("failed to read latest processed block value: %w", err)
	}

	return types.BlockNumber(blockNumber), true, nil
}

func (b *BadgerDriver) getShardEarliestAbsentBlock(tx *badger.Txn, shardId types.ShardId) (types.BlockNumber, bool, error) {
	key := makeShardEarliestAbsentKey(shardId)
	item, err := tx.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to get earliest absent block: %w", err)
	}

	var blockNumber uint64
	err = item.Value(func(val []byte) error {
		blockNumber = binary.BigEndian.Uint64(val)
		return nil
	})
	if err != nil {
		return 0, false, fmt.Errorf("failed to read earliest absent block value: %w", err)
	}

	return types.BlockNumber(blockNumber), true, nil
}

func (b *BadgerDriver) FetchLatestProcessedBlockId(_ context.Context, id types.ShardId) (*types.BlockNumber, error) {
	var latestBlock *types.Block

	err := b.db.View(func(txn *badger.Txn) error {
		latestNumber, hasLatest, err := b.getShardLatestProcessedBlock(txn, id)
		if err != nil {
			return err
		}
		if !hasLatest {
			return nil
		}

		key := makeBlockKey(id, latestNumber)
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get latest block: %w", err)
		}

		err = item.Value(func(val []byte) error {
			var blockWithSSZ blockWithSSZ
			if err := json.Unmarshal(val, &blockWithSSZ); err != nil {
				return fmt.Errorf("failed to deserialize block: %w", err)
			}
			if blockWithSSZ.decoded != nil {
				latestBlock = blockWithSSZ.decoded.Block
			}
			return nil
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest processed block: %w", err)
	}
	if latestBlock == nil {
		result := types.InvalidBlockNumber
		return &result, nil
	}

	return &latestBlock.Id, nil
}

func (b *BadgerDriver) HaveBlock(ctx context.Context, id types.ShardId, number types.BlockNumber) (bool, error) {
	return true, nil
}

func (b *BadgerDriver) FetchEarliestAbsentBlockId(_ context.Context, id types.ShardId) (types.BlockNumber, error) {
	var earliestAbsent types.BlockNumber

	err := b.db.View(func(txn *badger.Txn) error {
		earliest, hasEarliest, err := b.getShardEarliestAbsentBlock(txn, id)
		if err != nil {
			return err
		}
		if hasEarliest {
			earliestAbsent = earliest
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch earliest absent block: %w", err)
	}

	return earliestAbsent, nil
}

func (b *BadgerDriver) FetchNextPresentBlockId(_ context.Context, id types.ShardId, number types.BlockNumber) (types.BlockNumber, error) {
	var nextPresent types.BlockNumber

	err := b.db.View(func(txn *badger.Txn) error {
		earliestAbsent, hasEarliest, err := b.getShardEarliestAbsentBlock(txn, id)
		if err != nil {
			return err
		}
		if hasEarliest && number < earliestAbsent {
			nextPresent = earliestAbsent - 1
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch next present block: %w", err)
	}

	return nextPresent, nil
}

func (b *BadgerDriver) createRoTx() *badger.Txn {
	return b.db.NewTransaction(false)
}

func (b *BadgerDriver) createRwTx() *badger.Txn {
	return b.db.NewTransaction(true)
}
