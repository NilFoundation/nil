package client

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// DbClient defines the interface for read-only interaction with the database.
type DbClient interface {
	// TODO: Add batching and sanity checks
	DbInitTimestamp(ctx context.Context, ts uint64) error
	DbExists(ctx context.Context, tableName db.TableName, key []byte) (bool, error)
	DbGet(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error)
	DbExistsInShard(
		ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error)
	DbGetFromShard(
		ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error)
}
