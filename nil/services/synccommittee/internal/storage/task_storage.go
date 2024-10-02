package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/badger/v4"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

// TaskEntriesTable BadgerDB tables, TaskId is used as a key
const (
	TaskEntriesTable = "TaskEntries"
)

var (
	ErrTaskInvalidStatus = errors.New("task has invalid status")
	ErrTaskWrongExecutor = errors.New("task belongs to another executor")
)

// TaskStorage Interface for storing and accessing tasks from DB
type TaskStorage interface {
	// AddSingleTaskEntry Store new task entry into DB
	AddSingleTaskEntry(ctx context.Context, entry types.TaskEntry) error

	// AddTaskEntries Store set of task entries as a single transaction
	AddTaskEntries(ctx context.Context, tasks []*types.TaskEntry) error

	// TryGetTaskEntry Retrieve a task entry by its id. In case if task does not exist, method returns nil
	TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error)

	// RemoveTaskEntry Delete existing task entry from DB
	RemoveTaskEntry(ctx context.Context, id types.TaskId) error

	// RequestTaskToExecute Find task with no dependencies and higher priority and assign it to the executor
	RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error)

	// ProcessTaskResult Check task result, update dependencies in case of success
	ProcessTaskResult(ctx context.Context, res types.TaskResult) error

	// RescheduleHangingTasks Identify tasks that exceed execution timeout and reschedule them to be re-executed
	RescheduleHangingTasks(ctx context.Context, currentTime time.Time, taskExecutionTimeout time.Duration) error
}

type taskStorage struct {
	database    db.DB
	retryRunner common.RetryRunner
	logger      zerolog.Logger
}

func NewTaskStorage(db db.DB, logger zerolog.Logger) TaskStorage {
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: func(_ uint32, err error) bool {
				return errors.Is(err, badger.ErrConflict)
			},
			NextDelay: func(_ uint32) time.Duration {
				delay, err := common.RandomDelay(20*time.Millisecond, 100*time.Millisecond)
				if err != nil {
					logger.Error().Err(err).Msg("failed to generate task storage retry delay")
					return 100 * time.Millisecond
				}
				return *delay
			},
		},
		logger,
	)

	return &taskStorage{
		database:    db,
		retryRunner: retryRunner,
		logger:      logger,
	}
}

// Helper to get and decode task entry from DB
func extractTaskEntry(tx db.RoTx, id types.TaskId) (*types.TaskEntry, error) {
	encoded, err := tx.Get(TaskEntriesTable, id.Bytes())
	if err != nil {
		return nil, err
	}

	entry := &types.TaskEntry{}
	if err = gob.NewDecoder(bytes.NewBuffer(encoded)).Decode(&entry); err != nil {
		return nil, fmt.Errorf("failed to decode task with id %v: %w", id, err)
	}
	return entry, nil
}

// Helper to encode and put task entry into DB
func putTaskEntry(tx db.RwTx, entry *types.TaskEntry) error {
	var inputBuffer bytes.Buffer
	err := gob.NewEncoder(&inputBuffer).Encode(entry)
	if err != nil {
		return fmt.Errorf("failed to encode task with id %v: %w", entry.Task.Id, err)
	}
	err = tx.Put(TaskEntriesTable, entry.Task.Id.Bytes(), inputBuffer.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (st *taskStorage) AddSingleTaskEntry(ctx context.Context, entry types.TaskEntry) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.addSingleTaskEntryImpl(ctx, entry)
	})
}

func (st *taskStorage) addSingleTaskEntryImpl(ctx context.Context, entry types.TaskEntry) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = putTaskEntry(tx, &entry)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (st *taskStorage) AddTaskEntries(ctx context.Context, tasks []*types.TaskEntry) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.addTaskEntriesImpl(ctx, tasks)
	})
}

func (st *taskStorage) addTaskEntriesImpl(ctx context.Context, tasks []*types.TaskEntry) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, entry := range tasks {
		err = putTaskEntry(tx, entry)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (st *taskStorage) TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error) {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	entry, err := extractTaskEntry(tx, id)

	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	return entry, err
}

func (st *taskStorage) RemoveTaskEntry(ctx context.Context, id types.TaskId) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.removeTaskEntryImpl(ctx, id)
	})
}

// Delete existing task entry from DB
func (st *taskStorage) removeTaskEntryImpl(ctx context.Context, id types.TaskId) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = tx.Delete(TaskEntriesTable, id.Bytes())
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Helper to update task status when it's ready to be executed or needs to be rescheduled
func updateTaskStatus(tx db.RwTx, id types.TaskId, newStatus types.TaskStatus, newOwner types.TaskExecutorId) error {
	entry, err := extractTaskEntry(tx, id)
	if err != nil {
		return err
	}
	entry.Status = newStatus
	entry.Modified = time.Now()
	entry.Owner = newOwner
	return putTaskEntry(tx, entry)
}

