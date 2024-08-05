package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func (api *APIImpl) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, err := db.ReadLastBlock(tx, types.MainShardId)
	if err != nil {
		return nil, err
	}

	treeShards := execution.NewDbShardBlocksTrieReader(tx, types.MainShardId, block.Id)
	treeShards.SetRootHash(block.ChildBlocksRootHash)
	return treeShards.Keys()
}
