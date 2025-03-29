//go:build test

package db

import (
	"context"
	"io"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
)

func NewDbMock(dbImpl DB) *DBMock {
	dbMock := &DBMock{
		CreateRoTxFunc: func(ctx context.Context) (RoTx, error) {
			return dbImpl.CreateRoTx(ctx)
		},
		CreateRoTxAtFunc: func(ctx context.Context, ts Timestamp) (RoTx, error) {
			return dbImpl.CreateRoTxAt(ctx, ts)
		},
		StreamFunc: func(ctx context.Context, keyFilter func([]byte) bool, writer io.Writer) error {
			return dbImpl.Stream(ctx, keyFilter, writer)
		},
		CreateRwTxFunc: func(ctx context.Context) (RwTx, error) {
			return dbImpl.CreateRwTx(ctx)
		},
		DropAllFunc: func() error {
			return dbImpl.DropAll()
		},
		LogGCFunc: func(ctx context.Context, discardRation float64, gcFrequency time.Duration) error {
			return dbImpl.LogGC(ctx, discardRation, gcFrequency)
		},
		FetchFunc: func(ctx context.Context, reader io.Reader) error {
			return dbImpl.Fetch(ctx, reader)
		},
		CloseFunc: func() {
			dbImpl.Close()
		},
	}
	return dbMock
}

func NewTxMock(tx RwTx) *RwTxMock {
	txMock := &RwTxMock{
		ExistsFunc: func(tableName TableName, key []byte) (bool, error) {
			return tx.Exists(tableName, key)
		},
		GetFunc: func(tableName TableName, key []byte) ([]byte, error) {
			return tx.Get(tableName, key)
		},
		RangeFunc: func(tableName TableName, from []byte, to []byte) (Iter, error) {
			return tx.Range(tableName, from, to)
		},
		ExistsInShardFunc: func(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
			return tx.ExistsInShard(shardId, tableName, key)
		},
		GetFromShardFunc: func(shardId types.ShardId, tableName ShardedTableName, key []byte) ([]byte, error) {
			return tx.GetFromShard(shardId, tableName, key)
		},
		RangeByShardFunc: func(
			shardId types.ShardId, tableName ShardedTableName, from []byte, to []byte,
		) (Iter, error) {
			return tx.RangeByShard(shardId, tableName, from, to)
		},
		ReadTimestampFunc: func() Timestamp {
			return tx.ReadTimestamp()
		},
		RollbackFunc: func() {
			tx.Rollback()
		},
		PutFunc: func(tableName TableName, key, value []byte) error {
			return tx.Put(tableName, key, value)
		},
		DeleteFunc: func(tableName TableName, key []byte) error {
			return tx.Delete(tableName, key)
		},
		PutToShardFunc: func(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
			return tx.PutToShard(shardId, tableName, key, value)
		},
		DeleteFromShardFunc: func(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
			return tx.DeleteFromShard(shardId, tableName, key)
		},
		CommitFunc: func() error {
			return tx.Commit()
		},
		CommitWithTsFunc: func() (Timestamp, error) {
			return tx.CommitWithTs()
		},
	}
	return txMock
}