// Helper to find available task with higher priority
func findHigherPriorityTask(tx db.RoTx) (*types.Task, error) {
	var res *types.Task = nil

	err := iterateOverTaskEntries(tx, func(entry *types.TaskEntry) error {
		if entry.Status == types.WaitingForExecutor {
			if res == nil || types.HigherPriority(entry.Task, *res) {
				res = &entry.Task
			}
		}
		return nil
	})

	return res, err
}

func (st *taskStorage) RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error) {
	var task *types.Task
	err := st.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		task, err = st.requestTaskToExecuteImpl(ctx, executor)
		return err
	})
	return task, err
}

func (st *taskStorage) requestTaskToExecuteImpl(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error) {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	resultTask, err := findHigherPriorityTask(tx)
	if err != nil {
		return nil, err
	}
	if resultTask == nil {
		// No task available
		return resultTask, nil
	}
	err = updateTaskStatus(tx, resultTask.Id, types.Running, executor)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return resultTask, nil
}

func (st *taskStorage) ProcessTaskResult(ctx context.Context, res types.TaskResult) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.processTaskResultImpl(ctx, res)
	})
}

func (st *taskStorage) processTaskResultImpl(ctx context.Context, res types.TaskResult) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// First we check the result and set status to failed if unsuccessful
	entry, err := extractTaskEntry(tx, res.TaskId)
	if err != nil {
		// ErrKeyNotFound is not considered an error because of possible re-invocations
		if errors.Is(err, db.ErrKeyNotFound) {
			st.logger.Warn().Err(err).Interface("taskId", res.TaskId).Msg("Task entry was not found")
			return nil
		}

		return err
	}

	if err := st.validateTaskResult(*entry, res); err != nil {
		return err
	}

	if !res.IsSuccess {
		entry.Modified = time.Now()
		entry.Status = types.Failed
		if err = tx.Commit(); err != nil {
			return err
		}
		return nil
	}

	// We don't keep finished tasks in DB
	st.logger.Debug().
		Interface("taskId", res.TaskId).
		Interface("requestSenderId", res.Sender).
		Msg("Task execution is completed, removing it from the storage")

	err = tx.Delete(TaskEntriesTable, res.TaskId.Bytes())
	if err != nil {
		return err
	}

	// Update all the tasks that are waiting for this result
	for _, id := range entry.PendingDeps {
		depEntry, err := extractTaskEntry(tx, id)
		if err != nil {
			return err
		}
		depEntry.Task.AddDependencyResult(res)
		depEntry.Modified = time.Now()
		if len(depEntry.Task.Dependencies) == int(depEntry.Task.DependencyNum) {
			depEntry.Status = types.WaitingForExecutor
		}
		err = putTaskEntry(tx, depEntry)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (st *taskStorage) validateTaskResult(entry types.TaskEntry, res types.TaskResult) error {
	const errFormat = "failed to process task result, taskId=%v, taskStatus=%v, taskOwner=%v, requestSenderId=%v: %w"

	if entry.Owner != res.Sender {
		return fmt.Errorf(errFormat, entry.Task.Id, entry.Status, entry.Owner, res.Sender, ErrTaskWrongExecutor)
	}

	if entry.Status != types.Running {
		return fmt.Errorf(errFormat, entry.Task.Id, entry.Status, entry.Owner, res.Sender, ErrTaskInvalidStatus)
	}

	return nil
}

func (st *taskStorage) RescheduleHangingTasks(ctx context.Context, currentTime time.Time, taskExecutionTimeout time.Duration) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.rescheduleHangingTasksImpl(ctx, currentTime, taskExecutionTimeout)
	})
}

func (st *taskStorage) rescheduleHangingTasksImpl(
	ctx context.Context,
	currentTime time.Time,
	taskExecutionTimeout time.Duration,
) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = iterateOverTaskEntries(tx, func(entry *types.TaskEntry) error {
		if entry.Status != types.Running {
			return nil
		}

		executionTime := currentTime.Sub(entry.Modified)
		if executionTime <= taskExecutionTimeout {
			return nil
		}

		st.logger.Warn().
			Interface("taskId", entry.Task.Id).
			Interface("executorId", entry.Owner).
			Dur("executionTime", executionTime).
			Msg("Task execution timeout, rescheduling")

		entry.Modified = time.Now()
		entry.Status = types.WaitingForExecutor
		entry.Owner = types.UnknownExecutorId
		return putTaskEntry(tx, entry)
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

func iterateOverTaskEntries(tx db.RoTx, action func(entry *types.TaskEntry) error) error {
	iter, err := tx.Range(TaskEntriesTable, nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.HasNext() {
		key, val, err := iter.Next()
		if err != nil {
			return err
		}
		entry := &types.TaskEntry{}
		if err = gob.NewDecoder(bytes.NewBuffer(val)).Decode(&entry); err != nil {
			return fmt.Errorf("failed to decode task with id %v: %w", string(key), err)
		}
		err = action(entry)
		if err != nil {
			return err
		}
	}

	return nil
}
