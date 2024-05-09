package db

import (
	"context"
	"errors"

	"github.com/dgraph-io/badger/v4"
)

type BadgerDB struct {
	db *badger.DB
}

type BadgerTx struct {
	tx *badger.Txn
}

func makeKey(table string, key []byte) []byte {
	return append([]byte(table), key...)
}

func NewBadgerDb(pathToDb string) (*BadgerDB, error) {
	opts := badger.DefaultOptions(pathToDb).WithLogger(nil)
	return newBadgerDb(&opts)
}

func NewBadgerDbInMemory() (*BadgerDB, error) {
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	return newBadgerDb(&opts)
}

func newBadgerDb(opts *badger.Options) (*BadgerDB, error) {
	badgerInstance, err := badger.Open(*opts)
	if err != nil {
		return nil, err
	}
	return &BadgerDB{db: badgerInstance}, nil
}

func (k *BadgerDB) Close() {
	k.db.Close()
}

func (k *BadgerDB) Exists(table string, key []byte) (bool, error) {
	var exists bool
	err := k.db.View(
		func(tx *badger.Txn) error {
			if val, err := tx.Get(makeKey(table, key)); err != nil {
				return err
			} else if val != nil {
				exists = true
			}
			return nil
		})
	if errors.Is(err, badger.ErrKeyNotFound) {
		err = nil
	}
	return exists, err
}

func (k *BadgerDB) Get(table string, key []byte) (*[]byte, error) {
	var value *[]byte
	err := k.db.View(
		func(tx *badger.Txn) error {
			item, err := tx.Get(makeKey(table, key))
			if err != nil {
				return err
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			val = append([]byte{}, val...)
			value = &val
			return nil
		})
	return value, err
}

func (k *BadgerDB) Put(table string, key, value []byte) error {
	return k.db.Update(
		func(txn *badger.Txn) error {
			return txn.Set(makeKey(table, key), value)
		})
}

func (k *BadgerDB) Delete(table string, key []byte) error {
	return k.db.Update(
		func(txn *badger.Txn) error {
			return txn.Delete(makeKey(table, key))
		})
}

func (k *BadgerDB) CreateRoTx(ctx context.Context) (Tx, error) {
	txn := k.db.NewTransaction(false)
	return &BadgerTx{tx: txn}, nil
}

func (k *BadgerDB) CreateRwTx(ctx context.Context) (Tx, error) {
	txn := k.db.NewTransaction(true)
	return &BadgerTx{tx: txn}, nil
}

func (tx *BadgerTx) Commit() error {
	return tx.tx.Commit()
}

func (tx *BadgerTx) Rollback() {
	tx.tx.Discard()
}

func (tx *BadgerTx) Put(table string, key []byte, value []byte) error {
	return tx.tx.Set(makeKey(table, key), value)
}

func (tx *BadgerTx) Get(table string, key []byte) (val *[]byte, err error) {
	var res *[]byte
	item, err := tx.tx.Get(makeKey(table, key))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return res, err
	}
	err = item.Value(func(val []byte) error {
		val = append([]byte{}, val...)
		res = &val
		return nil
	})
	return res, err
}

func (tx *BadgerTx) Exists(table string, key []byte) (bool, error) {
	_, err := tx.tx.Get(makeKey(table, key))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (tx *BadgerTx) Delete(table string, key []byte) error {
	return tx.tx.Delete(makeKey(table, key))
}
