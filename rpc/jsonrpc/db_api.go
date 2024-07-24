package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type DbAPI interface {
	InitDbTimestamp(ctx context.Context, ts uint64) error
	Exists(ctx context.Context, tableName db.TableName, key []byte) (bool, error)
	Get(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error)

	ExistsInShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error)
	GetFromShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error)
}

type DbAPIImpl struct {
	ts *uint64
	db db.ReadOnlyDB

	logger zerolog.Logger
}

var _ DbAPI = (*DbAPIImpl)(nil)

// NewDbAPI creates a new DbAPI instance.
func NewDbAPI(db db.ReadOnlyDB, logger zerolog.Logger) *DbAPIImpl {
	return &DbAPIImpl{
		db:     db,
		logger: logger,
	}
}

func (dbApi *DbAPIImpl) createRoTx(ctx context.Context) (db.RoTx, error) {
	if dbApi.ts == nil {
		return dbApi.db.CreateRoTx(ctx)
	} else {
		return dbApi.db.CreateRoTxAt(ctx, db.Timestamp(*dbApi.ts))
	}
}

func (dbApi *DbAPIImpl) Exists(ctx context.Context, tableName db.TableName, key []byte) (bool, error) {
	tx, err := dbApi.createRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	return tx.Exists(tableName, key)
}

func (dbApi *DbAPIImpl) Get(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error) {
	tx, err := dbApi.createRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.Get(tableName, key)
}

func (dbApi *DbAPIImpl) ExistsInShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	tx, err := dbApi.createRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	return tx.ExistsInShard(shardId, tableName, key)
}

func (dbApi *DbAPIImpl) GetFromShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	tx, err := dbApi.createRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.GetFromShard(shardId, tableName, key)
}

// InitDbTimestamp initializes the database timestamp.
func (db *DbAPIImpl) InitDbTimestamp(ctx context.Context, ts uint64) error {
	db.ts = &ts
	return nil
}
