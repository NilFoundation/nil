package shardchain

import (
	"context"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

type BlockGenerator interface {
	CreateRoTx(ctx context.Context) (db.RoTx, error)
	CreateRwTx(ctx context.Context) (db.RwTx, error)
}

type ShardChain struct {
	Id types.ShardId
	db db.DB
}

var _ BlockGenerator = new(ShardChain)

func (c *ShardChain) CreateRoTx(ctx context.Context) (db.RoTx, error) {
	return c.db.CreateRoTx(ctx)
}

func (c *ShardChain) CreateRwTx(ctx context.Context) (db.RwTx, error) {
	return c.db.CreateRwTx(ctx)
}

func NewShardChain(
	shardId types.ShardId,
	db db.DB,
) *ShardChain {
	return &ShardChain{Id: shardId, db: db}
}
