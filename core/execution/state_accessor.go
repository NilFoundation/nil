package execution

import (
	"errors"
	"fmt"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

var ErrNotFound = errors.New("not found")

type StateAccessor struct {
	blocksLRU   *lru.Cache[common.Hash, *types.Block]
	messagesLRU *lru.Cache[common.Hash, []*types.Message]
	receiptsLRU *lru.Cache[common.Hash, []*types.Receipt]
}

func NewStateAccessor() (*StateAccessor, error) {
	const (
		blocksLRUSize   = 128 // ~32Mb
		messagesLRUSize = 32
		receiptsLRUSize = 32
	)

	blocksLRU, err := lru.New[common.Hash, *types.Block](blocksLRUSize)
	if err != nil {
		return nil, err
	}
	messagesLRU, err := lru.New[common.Hash, []*types.Message](messagesLRUSize)
	if err != nil {
		return nil, err
	}
	receiptsLRU, err := lru.New[common.Hash, []*types.Receipt](receiptsLRUSize)
	if err != nil {
		return nil, err
	}
	return &StateAccessor{
		blocksLRU:   blocksLRU,
		messagesLRU: messagesLRU,
		receiptsLRU: receiptsLRU,
	}, nil
}

func (s *StateAccessor) GetBlockByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) *types.Block {
	block, ok := s.blocksLRU.Get(hash)
	if ok {
		return block
	}
	block = db.ReadBlock(tx, shardId, hash)
	if block != nil {
		// We should not cache (at least without TTL) missing blocks because they may appear in the future.
		s.blocksLRU.Add(hash, block)
	}
	return block
}

func (s *StateAccessor) GetBlockByNumber(
	tx db.RoTx, shardId types.ShardId, number types.BlockNumber,
) (*types.Block, error) {
	hash, err := db.ReadBlockHashByNumber(tx, shardId, number)
	if err != nil {
		return nil, err
	}
	return s.GetBlockByHash(tx, shardId, hash), nil
}

func (s *StateAccessor) GetBlockTransactionCountByHash(
	tx db.RoTx, shardId types.ShardId, hash common.Hash,
) (int, error) {
	_, messages, _, err := s.GetBlockWithCollectedEntitiesByHash(tx, shardId, hash)
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

func (s *StateAccessor) GetBlockTransactionCountByNumber(
	tx db.RoTx, shardId types.ShardId, number types.BlockNumber,
) (int, error) {
	hash, err := db.ReadBlockHashByNumber(tx, shardId, number)
	if err != nil {
		return 0, err
	}
	return s.GetBlockTransactionCountByHash(tx, shardId, hash)
}

func (s *StateAccessor) getBlockEntities(tx db.RoTx, shardId types.ShardId, block *types.Block) ([]*types.Message, []*types.Receipt, error) {
	hash := block.Hash()
	messages, messagesCached := s.messagesLRU.Get(hash)
	if !messagesCached {
		messages, err := CollectBlockEntities[*types.Message](tx, shardId, db.MessageTrieTable, block.InMessagesRoot)
		if err != nil {
			return nil, nil, err
		}
		s.messagesLRU.Add(hash, messages)
	}

	receipts, receiptsCached := s.receiptsLRU.Get(hash)
	if !receiptsCached {
		receipts, err := CollectBlockEntities[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot)
		if err != nil {
			return nil, nil, err
		}
		s.receiptsLRU.Add(hash, receipts)
	}

	return messages, receipts, nil
}

func (s *StateAccessor) GetBlockWithCollectedEntitiesByHash(
	tx db.RoTx, shardId types.ShardId, hash common.Hash) (
	*types.Block, []*types.Message, []*types.Receipt, error,
) {
	block := s.GetBlockByHash(tx, shardId, hash)
	if block == nil {
		return nil, nil, nil, nil
	}
	msg, receipt, err := s.getBlockEntities(tx, shardId, block)
	return block, msg, receipt, err
}

func (s *StateAccessor) GetBlockWithCollectedEntitiesByNumber(
	tx db.RoTx, shardId types.ShardId, number types.BlockNumber) (
	*types.Block, []*types.Message, []*types.Receipt, error,
) {
	block, err := s.GetBlockByNumber(tx, shardId, number)
	if err != nil {
		return nil, nil, nil, err
	}
	msg, receipt, err := s.getBlockEntities(tx, shardId, block)
	return block, msg, receipt, err
}

func CollectBlockEntities[
	T interface {
		~*S
		ssz.Unmarshaler
	},
	S any,
](tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash) ([]*S, error) {
	root := mpt.NewReaderWithRoot(tx, shardId, tableName, rootHash)

	entities := make([]*S, 0, 1024)
	var index uint64
	for {
		k := ssz.MarshalUint64(nil, index)

		entity, err := mpt.GetEntity[T](root, k)
		if errors.Is(err, db.ErrKeyNotFound) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to get from %v with index %v from trie: %w", tableName, index, err)
		}
		entities = append(entities, entity)
		index += 1
	}
	return entities, nil
}

func (s *StateAccessor) GetBlockAndMessageIndexByMessageHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, db.BlockHashAndMessageIndex, error) {
	value, err := tx.GetFromShard(shardId, db.BlockHashAndMessageIndexByMessageHash, hash.Bytes())
	if err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}

	var blockHashAndMessageIndex db.BlockHashAndMessageIndex
	if err := blockHashAndMessageIndex.UnmarshalSSZ(*value); err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}

	block := s.GetBlockByHash(tx, shardId, blockHashAndMessageIndex.BlockHash)
	if block == nil {
		return nil, db.BlockHashAndMessageIndex{}, ErrNotFound
	}
	return block, blockHashAndMessageIndex, nil
}

func (s *StateAccessor) GetMessageWithEntitiesByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (msg *types.Message, receipt *types.Receipt, index types.MessageIndex, block *types.Block, err error) {
	block, indexes, err := s.GetBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		err = nil
		return
	}
	if err != nil {
		return
	}
	index = indexes.MessageIndex

	var root common.Hash
	if indexes.Outgoing {
		root = block.OutMessagesRoot
	} else {
		root = block.InMessagesRoot
	}

	msg, err = getBlockEntity[*types.Message](tx, shardId, db.MessageTrieTable, root, indexes.MessageIndex.Bytes())
	if msg == nil || err != nil {
		return
	}
	common.Require(msg.Hash() == hash)
	receipt, err = getBlockEntity[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, indexes.MessageIndex.Bytes())
	return
}

func getBlockEntity[
	T interface {
		~*S
		ssz.Unmarshaler
	},
	S any,
](tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte) (*S, error) {
	root := mpt.NewReaderWithRoot(tx, shardId, tableName, rootHash)
	return mpt.GetEntity[T](root, entityKey)
}
