package storage

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	// executorIdTable stores a task executor identifier assigned to the current running service.
	// Key: executorIdRowKey, Value: types.TaskExecutorId.
	executorIdTable db.TableName = "task_executor_id"
)

var executorIdRowKey = []byte(executorIdTable)

type executorIdStorage struct {
	commonStorage
}

func NewExecutorIdStorage(
	database db.DB,
	logger logging.Logger,
) *executorIdStorage {
	return &executorIdStorage{
		commonStorage: makeCommonStorage(database, logger),
	}
}

func (s *executorIdStorage) GetOrAdd(
	ctx context.Context, idGenerator func() types.TaskExecutorId,
) (*types.TaskExecutorId, error) {
	var id *types.TaskExecutorId
	err := s.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		id, err = s.getOrAdd(ctx, idGenerator)
		return err
	})
	return id, err
}

func (s *executorIdStorage) getOrAdd(
	ctx context.Context,
	idGenerator func() types.TaskExecutorId,
) (*types.TaskExecutorId, error) {
	tx, err := s.database.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	id, err := s.tryGetId(tx)
	if err != nil {
		return nil, err
	}
	if id != nil {
		return id, nil
	}

	generated := idGenerator()
	if err := s.putId(tx, generated); err != nil {
		return nil, err
	}

	if err := s.commit(tx); err != nil {
		return nil, err
	}

	return &generated, nil
}

func (s *executorIdStorage) tryGetId(tx db.RoTx) (*types.TaskExecutorId, error) {
	idBytes, err := tx.Get(executorIdTable, executorIdRowKey)
	switch {
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	}

	var id types.TaskExecutorId
	if err := id.UnmarshalBinary(idBytes); err != nil {
		return nil, err
	}

	return &id, nil
}

func (s *executorIdStorage) putId(tx db.RwTx, id types.TaskExecutorId) error {
	bytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}
	return tx.Put(executorIdTable, executorIdRowKey, bytes)
}
