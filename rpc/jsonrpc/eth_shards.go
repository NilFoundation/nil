package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
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
		return make([]types.ShardId, 0), nil
	}

	treeShards := mpt.NewMerklePatriciaTrieWithRoot(tx, types.MasterShardId, db.ShardBlocksTrieTableName(block.Id), block.ChildBlocksRootHash)

	chanShards := treeShards.Iterate()
	shards := make([]types.ShardId, 0)
	for shard := range chanShards {
		shardNumId, err := types.ParseShardIdFromString(string(shard.Key))
		if err != nil {
			return nil, err
		}
		shards = append(shards, shardNumId)
	}
	return shards, nil
}
