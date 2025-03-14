package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

// taskResultTable stores task execution results. Key: types.TaskId, Value: types.TaskResult;
const (
	taskResultsTable db.TableName = "task_results"
)

func NewTaskResultStorage(
	db db.DB,
	logger zerolog.Logger,
) *TaskResultStorage {
	return &TaskResultStorage{
		commonStorage: makeCommonStorage(db, logger),
	}
}

// TaskResultStorage defines the type for storing and managing task results.
type TaskResultStorage struct {
	commonStorage
}

// TryGetPending retrieves the first available TaskResult from storage or returns nil if none are available.
func (s *TaskResultStorage) TryGetPending(ctx context.Context) (*types.TaskResult, error) {
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

// Put stores the provided TaskResult into the storage.
func (s *TaskResultStorage) Put(ctx context.Context, result *types.TaskResult) error {
	if result == nil {
		return errors.New("result cannot be nil")
	}

	return s.retryRunner.Do(ctx, func(ctx context.Context) error {
		return s.putImpl(ctx, result)
	})
}

func (s *TaskResultStorage) putImpl(ctx context.Context, result *types.TaskResult) error {
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

// SetAsSubmitted removes the task result with the specified TaskId from the storage.
func (s *TaskResultStorage) SetAsSubmitted(ctx context.Context, taskId types.TaskId) error {
	return s.retryRunner.Do(ctx, func(ctx context.Context) error {
		return s.deleteImpl(ctx, taskId)
	})
}

func (s *TaskResultStorage) deleteImpl(ctx context.Context, taskId types.TaskId) error {
	tx, err := s.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	key := taskId.Bytes()
	err = tx.Delete(taskResultsTable, key)
	if errors.Is(err, db.ErrKeyNotFound) {
		s.logger.Debug().
			Stringer(logging.FieldTaskId, taskId).
			Msg("task result with the specified taskId does not exist")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete task result with id=%s: %w", taskId, err)
	}

	return s.commit(tx)
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
