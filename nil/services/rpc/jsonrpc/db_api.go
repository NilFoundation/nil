package jsonrpc

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var ErrApiKeyNotFound = errors.New("key not found in db api")

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

	logger logging.Logger
}

var _ DbAPI = (*DbAPIImpl)(nil)

// NewDbAPI creates a new DbAPI instance.
func NewDbAPI(db db.ReadOnlyDB, logger logging.Logger) *DbAPIImpl {
	return &DbAPIImpl{
		db:     db,
		logger: logger,
	}
}

func (dbApi *DbAPIImpl) createRoTx(ctx context.Context) (db.RoTx, error) {
	if dbApi.ts == nil {
		return nil, errors.New("timestamp is not initialized in DB API")
	}
	return dbApi.db.CreateRoTxAt(ctx, db.Timestamp(*dbApi.ts))
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

	res, err := tx.Get(tableName, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return res, ErrApiKeyNotFound
	}
	return res, err
}

func (dbApi *DbAPIImpl) ExistsInShard(
	ctx context.Context,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	key []byte,
) (bool, error) {
	return dbApi.Exists(ctx, db.ShardTableName(tableName, shardId), key)
}

func (dbApi *DbAPIImpl) GetFromShard(
	ctx context.Context,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	key []byte,
) ([]byte, error) {
	return dbApi.Get(ctx, db.ShardTableName(tableName, shardId), key)
}

// InitDbTimestamp initializes the database timestamp.
func (db *DbAPIImpl) InitDbTimestamp(ctx context.Context, ts uint64) error {
	db.ts = &ts
	return nil
}
