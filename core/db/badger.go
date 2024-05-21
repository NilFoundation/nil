package db

import (
	"bytes"
	"context"
	"errors"

	"github.com/NilFoundation/nil/core/types"
	"github.com/dgraph-io/badger/v4"
)

type BadgerDB struct {
	db *badger.DB
}

type BadgerRoTx struct {
	tx *badger.Txn
}

type BadgerRwTx struct {
	BadgerRoTx
}

type BadgerIter struct {
	iter        *badger.Iterator
	tablePrefix []byte
	toPrefix    []byte
}

// interfaces
var (
	_ RoTx = new(BadgerRoTx)
	_ RwTx = new(BadgerRwTx)
	_ DB   = new(BadgerDB)
	_ Iter = new(BadgerIter)
)

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

func (k *BadgerDB) Range(table TableName, from []byte, to []byte) (Iter, error) {
	return nil, ErrNotImplemented
}

func (k *BadgerDB) CreateRoTx(ctx context.Context) (RoTx, error) {
	txn := k.db.NewTransaction(false)
	return &BadgerRoTx{tx: txn}, nil
}

func (k *BadgerDB) CreateRwTx(ctx context.Context) (RwTx, error) {
	txn := k.db.NewTransaction(true)
	return &BadgerRwTx{BadgerRoTx{tx: txn}}, nil
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

func (db *BadgerDB) RangeByShard(shardId types.ShardId, tableName ShardedTableName, from []byte, to []byte) (Iter, error) {
	return db.Range(shardTableName(tableName, shardId), from, to)
}

func (tx *BadgerRoTx) Commit() error {
	return tx.tx.Commit()
}

func (tx *BadgerRoTx) Rollback() {
	tx.tx.Discard()
}

func (tx *BadgerRoTx) Put(tableName TableName, key, value []byte) error {
	return ErrNotImplemented
}

func (tx *BadgerRwTx) Put(tableName TableName, key, value []byte) error {
	return tx.tx.Set(makeKey(tableName, key), value)
}

func (tx *BadgerRoTx) Get(tableName TableName, key []byte) (val *[]byte, err error) {
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

func (tx *BadgerRoTx) Exists(tableName TableName, key []byte) (bool, error) {
	_, err := tx.tx.Get(makeKey(tableName, key))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (tx *BadgerRoTx) Delete(tableName TableName, key []byte) error {
	return ErrNotImplemented
}

func (tx *BadgerRwTx) Delete(tableName TableName, key []byte) error {
	return tx.tx.Delete(makeKey(tableName, key))
}

func (tx *BadgerRoTx) Range(tableName TableName, from []byte, to []byte) (Iter, error) {
	var iter BadgerIter
	iter.iter = tx.tx.NewIterator(badger.DefaultIteratorOptions)
	if iter.iter == nil {
		return nil, ErrIteratorCreate
	}

	prefix := makeKey(tableName, from)
	iter.iter.Seek(prefix)
	iter.tablePrefix = []byte(tableName + ":")
	if to != nil {
		iter.toPrefix = makeKey(tableName, to)
	}

	return &iter, nil
}

func (tx *BadgerRoTx) ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
	return tx.Exists(shardTableName(tableName, shardId), key)
}

func (tx *BadgerRoTx) GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (*[]byte, error) {
	return tx.Get(shardTableName(tableName, shardId), key)
}

func (tx *BadgerRoTx) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return ErrNotImplemented
}

func (tx *BadgerRwTx) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return tx.Put(shardTableName(tableName, shardId), key, value)
}

func (tx *BadgerRoTx) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return tx.Delete(shardTableName(tableName, shardId), key)
}

func (tx *BadgerRwTx) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return ErrNotImplemented
}

func (tx *BadgerRoTx) RangeByShard(shardId types.ShardId, tableName ShardedTableName, from []byte, to []byte) (Iter, error) {
	return tx.Range(shardTableName(tableName, shardId), from, to)
}

func (it *BadgerIter) HasNext() bool {
	if !it.iter.ValidForPrefix(it.tablePrefix) {
		return false
	}

	if it.toPrefix == nil {
		return true
	}

	if k := it.iter.Item().Key(); bytes.Compare(k, it.toPrefix) > 0 {
		return false
	}
	return true
}

func (it *BadgerIter) Next() ([]byte, []byte, error) {
	item := it.iter.Item()
	it.iter.Next()
	key := item.KeyCopy(nil)
	value, err := item.ValueCopy(nil)
	if err != nil {
		return nil, nil, err
	}
	return key[len(it.tablePrefix):], value, nil
}

func (it *BadgerIter) Close() {
	it.iter.Close()
}
