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

	value, err := tx.GetFromShard(shardId, db.BlockHashAndMessageIndexByMessageHash, hash.Bytes())
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var blockHashAndMessageIndex db.BlockHashAndMessageIndex
	if err := blockHashAndMessageIndex.UnmarshalSSZ(*value); err != nil {
		return nil, err
	}

	block := db.ReadBlock(tx, shardId, blockHashAndMessageIndex.BlockHash)
	if block == nil {
		return nil, errors.New("Block not found")
	}

	mptMessages := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.MessageTrieTable, block.MessagesRoot)
	messageBytes, err := mptMessages.Get(fastssz.MarshalUint64(nil, blockHashAndMessageIndex.MessageIndex))
	if err != nil {
		return nil, err
	}

	var message types.Message
	if err := message.UnmarshalSSZ(messageBytes); err != nil {
		return nil, err
	}
	return &message, nil
}
