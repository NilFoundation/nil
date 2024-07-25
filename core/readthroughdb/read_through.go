package readthroughdb

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

// TODO: add tombstones and in-memory concurrenct cache
type ReadThroughDb struct {
	client client.DbClient
	db     db.DB
}

type RoTx struct {
	// Using db.RwTx and RwWrapper to avoid implementing all the methods of db.RoTx in RwTx
	tx     db.RwTx
	client client.DbClient
}

var (
	_ db.RoTx = (*RoTx)(nil)
	_ db.DB   = (*ReadThroughDb)(nil)
	_ db.RwTx = (*RwTx)(nil)
)

func (tx *RoTx) Exists(tableName db.TableName, key []byte) (bool, error) {
	res, err := tx.tx.Exists(tableName, key)
	if err != nil {
		return false, err
	}
	if res {
		return true, nil
	}
	return tx.client.DbExists(tableName, key)
}

func (tx *RoTx) ReadTimestamp() db.Timestamp {
	return tx.tx.ReadTimestamp()
}

func (tx *RoTx) Get(tableName db.TableName, key []byte) ([]byte, error) {
	res, err := tx.tx.Get(tableName, key)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}
	if res != nil {
		return res, nil
	} else {
		return tx.client.DbGet(tableName, key)
	}
}

func (tx *RoTx) Range(tableName db.TableName, from []byte, to []byte) (db.Iter, error) {
	// TODO: Implement this when we will actually need ranges
	panic("implement me")
}

func (tx *RoTx) ExistsInShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	res, err := tx.tx.ExistsInShard(shardId, tableName, key)
	if err != nil {
		return false, err
	}
	if res {
		return true, nil
	} else {
		return tx.client.DbExistsInShard(shardId, tableName, key)
	}
}

func (tx *RoTx) GetFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	res, err := tx.tx.GetFromShard(shardId, tableName, key)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}
	if res != nil {
		return res, nil
	} else {
		res, err := tx.client.DbGetFromShard(shardId, tableName, key)
		if res == nil && err == nil {
			return nil, db.ErrKeyNotFound
		}
		return res, err
	}
}

func (tx *RoTx) RangeByShard(shardId types.ShardId, tableName db.ShardedTableName, from []byte, to []byte) (db.Iter, error) {
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
	return tx.tx.Delete(tableName, key)
}

// TODO: add tombstones for delete
func (tx *RwTx) DeleteFromShard(shardId types.ShardId, tableName db.ShardedTableName, key []byte) error {
	return tx.tx.DeleteFromShard(shardId, tableName, key)
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

func (db *ReadThroughDb) DropAll() error {
	return db.db.DropAll()
}

func (rdb *ReadThroughDb) CreateRoTx(ctx context.Context) (db.RoTx, error) {
	tx, err := rdb.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	return &RoTx{tx: &db.RwWrapper{RoTx: tx}, client: rdb.client}, nil
}

func (rdb *ReadThroughDb) CreateRoTxAt(ctx context.Context, ts db.Timestamp) (db.RoTx, error) {
	tx, err := rdb.db.CreateRoTxAt(ctx, ts)
	if err != nil {
		return nil, err
	}
	return &RoTx{tx: &db.RwWrapper{RoTx: tx}, client: rdb.client}, nil
}

func (db *ReadThroughDb) CreateRwTx(ctx context.Context) (db.RwTx, error) {
	tx, err := db.db.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	return &RwTx{&RoTx{tx: tx, client: db.client}}, nil
}

func (db *ReadThroughDb) LogGC(ctx context.Context, discardRation float64, gcFrequency time.Duration) error {
	return db.db.LogGC(ctx, discardRation, gcFrequency)
}

func NewReadThroughDb(client client.DbClient, db db.DB) db.DB {
	return &ReadThroughDb{
		client: client,
		db:     db,
	}
}

func NewReadThroughDbWithMasterChain(ctx context.Context, client client.Client, cacheDb db.DB, masterBlockNumber transport.BlockNumber) (db.DB, error) {
	block, err := client.GetBlock(types.MainShardId, masterBlockNumber, false)
	if err != nil {
		return nil, err
	}
	check.PanicIfNot(block.Number != types.BlockNumber(masterBlockNumber))

	tx, err := cacheDb.CreateRwTx(ctx)
	defer tx.Rollback()
	if err != nil {
		return nil, err
	}

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

	if block.DbTimestamp == types.InvalidDbTimestamp {
		return nil, errors.New("The chosen block is too old and doesn't support read-through mode")
	}
	if err := client.DbInitTimestamp(block.DbTimestamp); err != nil {
		return nil, err
	}

	return &ReadThroughDb{
		client: client,
		db:     cacheDb,
	}, nil
}

// Construct from endpoint string and db.DB
func NewReadThroughWithEndpoint(ctx context.Context, endpoint string, cacheDb db.DB, masterBlockNumber transport.BlockNumber) (db.DB, error) {
	client := rpc.NewClient(endpoint, logging.NewLogger("db_client"))
	return NewReadThroughDbWithMasterChain(ctx, client, cacheDb, masterBlockNumber)
}
