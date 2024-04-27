package db

import (
	"context"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"sync/atomic"
)

type SqliteDB struct {
	closed atomic.Bool
	path   string
	db     *sql.DB
}

type SqliteTx struct {
	tx *sql.Tx
}

func (db *SqliteDB) Close() {
	if ok := db.closed.CompareAndSwap(false, true); !ok {
		return
	}

	db.Close()
}

func (db *SqliteDB) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}

	return &SqliteTx{tx: tx}, nil
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
		log.Fatal(err)
	}
}

func (tx *SqliteTx) Put(table string, key []byte, value []byte) error {
	stmt, err := tx.tx.Prepare("insert into kv values(?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(table, key, value)
	if err != nil {
		return err
	}
	return nil
}

func (tx *SqliteTx) GetOne(table string, key []byte) (val []byte, err error) {
	stmt, err := tx.tx.Prepare("select (value) from kv where tbl = ? and key = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	var value []byte
	err = stmt.QueryRow(table, key).Scan(&value)

	if err != nil {
		return nil, err
	}

	return value, nil
}

func (tx *SqliteTx) Has(table string, key []byte) (bool, error) {
	_, err := tx.GetOne(table, key)
	if err != nil {
		if err != sql.ErrNoRows {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (tx *SqliteTx) Delete(table string, key []byte) error {
	stmt, err := tx.tx.Prepare("delete from kv where tbl = ? and key = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(table, key)
	if err != nil {
		return err
	}

	return nil
}

func NewSqlite(path string) DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal(err)
	}

	create_table := "CREATE TABLE IF NOT EXISTS kv (tbl TEXT NOT NULL, key BLOB NOT NULL, value BLOB NOT NULL, PRIMARY KEY (tbl, key))"
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare(create_table)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	return &SqliteDB{path: path, db: db}
}
