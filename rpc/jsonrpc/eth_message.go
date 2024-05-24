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

// GetMessageByHash implements eth_getTransactioByHash. Returns the message structure
func (api *APIImpl) GetMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Message, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block, messageIndex, err := getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	mptMessages := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.MessageTrieTable, block.MessagesRoot)
	messageBytes, err := mptMessages.Get(fastssz.MarshalUint64(nil, messageIndex))
	if err != nil {
		return nil, err
	}

	var message types.Message
	if err := message.UnmarshalSSZ(messageBytes); err != nil {
		return nil, err
	}
	return &message, nil
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
