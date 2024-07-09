package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
)

type DbAPI interface {
	Exists(ctx context.Context, tableName db.TableName, key []byte) (bool, error)
	Get(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error)

	ExistsInShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error)
	GetFromShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error)
}

type DbAPIImpl struct {
	db db.ReadOnlyDB

	logger zerolog.Logger
}

// NewDbAPI creates a new DbAPI instance.
func NewDbAPI(db db.ReadOnlyDB, logger zerolog.Logger) *DbAPIImpl {
	return &DbAPIImpl{
		db:     db,
		logger: logger,
	}
}

func (db *DbAPIImpl) Exists(ctx context.Context, tableName db.TableName, key []byte) (bool, error) {
	tx, err := db.db.CreateRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	return tx.Exists(tableName, key)
}

func (db *DbAPIImpl) Get(ctx context.Context, tableName db.TableName, key []byte) ([]byte, error) {
	tx, err := db.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.Get(tableName, key)
}

func (db *DbAPIImpl) ExistsInShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) (bool, error) {
	tx, err := db.db.CreateRoTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	return tx.ExistsInShard(shardId, tableName, key)
}

func (db *DbAPIImpl) GetFromShard(ctx context.Context, shardId types.ShardId, tableName db.ShardedTableName, key []byte) ([]byte, error) {
	tx, err := db.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return tx.GetFromShard(shardId, tableName, key)
}
