package client

import (
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

// DbClient defines the interface for read-only interaction with the database.
type DbClient interface {
	// TODO: Add batching and sanity checks
	DbInitTimestamp(ts uint64) error
	DbExists(tableName db.TableName, key []byte) (bool, error)
	DbGet(tableName db.TableName, key []byte) ([]byte, error)
	DbExistsInShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error)
	DbGetFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error)
}
