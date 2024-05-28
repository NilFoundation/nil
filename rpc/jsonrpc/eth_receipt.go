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

func (api *APIImpl) GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Receipt, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block, messageIndex, err := getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	mptReceipts := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot)
	receiptBytes, err := mptReceipts.Get(fastssz.MarshalUint64(nil, messageIndex))
	if err != nil {
		return nil, err
	}

	var receipt types.Receipt
	return &receipt, receipt.UnmarshalSSZ(receiptBytes)
}
