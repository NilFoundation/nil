package badger

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
	indexertypes "github.com/NilFoundation/nil/nil/services/indexer/types"
	"github.com/dgraph-io/badger/v4"
)

type BadgerDriver struct {
	db *badger.DB
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

func (b *BadgerDriver) IndexBlocks(_ context.Context, blocksToIndex []*types.BlockWithShardId) error {
	tx := b.createRwTx()
	defer tx.Discard()

	blocks := make([]types.BlockWithSSZ, len(blocksToIndex))
	receipts := make(map[common.Hash]types.ReceiptWithSSZ)

	shardLatest := make(map[types.ShardId]types.BlockNumber)

	for blockIndex, block := range blocksToIndex {
		sszEncodedBlock, err := block.EncodeSSZ()
		if err != nil {
			return fmt.Errorf("failed to encode block: %w", err)
		}
		blocks[blockIndex] = types.BlockWithSSZ{Decoded: block}

		for receiptIndex, receipt := range block.Receipts {
			receipts[receipt.TxnHash] = types.ReceiptWithSSZ{
				Decoded:    receipt,
				SszEncoded: sszEncodedBlock.Receipts[receiptIndex],
			}
		}

		if current, exists := shardLatest[block.ShardId]; !exists || block.Block.Id > current {
			shardLatest[block.ShardId] = block.Block.Id
		}

		key := makeBlockKey(block.ShardId, block.Block.Id)
		value, err := blocks[blockIndex].MarshalSSZ()
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

func (b *BadgerDriver) indexBlockTransactions(
	tx *badger.Txn,
	block *types.BlockWithShardId,
	receipts map[common.Hash]types.ReceiptWithSSZ,
) error {
	for _, txn := range block.InTransactions {
		receipt, exists := receipts[txn.Hash()]
		if !exists {
			return fmt.Errorf("receipt not found for transaction %s", txn.Hash())
		}

		baseAction := indexertypes.AddressAction{
			Hash:      txn.Hash(),
			From:      txn.From,
			To:        txn.To,
			Amount:    txn.Value,
			Timestamp: db.Timestamp(block.Block.Timestamp),
			BlockId:   block.Block.Id,
			Status:    getTransactionStatus(receipt.Decoded),
		}

		logger := logging.NewLogger("indexer-badger")
		logger.Debug().Msgf("indexing block transaction %s, from %s to %s", txn.Hash(), txn.From, txn.To)

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

func getTransactionStatus(receipt *types.Receipt) indexertypes.AddressActionStatus {
	if receipt.Success {
		return indexertypes.Success
	}
	return indexertypes.Failed
}

func storeAddressAction(tx *badger.Txn, address types.Address, action *indexertypes.AddressAction) error {
	key := makeAddressActionKey(address, uint64(action.Timestamp), action.Hash)
	value, err := action.MarshalSSZ()
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

func (b *BadgerDriver) FetchAddressActions(
	address types.Address,
	since db.Timestamp,
) ([]indexertypes.AddressAction, error) {
	actions := make([]indexertypes.AddressAction, 0)
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
				var action indexertypes.AddressAction
				if err := action.UnmarshalSSZ(val); err != nil {
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
			var blockWithSSZ types.BlockWithSSZ
			if err := blockWithSSZ.UnmarshalSSZ(val); err != nil {
				return fmt.Errorf("failed to deserialize block: %w", err)
			}
			block = blockWithSSZ.Decoded.Block
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

func (b *BadgerDriver) updateShardLatestProcessedBlock(
	tx *badger.Txn,
	shardId types.ShardId,
	blockNumber types.BlockNumber,
) error {
	key := makeShardLatestProcessedKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Set(key, value)
}

func (b *BadgerDriver) updateShardEarliestAbsentBlock(
	tx *badger.Txn,
	shardId types.ShardId,
	blockNumber types.BlockNumber,
) error {
	key := makeShardEarliestAbsentKey(shardId)
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(blockNumber))
	return tx.Set(key, value)
}

func (b *BadgerDriver) getShardLatestProcessedBlock(
	tx *badger.Txn,
	shardId types.ShardId,
) (types.BlockNumber, bool, error) {
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

func (b *BadgerDriver) getShardEarliestAbsentBlock(
	tx *badger.Txn,
	shardId types.ShardId,
) (types.BlockNumber, bool, error) {
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
			var blockWithSSZ types.BlockWithSSZ
			if err := blockWithSSZ.UnmarshalSSZ(val); err != nil {
				return fmt.Errorf("failed to deserialize block: %w", err)
			}
			if blockWithSSZ.Decoded != nil {
				latestBlock = blockWithSSZ.Decoded.Block
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

func (b *BadgerDriver) FetchNextPresentBlockId(
	_ context.Context,
	id types.ShardId,
	number types.BlockNumber,
) (types.BlockNumber, error) {
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

func (b *BadgerDriver) createRwTx() *badger.Txn {
	return b.db.NewTransaction(true)
}
