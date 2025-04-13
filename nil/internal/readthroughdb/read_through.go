package readthroughdb

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// TODO: add tombstones and in-memory concurrent cache
type ReadThroughDb struct {
	client    client.DbClient
	db        db.DB
	cache     sync.Map
	cacheTomb sync.Map
}

type RoTx struct {
	// Using db.RwTx and RwWrapper to avoid implementing all the methods of db.RoTx in RwTx
	tx        db.RwTx
	cache     *sync.Map
	cacheTomb *sync.Map
	client    client.DbClient
}

var (
	_ db.RoTx = (*RoTx)(nil)
	_ db.DB   = (*ReadThroughDb)(nil)
	_ db.RwTx = (*RwTx)(nil)
)

const TombstoneTableName = db.TableName("__TOMBSTONES__")

func (tx *RoTx) Get(tableName db.TableName, key []byte) ([]byte, error) {
	value, err := tx.tx.Get(tableName, key)
	if err == nil {
		return value, nil
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}
	tombExists, err := tx.tx.Exists(TombstoneTableName, db.MakeKey(tableName, key))
	if err != nil {
		return nil, err
	}
	if tombExists {
		return nil, db.ErrKeyNotFound
	}
	_, cacheTombExists := tx.cacheTomb.Load(string(db.MakeKey(tableName, key)))
	if cacheTombExists {
		return nil, db.ErrKeyNotFound
	}
	ivalue, ok := tx.cache.Load(string(db.MakeKey(tableName, key)))
	if ok {
		value, ok = ivalue.([]byte)
		check.PanicIfNot(ok)
		return value, nil
	}
	value, err = tx.client.DbGet(context.Background(), tableName, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		tx.cacheTomb.Store(string(db.MakeKey(tableName, key)), nil)
		return nil, db.ErrKeyNotFound
	} else if err != nil {
		return nil, err
	}
	tx.cache.Store(string(db.MakeKey(tableName, key)), value)
	return value, nil
}

func (tx *RoTx) Exists(tableName db.TableName, key []byte) (bool, error) {
	_, err := tx.Get(tableName, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (tx *RoTx) ReadTimestamp() db.Timestamp {
	return tx.tx.ReadTimestamp()
}

func (tx *RoTx) Range(tableName db.TableName, from []byte, to []byte) (db.Iter, error) {
	// TODO: Implement this when we will actually need ranges
	panic("implement me")
}

func (tx *RoTx) ExistsInShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	return tx.Exists(db.ShardTableName(tableName, shardId), key)
}

func (tx *RoTx) GetFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	return tx.Get(db.ShardTableName(tableName, shardId), key)
}

func (tx *RoTx) RangeByShard(
	shardId types.ShardId,
	tableName db.ShardedTableName,
	from []byte,
	to []byte,
) (db.Iter, error) {
	// TODO: Implement this when we will actually need ranges
	panic("implement me")
}

type RwTx struct {
	*RoTx
}

func (tx *RwTx) Put(tableName db.TableName, key, value []byte) error {
	return tx.tx.Put(tableName, key, value)
}

func (tx *RwTx) PutToShard(shardId types.ShardId, tableName db.ShardedTableName, key, value []byte) error {
	return tx.tx.PutToShard(shardId, tableName, key, value)
}

// TODO: add tombstones for delete (do we even need delete? It seems that our main workflow is append-only)
func (tx *RwTx) Delete(tableName db.TableName, key []byte) error {
	if err := tx.tx.Delete(tableName, key); err != nil {
		return err
	}
	return tx.tx.Put(TombstoneTableName, db.MakeKey(tableName, key), nil)
}

// TODO: add tombstones for delete
func (tx *RwTx) DeleteFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) error {
	return tx.tx.Delete(db.ShardTableName(tableName, shardId), key)
}

func (tx *RwTx) Commit() error {
	return tx.tx.Commit()
}

func (tx *RwTx) CommitWithTs() (db.Timestamp, error) {
	return tx.tx.CommitWithTs()
}

func (tx *RoTx) Rollback() {
	tx.tx.Rollback()
}

func (db *ReadThroughDb) Close() {
	db.db.Close()
}

func (db *ReadThroughDb) Stream(ctx context.Context, keyFilter func([]byte) bool, writer io.Writer) error {
	panic("Not supported")
}

