package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

func (api *APIImpl) GetMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Receipt, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()
	return db.ReadReceipt(tx, shardId, hash), nil
}
