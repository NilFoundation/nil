package db

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/NilFoundation/badger/v4"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/rs/zerolog/log"
)

type badgerDB struct {
	db       *badger.DB
	txLedger assert.TxLedger
	lock     sync.Mutex
}

type BadgerDBOptions struct {
	Path         string        `yaml:"path"`
	DiscardRatio float64       `yaml:"gcDiscardRatio,omitempty"`
	GcFrequency  time.Duration `yaml:"gcFrequency,omitempty"`
	AllowDrop    bool          `yaml:"allowDrop,omitempty"`
}

func NewDefaultBadgerDBOptions() *BadgerDBOptions {
	return &BadgerDBOptions{
		Path:         "test.db",
		DiscardRatio: 0.5,
		GcFrequency:  time.Hour,
	}
}

type BadgerRoTx struct {
	tx       *badger.Txn
	onFinish assert.TxFinishCb
	managed  bool
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
	_ DB   = new(badgerDB)
	_ Iter = new(BadgerIter)
)

func MakeKey(table TableName, key []byte) []byte {
	return append([]byte(table+":"), key...)
}

func IsKeyFromTable(table TableName, fullKey []byte) bool {
	return bytes.HasPrefix(fullKey, []byte(table+":"))
}

func NewBadgerDb(pathToDb string) (*badgerDB, error) {
	opts := badger.DefaultOptions(pathToDb).WithLogger(nil)
	return newBadgerDb(&opts)
}

func NewBadgerDbInMemory() (*badgerDB, error) {
	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	return newBadgerDb(&opts)
}

func newBadgerDb(opts *badger.Options) (*badgerDB, error) {
	badgerInstance, err := badger.Open(*opts)
	if err != nil {
		return nil, err
	}

	db := &badgerDB{db: badgerInstance, txLedger: assert.NewTxLedger()}
	return db, nil
}

func (db *badgerDB) Close() {
	db.db.Close()
	db.txLedger.CheckLeakyTransactions()
}

func (db *badgerDB) DropAll() error {
	return db.db.DropAll()
}

func captureStacktrace() []byte {
	stack := make([]byte, 1024)
	_ = runtime.Stack(stack, false)
	return stack
}

func (db *badgerDB) createRoTx(_ context.Context, txn *badger.Txn, managed bool) (RoTx, error) {
	tx := &BadgerRoTx{tx: txn, onFinish: func() {}, managed: managed}
	if assert.Enable {
		stack := captureStacktrace()
		tx.onFinish = db.txLedger.TxOnStart(stack)
	}
	return tx, nil
}

func (db *badgerDB) CreateRoTxAt(ctx context.Context, ts Timestamp) (RoTx, error) {
	return db.createRoTx(ctx, db.db.NewTransactionAt(uint64(ts), false), true)
}

func (db *badgerDB) CreateRoTx(ctx context.Context) (RoTx, error) {
	return db.createRoTx(ctx, db.db.NewTransaction(false), false)
}

func (db *badgerDB) createRwTx(_ context.Context, txn *badger.Txn) (RwTx, error) {
	tx := &BadgerRwTx{&BadgerRoTx{tx: txn, onFinish: func() {}}}
	if assert.Enable {
		stack := captureStacktrace()
		tx.onFinish = db.txLedger.TxOnStart(stack)
	}
	return tx, nil
}

func (db *badgerDB) CreateRwTx(ctx context.Context) (RwTx, error) {
	return db.createRwTx(ctx, db.db.NewTransaction(true))
}

func (db *badgerDB) Stream(
	ctx context.Context, keyFilter func([]byte) bool, writer io.Writer,
) error {
	stream := db.db.NewStream()
	stream.NumGo = 4
	stream.ChooseKey = func(item *badger.Item) bool {
		return keyFilter(item.Key())
	}

	_, err := stream.Backup(writer, 0)
	if err != nil {
		return err
	}
	return nil
}

