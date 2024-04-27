package db

import (
	"context"
)


type Tx interface {

	Has(table string, key []byte) (bool, error)
	GetOne(table string, key []byte) (val []byte, err error)
	Put(table string, k, v []byte) error
	Delete(table string, k []byte) error

	Commit() error
	Rollback()

}


type DB interface {
	BeginTx(ctx context.Context) (Tx, error)

	Close()
}
