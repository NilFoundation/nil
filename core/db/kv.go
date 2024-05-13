package db

import (
	"context"
)

type DBAccessor interface {
	Exists(table string, key []byte) (bool, error)
	Get(table string, key []byte) (*[]byte, error)
	Put(table string, key, value []byte) error
	Delete(table string, key []byte) error
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
