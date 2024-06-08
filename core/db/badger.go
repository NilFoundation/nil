package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/common/assert"
	"github.com/NilFoundation/nil/core/types"
	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
)

type BadgerDB struct {
	db *badger.DB
}

type BadgerDBOptions struct {
	Path         string
	DiscardRatio float64
	GcFrequency  time.Duration
	AllowDrop    bool
}

type BadgerRoTx struct {
	tx         *badger.Txn
	Terminated atomic.Bool
}

type BadgerRwTx struct {
	*BadgerRoTx
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

func (db *BadgerDB) Close() {
	db.db.Close()
}

func (db *BadgerDB) DropAll() error {
	return db.db.DropAll()
}

func captureStacktrace() []byte {
	stack := make([]byte, 1024)
	_ = runtime.Stack(stack, false)
	return stack
}

func runTxLeakChecker(tx *BadgerRoTx, stack []byte, timeout time.Duration) {
	time.Sleep(timeout)
	if !tx.Terminated.Load() {
		panic(fmt.Sprintf("Transaction wasn't terminated:\n%s", stack))
	}
}

func (db *BadgerDB) CreateRoTx(ctx context.Context) (RoTx, error) {
	txn := db.db.NewTransaction(false)
	tx := &BadgerRoTx{tx: txn}
	if assert.Enable {
		stack := captureStacktrace()
		go runTxLeakChecker(tx, stack, 1*time.Second)
	}
	return tx, nil
}

func (db *BadgerDB) CreateRwTx(ctx context.Context) (RwTx, error) {
	txn := db.db.NewTransaction(true)
	tx := &BadgerRwTx{&BadgerRoTx{tx: txn}}
	if assert.Enable {
		stack := captureStacktrace()
		go runTxLeakChecker(tx.BadgerRoTx, stack, 10*time.Second)
	}
	return tx, nil
}

func (db *BadgerDB) LogGC(ctx context.Context, discardRation float64, gcFrequency time.Duration) error {
	log.Info().Msg("Starting badger log garbage collection...")
	ticker := time.NewTicker(gcFrequency)
	for {
		select {
		case <-ticker.C:
			log.Debug().Msg("Execute badger LogGC")
			var err error
			for ; err == nil; err = db.db.RunValueLogGC(discardRation) {
			}
			if !errors.Is(badger.ErrNoRewrite, err) {
				log.Error().Err(err).Msg("Error during badger LogGC")
				return err
			}
		case <-ctx.Done():
			log.Info().Msg("Stopping badger log garbage collection...")
			return nil
		}
	}
}

func (tx *BadgerRwTx) Commit() error {
	tx.Terminated.Store(true)
	return tx.tx.Commit()
}

func (tx *BadgerRoTx) Rollback() {
	tx.Terminated.Store(true)
	tx.tx.Discard()
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

func (tx *BadgerRwTx) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return tx.Put(shardTableName(tableName, shardId), key, value)
}

func (tx *BadgerRwTx) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return tx.Delete(shardTableName(tableName, shardId), key)
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
