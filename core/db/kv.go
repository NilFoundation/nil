package db

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/core/types"
)

type Timestamp uint64

type RoTx interface {
	Exists(tableName TableName, key []byte) (bool, error)
	Get(tableName TableName, key []byte) ([]byte, error)
	Range(tableName TableName, from []byte, to []byte) (Iter, error)

	ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error)
	GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) ([]byte, error)
	RangeByShard(shardId types.ShardId, tableName ShardedTableName, from []byte, to []byte) (Iter, error)

	ReadTimestamp() Timestamp

	Rollback()
}

type RwTx interface {
	RoTx

	Put(tableName TableName, key, value []byte) error
	Delete(tableName TableName, key []byte) error

	PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error
	DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error

	Commit() error
	CommitWithTs() (Timestamp, error)
}

type Iter interface {
	HasNext() bool
	Next() ([]byte, []byte, error)
	Close()
}

type ReadOnlyDB interface {
	CreateRoTx(ctx context.Context) (RoTx, error)
	CreateRoTxAt(ctx context.Context, ts Timestamp) (RoTx, error)
}

type DB interface {
	ReadOnlyDB

	CreateRwTx(ctx context.Context) (RwTx, error)

	DropAll() error
	LogGC(ctx context.Context, discardRation float64, gcFrequency time.Duration) error
	Close()
}

type RwWrapper struct {
	RoTx
}

var _ RwTx = new(RwWrapper)

func (a *RwWrapper) Put(TableName, []byte, []byte) error {
	panic("unsupported operation")
}

func (a *RwWrapper) Delete(TableName, []byte) error {
	panic("unsupported operation")
}

func (a *RwWrapper) PutToShard(types.ShardId, ShardedTableName, []byte, []byte) error {
	panic("unsupported operation")
}

func (a *RwWrapper) DeleteFromShard(types.ShardId, ShardedTableName, []byte) error {
	panic("unsupported operation")
}

func (a *RwWrapper) Commit() error {
	panic("unsupported operation")
}

func (a *RwWrapper) CommitWithTs() (Timestamp, error) {
	panic("unsupported operation")
}
