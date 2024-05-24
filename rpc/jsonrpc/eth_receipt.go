//nolint:dupl
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

func (api *APIImpl) GetMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Receipt, error) {
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

	mptReceipts := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot)
	receiptBytes, err := mptReceipts.Get(fastssz.MarshalUint64(nil, blockHashAndMessageIndex.MessageIndex))
	if err != nil {
		return nil, err
	}

	var receipt types.Receipt
	if err := receipt.UnmarshalSSZ(receiptBytes); err != nil {
		return nil, err
	}
	return &receipt, nil
}
