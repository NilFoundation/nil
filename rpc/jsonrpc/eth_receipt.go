package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

func (api *APIImpl) GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCReceipt, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block, indexes, err := api.getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	receipt, err := getBlockEntity[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, indexes.MessageIndex.Bytes())
	if err != nil {
		return nil, err
	}
	return NewRPCReceipt(block, indexes.MessageIndex, receipt), nil
}
