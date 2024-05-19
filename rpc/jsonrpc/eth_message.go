package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

// GetMessageByHash implements eth_getTransactioByHash. Returns the message structure
func (api *APIImpl) GetMessageByHash(ctx context.Context, hash common.Hash) (*types.Message, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	// TODO: shardId
	shardId := types.MasterShardId
	return db.ReadMessage(tx, shardId, hash), nil
}
