package db

import (
	"context"
	"database/sql"
	"sync/atomic"

	_ "github.com/mattn/go-sqlite3"
)


type SqliteDB struct {
	closed atomic.Bool
	path   string
	db     *sql.DB
}

type SqliteTx struct {
	tx *sql.Tx
	ctx context.Context
}

func (db *SqliteDB) Close() {
	if ok := db.closed.CompareAndSwap(false, true); !ok {
		return
	}

	db.Close()
}

func (db *SqliteDB) CreateTx(ctx context.Context) (Tx, error) {
	tx, err := db.db.Begin()
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
	tx, err := db.CreateTx(ctx)
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
	tx, err := db.CreateTx(ctx)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *SqliteDB) Exists(table string, key []byte) (bool, error) {
	var exists bool
	err := db.View(
		func(tx Tx) error {
			if val, err := tx.Exists(table, key); err != nil {
				return err
			} else {
				exists = val
			}
			return nil
		})
	return exists, err
}

func (db *SqliteDB) Get(table string, key []byte) (*[]byte, error) {
	var value *[]byte
	return value, db.View(
		func(tx Tx) error {
			item, err := tx.Get(table, key)
			if err != nil {
				return err
			}
			value = item
			return nil
		})
}

func (db *SqliteDB) Set(table string, key, value []byte) error {
	return db.Update(
		func(txn Tx) error {
			return txn.Put(table, key, value)
		})
}

func (db *SqliteDB) Delete(table string, key []byte) error {
	return db.Update(
		func(txn Tx) error {
			return txn.Delete(table, key)
		})
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

func (tx *SqliteTx) Put(table string, key []byte, value []byte) error {
	stmt, err := tx.tx.Prepare("insert into kv values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(tx.ctx, table, key, value)
	return err
}

func (tx *SqliteTx) Get(table string, key []byte) (val *[]byte, err error) {
	stmt, err := tx.tx.Prepare("select (value) from kv where tbl = ? and key = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var value []byte
	err = stmt.QueryRowContext(tx.ctx, table, key).Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &value, nil
}

func (tx *SqliteTx) Exists(table string, key []byte) (bool, error) {
	res, err := tx.Get(table, key)
	if err != nil {
		return false, err
	}
	return res != nil, nil
}

func (tx *SqliteTx) Delete(table string, key []byte) error {
	stmt, err := tx.tx.Prepare("delete from kv where tbl = ? and key = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(tx.ctx, table, key)
	return err
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
	_, err = stmt.Exec()
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &SqliteDB{path: path, db: db}, nil
}
