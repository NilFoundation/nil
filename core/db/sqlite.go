package db

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"

	"github.com/NilFoundation/nil/core/types"
	_ "github.com/mattn/go-sqlite3"
)

type SqliteDB struct {
	closed atomic.Bool
	path   string
	db     *sql.DB
}

// interfaces
var _ DB = new(SqliteDB)

type SqliteTx struct {
	tx  *sql.Tx
	ctx context.Context
}

// interfaces
var _ Tx = new(SqliteTx)

func (db *SqliteDB) Close() {
	if ok := db.closed.CompareAndSwap(false, true); !ok {
		return
	}

	db.Close()
}

func (db *SqliteDB) CreateRwTx(ctx context.Context) (Tx, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}

	return &SqliteTx{tx: tx, ctx: ctx}, nil
}

func (db *SqliteDB) CreateRoTx(ctx context.Context) (Tx, error) {
	tx, err := db.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}

	return &SqliteTx{tx: tx, ctx: ctx}, nil
}

func (db *SqliteDB) View(fn func(txn Tx) error) error {
	if db.closed.Load() {
		return sql.ErrConnDone
	}
	ctx := context.Background()
	tx, err := db.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	return fn(tx)
}

func (db *SqliteDB) Update(fn func(txn Tx) error) error {
	if db.closed.Load() {
		return sql.ErrConnDone
	}
	ctx := context.Background()
	tx, err := db.CreateRwTx(ctx)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *SqliteDB) Exists(tableName TableName, key []byte) (bool, error) {
	var exists bool
	err := db.View(
		func(tx Tx) error {
			if val, err := tx.Exists(tableName, key); err != nil {
				return err
			} else {
				exists = val
			}
			return nil
		})
	return exists, err
}

func (db *SqliteDB) Get(tableName TableName, key []byte) (*[]byte, error) {
	var value *[]byte
	return value, db.View(
		func(tx Tx) error {
			item, err := tx.Get(tableName, key)
			if err != nil {
				return err
			}
			value = item
			return nil
		})
}

func (db *SqliteDB) Put(tableName TableName, key, value []byte) error {
	return db.Update(
		func(txn Tx) error {
			return txn.Put(tableName, key, value)
		})
}

func (db *SqliteDB) Delete(tableName TableName, key []byte) error {
	return db.Update(
		func(txn Tx) error {
			return txn.Delete(tableName, key)
		})
}

func (db *SqliteDB) ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
	return db.Exists(shardTableName(tableName, shardId), key)
}

func (db *SqliteDB) GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (*[]byte, error) {
	return db.Get(shardTableName(tableName, shardId), key)
}

func (db *SqliteDB) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return db.Put(shardTableName(tableName, shardId), key, value)
}

func (db *SqliteDB) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return db.Delete(shardTableName(tableName, shardId), key)
}

func (tx *SqliteTx) Commit() error {
	if tx.tx == nil {
		return nil
	}
	defer func() {
		tx.tx = nil
	}()

	err := tx.tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (tx *SqliteTx) Rollback() {
	if tx.tx == nil {
		return
	}
	defer func() {
		tx.tx = nil
	}()
	err := tx.tx.Rollback()
	if err != nil {
		logger.Fatal().Msgf("Can't roll back transaction. err: %s", err)
	}
}

func (tx *SqliteTx) Put(tableName TableName, key, value []byte) error {
	stmt, err := tx.tx.Prepare("insert into kv values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(tx.ctx, tableName, key, value)
	return err
}

func (tx *SqliteTx) Get(tableName TableName, key []byte) (val *[]byte, err error) {
	stmt, err := tx.tx.Prepare("select (value) from kv where tbl = ? and key = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var value []byte
	if err = stmt.QueryRowContext(tx.ctx, tableName, key).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	return &value, nil
}

func (tx *SqliteTx) Exists(tableName TableName, key []byte) (bool, error) {
	_, err := tx.Get(tableName, key)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (tx *SqliteTx) Delete(tableName TableName, key []byte) error {
	stmt, err := tx.tx.Prepare("delete from kv where tbl = ? and key = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(tx.ctx, tableName, key)
	return err
}

func (tx *SqliteTx) ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
	return tx.Exists(shardTableName(tableName, shardId), key)
}

func (tx *SqliteTx) GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (*[]byte, error) {
	return tx.Get(shardTableName(tableName, shardId), key)
}

func (tx *SqliteTx) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return tx.Put(shardTableName(tableName, shardId), key, value)
}

func (tx *SqliteTx) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return tx.Delete(shardTableName(tableName, shardId), key)
}

func NewSqlite(path string) (*SqliteDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	createTable := "CREATE TABLE IF NOT EXISTS kv (tbl TEXT NOT NULL, key BLOB NOT NULL, value BLOB NOT NULL, PRIMARY KEY (tbl, key))"
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	stmt, err := tx.Prepare(createTable)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	if _, err = stmt.Exec(); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &SqliteDB{path: path, db: db}, nil
}
