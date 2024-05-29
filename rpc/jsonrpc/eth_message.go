package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	fastssz "github.com/ferranbt/fastssz"
)

// GetInMessageByHash implements eth_getTransactioByHash. Returns the message structure
func (api *APIImpl) GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Message, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block, messageIndex, err := getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	// TODO: make MessageIndex strong typed and use MarshalSSZ
	return getBlockEntity[*types.Message](tx, shardId, db.MessageTrieTable, block.InMessagesRoot, fastssz.MarshalUint64(nil, messageIndex))
}

func getBlockAndMessageIndexByMessageHash(tx db.Tx, shardId types.ShardId, hash common.Hash) (*types.Block, uint64, error) {
	value, err := tx.GetFromShard(shardId, db.BlockHashAndMessageIndexByMessageHash, hash.Bytes())
	if err != nil {
		return nil, 0, err
	}

	var blockHashAndMessageIndex db.BlockHashAndMessageIndex
	if err := blockHashAndMessageIndex.UnmarshalSSZ(*value); err != nil {
		return nil, 0, err
	}

	block := db.ReadBlock(tx, shardId, blockHashAndMessageIndex.BlockHash)
	if block == nil {
		return nil, 0, errors.New("Block not found")
	}
	return block, blockHashAndMessageIndex.MessageIndex, nil
}

func getBlockEntity[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
](tx db.Tx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte) (*S, error) {
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, tableName, rootHash)
	return mpt.GetEntity[T](root, entityKey)
}
