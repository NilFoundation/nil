package db

import (
	"context"

	"github.com/NilFoundation/nil/core/types"
)

type DBAccessor interface {
	Exists(tableName TableName, key []byte) (bool, error)
	Get(tableName TableName, key []byte) (*[]byte, error)
	Put(tableName TableName, key, value []byte) error
	Delete(tableName TableName, key []byte) error

	ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error)
	GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (*[]byte, error)
	PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error
	DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error
}

type Tx interface {
	DBAccessor

	Commit() error
	// Rollback can't really fail, because it's not clear how to proceed.
	// It's better to just panic in this case and restart.
	Rollback()
}

type DB interface {
	DBAccessor

	CreateRwTx(ctx context.Context) (Tx, error)
	CreateRoTx(ctx context.Context) (Tx, error)
	Close()
}
