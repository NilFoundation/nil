package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
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

	treeShards := execution.NewShardBlocksTrieReader(
		mpt.NewReaderWithRoot(tx, types.MainShardId, db.ShardBlocksTrieTableName(block.Id), block.ChildBlocksRootHash))
	return treeShards.Keys()
}
