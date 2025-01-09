package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

// taskResultTable stores task execution results. Key: types.TaskId, Value: types.TaskResult;
const (
	taskResultsTable db.TableName = "task_results"
)

// TaskResultStorage defines the interface for storing and managing task results.
type TaskResultStorage interface {
	// TryGetNext retrieves the next available TaskResult from storage or returns nil if none are available.
	TryGetNext(ctx context.Context) (*types.TaskResult, error)

	// Put stores the provided TaskResult into the storage.
	Put(ctx context.Context, result *types.TaskResult) error

	// Delete removes the task result with the specified TaskId from the storage.
	Delete(ctx context.Context, taskId types.TaskId) error
}

func NewTaskResultStorage(
	db db.DB,
	logger zerolog.Logger,
) TaskResultStorage {
	return &taskResultStorage{
		database:    db,
		retryRunner: badgerRetryRunner(logger),
		logger:      logger,
	}
}

type taskResultStorage struct {
	database    db.DB
	retryRunner common.RetryRunner
	logger      zerolog.Logger
}

func (s *taskResultStorage) TryGetNext(ctx context.Context) (*types.TaskResult, error) {
	tx, err := s.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	iter, err := tx.Range(taskResultsTable, nil, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	if !iter.HasNext() {
		return nil, nil
	}

	key, val, err := iter.Next()
	if err != nil {
		return nil, err
	}
	return unmarshallTaskResult(key, val)
}

func (s *taskResultStorage) Put(ctx context.Context, result *types.TaskResult) error {
	if result == nil {
		return errors.New("result cannot be nil")
	}

	return s.retryRunner.Do(ctx, func(ctx context.Context) error {
		return s.putImpl(ctx, result)
	})
}

func (s *taskResultStorage) putImpl(ctx context.Context, result *types.TaskResult) error {
	tx, err := s.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := result.TaskId.Bytes()
	val, err := marshallTaskResult(result)
	if err != nil {
		return err
	}

	if err := tx.Put(taskResultsTable, key, val); err != nil {
		return fmt.Errorf("failed to put task result with id=%s: %w", result.TaskId, err)
	}

	return s.commit(tx)
}

func (s *taskResultStorage) Delete(ctx context.Context, taskId types.TaskId) error {
	return s.retryRunner.Do(ctx, func(ctx context.Context) error {
		return s.deleteImpl(ctx, taskId)
	})
}

func (s *taskResultStorage) deleteImpl(ctx context.Context, taskId types.TaskId) error {
	tx, err := s.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := taskId.Bytes()
	err = tx.Delete(taskResultsTable, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		s.logger.Debug().Interface("taskId", taskId).Msg("task result with the specified taskId does not exist")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete task result with id=%s: %w", taskId, err)
	}

	return s.commit(tx)
}

func (*taskResultStorage) commit(tx db.RwTx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func marshallTaskResult(result *types.TaskResult) ([]byte, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: failed to marshall result of the task with id=%s: %w",
			ErrSerializationFailed, result.TaskId, err,
		)
	}
	return bytes, nil
}

func unmarshallTaskResult(key []byte, val []byte) (*types.TaskResult, error) {
	result := &types.TaskResult{}
	if err := json.Unmarshal(val, result); err != nil {
		return nil, fmt.Errorf(
			"%w: failed to unmarshall result of the task with id=%s: %w",
			ErrSerializationFailed, hex.EncodeToString(key), err,
		)
	}
	return result, nil
}