func (db *ReadThroughDb) Fetch(ctx context.Context, reader io.Reader) error {
	panic("Not supported")
}

func (db *ReadThroughDb) GetSequence(ctx context.Context, key []byte, bandwidth uint64) (db.Sequence, error) {
	panic("Not supported")
}

func (db *ReadThroughDb) DropAll() error {
	return db.db.DropAll()
}

func (rdb *ReadThroughDb) CreateRoTx(ctx context.Context) (db.RoTx, error) {
	tx, err := rdb.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	return &RoTx{tx: &db.RwWrapper{RoTx: tx}, client: rdb.client, cache: &rdb.cache, cacheTomb: &rdb.cacheTomb}, nil
}

func (rdb *ReadThroughDb) CreateRoTxAt(ctx context.Context, ts db.Timestamp) (db.RoTx, error) {
	tx, err := rdb.db.CreateRoTxAt(ctx, ts)
	if err != nil {
		return nil, err
	}
	return &RoTx{tx: &db.RwWrapper{RoTx: tx}, client: rdb.client, cache: &rdb.cache, cacheTomb: &rdb.cacheTomb}, nil
}

func (db *ReadThroughDb) CreateRwTx(ctx context.Context) (db.RwTx, error) {
	tx, err := db.db.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	return &RwTx{&RoTx{tx: tx, client: db.client, cache: &db.cache, cacheTomb: &db.cacheTomb}}, nil
}

func (db *ReadThroughDb) LogGC(ctx context.Context, discardRation float64, gcFrequency time.Duration) error {
	return db.db.LogGC(ctx, discardRation, gcFrequency)
}

func NewReadThroughDb(client client.DbClient, baseDb db.DB) (db.DB, error) {
	db := &ReadThroughDb{
		client: client,
		db:     baseDb,
	}
	return db, nil
}

func NewReadThroughDbWithMainShard(
	ctx context.Context,
	client client.Client,
	cacheDb db.DB,
	masterBlockNumber transport.BlockNumber,
) (db.DB, error) {
	block, err := client.GetBlock(ctx, types.MainShardId, masterBlockNumber, false)
	if err != nil {
		return nil, err
	}
	if masterBlockNumber.IsSpecial() {
		check.PanicIfNotf(block != nil, "failed to fetch block %v from MC", masterBlockNumber)
	} else {
		check.PanicIfNotf(block != nil && block.Number == masterBlockNumber.BlockNumber(),
			"failed to fetch block %v from MC", masterBlockNumber)
	}

	tx, err := cacheDb.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if block.DbTimestamp == types.InvalidDbTimestamp {
		return nil, errors.New("the chosen block is too old and doesn't support read-through mode")
	}
	if err := client.DbInitTimestamp(ctx, block.DbTimestamp); err != nil {
		return nil, err
	}
	rdb := &ReadThroughDb{
		client: client,
		db:     cacheDb,
	}

	_, err = db.ReadLastBlockHash(tx, types.MainShardId)
	if err == nil {
		// In case there is a last block, it means that the cache is already initialized
		return rdb, nil
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	// TODO: For now, the only updatable value is the last block hash, so we don't need to remember all versions of
	// values on the server: `badgerOpts.WithNumBersionToKeep(1)`, we just need to rewrite the last block hash value.
	// Maybe we should enable all versions saving on the server with `badgerOpts.WithNumBersionToKeep(0)`
	// and get last block hash by block.dbTimestamp.
	// This solution would be more general, it would withstand other updatable values,
	// but it will consume more memory on server.
	if err := db.WriteLastBlockHash(tx, types.MainShardId, block.Hash); err != nil {
		return nil, err
	}

	for i, h := range block.ChildBlocks {
		if err := db.WriteLastBlockHash(tx, types.ShardId(i+1), h); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return rdb, nil
}

// Construct from endpoint string and db.DB
func NewReadThroughWithEndpoint(
	ctx context.Context,
	endpoint string,
	cacheDb db.DB,
	masterBlockNumber transport.BlockNumber,
) (db.DB, error) {
	client := rpc.NewClient(endpoint, logging.NewLogger("db_client"))
	return NewReadThroughDbWithMainShard(ctx, client, cacheDb, masterBlockNumber)
}