func (db *badgerDB) Fetch(_ context.Context, reader io.Reader) error {
	db.lock.Lock()
	defer db.lock.Unlock()
	const maxPendingWrites = 256
	return db.db.Load(reader, maxPendingWrites)
}

func (db *badgerDB) LogGC(ctx context.Context, discardRation float64, gcFrequency time.Duration) error {
	log.Info().Msg("Starting badger log garbage collection...")
	ticker := time.NewTicker(gcFrequency)
	for {
		select {
		case <-ticker.C:
			log.Debug().Msg("Execute badger LogGC")
			var err error
			for ; err == nil; err = db.db.RunValueLogGC(discardRation) {
			}
			if !errors.Is(err, badger.ErrNoRewrite) {
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
	tx.onFinish()
	return tx.tx.Commit()
}

func (tx *BadgerRwTx) CommitWithTs() (Timestamp, error) {
	tx.onFinish()
	ts, err := tx.tx.CommitWithTs()
	return Timestamp(ts), err
}

func (tx *BadgerRoTx) Rollback() {
	tx.onFinish()
	// Managed transaction can be only read-only.
	// We don't need to discard them, but if we do, than we will have a panic from badger.
	// It happens because badger tries to move watermarks, because it doesn't distinguish managed transactions in automatic mode
	// TODO: maybe we should patch badger to distinguish managed transactions in automatic mode
	if !tx.managed {
		tx.tx.Discard()
	}
}

func (tx *BadgerRoTx) ReadTimestamp() Timestamp {
	return Timestamp(tx.tx.ReadTs())
}

func (tx *BadgerRwTx) Put(tableName TableName, key, value []byte) error {
	return tx.tx.Set(MakeKey(tableName, key), value)
}

func (tx *BadgerRoTx) Get(tableName TableName, key []byte) ([]byte, error) {
	item, err := tx.tx.Get(MakeKey(tableName, key))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}

	return item.ValueCopy(nil)
}

func (tx *BadgerRoTx) Exists(tableName TableName, key []byte) (bool, error) {
	_, err := tx.tx.Get(MakeKey(tableName, key))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (tx *BadgerRwTx) Delete(tableName TableName, key []byte) error {
	return tx.tx.Delete(MakeKey(tableName, key))
}

func (tx *BadgerRoTx) Range(tableName TableName, from []byte, to []byte) (Iter, error) {
	var iter BadgerIter
	iter.iter = tx.tx.NewIterator(badger.DefaultIteratorOptions)

	prefix := MakeKey(tableName, from)
	iter.iter.Seek(prefix)
	iter.tablePrefix = []byte(tableName + ":")
	if to != nil {
		iter.toPrefix = MakeKey(tableName, to)
	}

	return &iter, nil
}

func (tx *BadgerRoTx) ExistsInShard(shardId types.ShardId, tableName ShardedTableName, key []byte) (bool, error) {
	return tx.Exists(ShardTableName(tableName, shardId), key)
}

func (tx *BadgerRoTx) GetFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) ([]byte, error) {
	return tx.Get(ShardTableName(tableName, shardId), key)
}

func (tx *BadgerRwTx) PutToShard(shardId types.ShardId, tableName ShardedTableName, key, value []byte) error {
	return tx.Put(ShardTableName(tableName, shardId), key, value)
}

func (tx *BadgerRwTx) DeleteFromShard(shardId types.ShardId, tableName ShardedTableName, key []byte) error {
	return tx.Delete(ShardTableName(tableName, shardId), key)
}

func (tx *BadgerRoTx) RangeByShard(shardId types.ShardId, tableName ShardedTableName, from []byte, to []byte) (Iter, error) {
	return tx.Range(ShardTableName(tableName, shardId), from, to)
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
	defer it.iter.Next() // Item() result is only valid until it.Next() gets called
	item := it.iter.Item()
	// *Copy methods prevent from deadlocks during iteration with updates, not sure if they are required here
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
