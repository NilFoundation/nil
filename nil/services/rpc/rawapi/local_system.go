package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func (api *LocalShardApi) GasPrice(ctx context.Context) (types.Value, error) {
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
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot get gas price: %w", err)
	}
	return types.Value{Uint256: &param.Shards[api.ShardId]}, nil
}

func (api *LocalShardApi) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	if api.ShardId != types.MainShardId {
		return nil, errors.New("GetShardIdList is only supported for the main shard")
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, _, err := db.ReadLastBlock(tx, types.MainShardId)
	if err != nil {
		return nil, err
	}

	treeShards := execution.NewDbShardBlocksTrieReader(tx, types.MainShardId, block.Id)
	treeShards.SetRootHash(block.ChildBlocksRootHash)
	return treeShards.Keys()
}
