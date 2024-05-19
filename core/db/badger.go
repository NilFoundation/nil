package db

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/core/types"
	"github.com/dgraph-io/badger/v4"
)

type BadgerDB struct {
	db *badger.DB
}

// interfaces
var _ DB = new(BadgerDB)

type BadgerTx struct {
	tx *badger.Txn
}

// interfaces
var _ Tx = new(BadgerTx)

func makeKey(table TableName, key []byte) []byte {
	return append([]byte(table+":"), key...)
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

func (k *BadgerDB) Exists(tableName TableName, key []byte) (bool, error) {
	var exists bool
	err := k.db.View(
		func(tx *badger.Txn) error {
			if val, err := tx.Get(makeKey(tableName, key)); err != nil {
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

func (k *BadgerDB) Get(tableName TableName, key []byte) (*[]byte, error) {
	var value *[]byte
	err := k.db.View(
		func(tx *badger.Txn) error {
			item, err := tx.Get(makeKey(tableName, key))
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
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrKeyNotFound
	}
	return value, nil
}

func (k *BadgerDB) Put(tableName TableName, key, value []byte) error {
	return k.db.Update(
		func(txn *badger.Txn) error {
			return txn.Set(makeKey(tableName, key), value)
		})
}

func (k *BadgerDB) Delete(tableName TableName, key []byte) error {
	return k.db.Update(
		func(txn *badger.Txn) error {
			return txn.Delete(makeKey(tableName, key))
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

func (db *BadgerDB) ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
	return db.Exists(shardTableName(tableName, shardId), key)
}

func (db *BadgerDB) GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (*[]byte, error) {
	return db.Get(shardTableName(tableName, shardId), key)
}

func (db *BadgerDB) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return db.Put(shardTableName(tableName, shardId), key, value)
}

func (db *BadgerDB) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return db.Delete(shardTableName(tableName, shardId), key)
}

func (tx *BadgerTx) Commit() error {
	return tx.tx.Commit()
}

func (tx *BadgerTx) Rollback() {
	tx.tx.Discard()
}

func (tx *BadgerTx) Put(tableName TableName, key, value []byte) error {
	return tx.tx.Set(makeKey(tableName, key), value)
}

func (tx *BadgerTx) Get(tableName TableName, key []byte) (val *[]byte, err error) {
	var res *[]byte
	item, err := tx.tx.Get(makeKey(tableName, key))
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

func (tx *BadgerTx) Exists(tableName TableName, key []byte) (bool, error) {
	_, err := tx.tx.Get(makeKey(tableName, key))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (tx *BadgerTx) Delete(tableName TableName, key []byte) error {
	return tx.tx.Delete(makeKey(tableName, key))
}

func (tx *BadgerTx) ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
	return tx.Exists(shardTableName(tableName, shardId), key)
}

func (tx *BadgerTx) GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (*[]byte, error) {
	return tx.Get(shardTableName(tableName, shardId), key)
}

func (tx *BadgerTx) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return tx.Put(shardTableName(tableName, shardId), key, value)
}

func (tx *BadgerTx) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return tx.Delete(shardTableName(tableName, shardId), key)
}
