package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func (api *localShardApiRo) GasPrice(ctx context.Context) (types.Value, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open tx: %w", err)
	}
	defer tx.Rollback()

	cfg, err := config.NewConfigReader(tx, nil)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open config accessor: %w", err)
	}
	param, err := config.GetParamGasPrice(cfg)
	if err != nil || len(param.Shards) <= int(api.shardId()) {
		return types.Value{}, fmt.Errorf("cannot get gas price: %w", err)
	}
	return types.Value{Uint256: &param.Shards[api.shardId()]}, nil
}

func (api *localShardApiRo) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	if api.shardId() != types.MainShardId {
		return nil, errors.New("GetShardIdList is only supported for the main shard")
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	blockHash, err := db.ReadLastBlockHash(tx, types.MainShardId)
	if err != nil {
		return nil, err
	}

	block, err := api.accessor.Access(tx, types.MainShardId).GetBlock().ByHash(blockHash)
	if err != nil {
		return nil, err
	}

	treeShards := execution.NewDbShardBlocksTrieReader(tx, types.MainShardId, block.Block().Id)
	if err := treeShards.SetRootHash(block.Block().ChildBlocksRootHash); err != nil {
		return nil, err
	}
	return treeShards.Keys()
}

func (api *localShardApiRo) GetNumShards(ctx context.Context) (uint64, error) {
	shards, err := api.GetShardIdList(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(len(shards) + 1), nil
}
