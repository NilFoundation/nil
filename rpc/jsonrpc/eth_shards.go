package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/common"
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

	hash, err := tx.Get(db.LastBlockTable, types.MasterShardId.Bytes())
	if err != nil {
		return nil, err
	}

	block := db.ReadBlock(tx, types.MasterShardId, common.CastToHash(*hash))
	if block == nil {
		return nil, nil
	}

	treeShards := execution.NewShardBlocksTrie(mpt.NewMerklePatriciaTrieWithRoot(tx, types.MasterShardId, db.ShardBlocksTrieTableName(block.Id), block.ChildBlocksRootHash))
	return treeShards.Keys()
}
