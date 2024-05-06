package db

import (
	"context"
)

type Tx interface {
	Exists(table string, key []byte) (bool, error)
	Get(table string, key []byte) (val *[]byte, err error)
	Put(table string, k, v []byte) error
	Delete(table string, k []byte) error
	Commit() error
	// Rollback can't really fail, because it's not clear how to proceed.
	// It's better to just panic in this case and restart.
	Rollback()
}

type DB interface {
	CreateTx(ctx context.Context) (Tx, error)
	Exists(table string, key []byte) (bool, error)
	Get(table string, key []byte) (*[]byte, error)
	Set(table string, key, value []byte) error
	Delete(table string, key []byte) error
	Close()
}
