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

// TaskEntriesTable BadgerDB tables, ProverTaskId is used as a key
const (
	TaskEntriesTable = "TaskEntries"
)

// ProverTaskStorage Interface for storing and accessing tasks from DB
type ProverTaskStorage interface {
	// AddSingleTaskEntry Store new task entry into DB
	AddSingleTaskEntry(ctx context.Context, entry types.ProverTaskEntry) error

	// AddTaskEntries Store set of task entries as a single transaction
	AddTaskEntries(ctx context.Context, tasks []*types.ProverTaskEntry) error

	// RemoveTaskEntry Delete existing task entry from DB
	RemoveTaskEntry(ctx context.Context, id types.ProverTaskId) error

	// RequestTaskToExecute Find task with no dependencies and higher priority and assign it to prover
	RequestTaskToExecute(ctx context.Context, executor types.ProverId) (*types.ProverTask, error)

	// ProcessTaskResult Check task result, update dependencies in case of success
	ProcessTaskResult(ctx context.Context, res types.ProverTaskResult) error

	// RescheduleHangingTasks Identify tasks that exceed execution timeout and reschedule them to be re-executed
	RescheduleHangingTasks(ctx context.Context, currentTime time.Time, taskExecutionTimeout time.Duration) error
}

type proverTaskStorage struct {
	database    db.DB
	retryRunner common.RetryRunner
	logger      zerolog.Logger
}

func NewTaskStorage(db db.DB, logger zerolog.Logger) ProverTaskStorage {
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: func(err error) bool {
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

	return &proverTaskStorage{
		database:    db,
		retryRunner: retryRunner,
		logger:      logger,
	}
}

// Helper to get and decode task entry from DB
func extractTaskEntry(tx db.RwTx, id types.ProverTaskId) (*types.ProverTaskEntry, error) {
	encoded, err := tx.Get(TaskEntriesTable, id.Bytes())
	if err != nil {
		return nil, err
	}

	entry := &types.ProverTaskEntry{}
	if err = gob.NewDecoder(bytes.NewBuffer(encoded)).Decode(&entry); err != nil {
		return nil, fmt.Errorf("failed to decode task with id %v: %w", id, err)
	}
	return entry, nil
}

// Helper to encode and put task entry into DB
func putTaskEntry(tx db.RwTx, entry *types.ProverTaskEntry) error {
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

func (st *proverTaskStorage) AddSingleTaskEntry(ctx context.Context, entry types.ProverTaskEntry) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.addSingleTaskEntryImpl(ctx, entry)
	})
}

func (st *proverTaskStorage) addSingleTaskEntryImpl(ctx context.Context, entry types.ProverTaskEntry) error {
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

func (st *proverTaskStorage) AddTaskEntries(ctx context.Context, tasks []*types.ProverTaskEntry) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.addTaskEntriesImpl(ctx, tasks)
	})
}

func (st *proverTaskStorage) addTaskEntriesImpl(ctx context.Context, tasks []*types.ProverTaskEntry) error {
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

func (st *proverTaskStorage) RemoveTaskEntry(ctx context.Context, id types.ProverTaskId) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.removeTaskEntryImpl(ctx, id)
	})
}

// Delete existing task entry from DB
func (st *proverTaskStorage) removeTaskEntryImpl(ctx context.Context, id types.ProverTaskId) error {
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
func updateTaskStatus(tx db.RwTx, id types.ProverTaskId, newStatus types.ProverTaskStatus, newOwner types.ProverId) error {
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
func findHigherPriorityTask(tx db.RoTx) (*types.ProverTask, error) {
	var res *types.ProverTask = nil

	err := iterateOverTaskEntries(tx, func(entry *types.ProverTaskEntry) error {
		if entry.Status == types.WaitingForProver {
			if res == nil || types.HigherPriority(entry.Task, *res) {
				res = &entry.Task
			}
		}
		return nil
	})

	return res, err
}

func (st *proverTaskStorage) RequestTaskToExecute(ctx context.Context, executor types.ProverId) (*types.ProverTask, error) {
	var task *types.ProverTask
	err := st.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		task, err = st.requestTaskToExecuteImpl(ctx, executor)
		return err
	})
	return task, err
}

func (st *proverTaskStorage) requestTaskToExecuteImpl(ctx context.Context, executor types.ProverId) (*types.ProverTask, error) {
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

func (st *proverTaskStorage) ProcessTaskResult(ctx context.Context, res types.ProverTaskResult) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.processTaskResultImpl(ctx, res)
	})
}

func (st *proverTaskStorage) processTaskResultImpl(ctx context.Context, res types.ProverTaskResult) error {
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
	if !res.IsSuccess {
		entry.Modified = time.Now()
		entry.Status = types.Failed
		if err = tx.Commit(); err != nil {
			return err
		}
		return nil
	}

	// We don't keep finished tasks in DB
	// TODO: maybe add some logging here
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
			depEntry.Status = types.WaitingForProver
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

func (st *proverTaskStorage) RescheduleHangingTasks(ctx context.Context, currentTime time.Time, taskExecutionTimeout time.Duration) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.rescheduleHangingTasksImpl(ctx, currentTime, taskExecutionTimeout)
	})
}

func (st *proverTaskStorage) rescheduleHangingTasksImpl(
	ctx context.Context,
	currentTime time.Time,
	taskExecutionTimeout time.Duration,
) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = iterateOverTaskEntries(tx, func(entry *types.ProverTaskEntry) error {
		if entry.Status != types.Running {
			return nil
		}

		executionTime := currentTime.Sub(entry.Modified)
		if executionTime <= taskExecutionTimeout {
			return nil
		}

		st.logger.Warn().
			Interface("taskId", entry.Task.Id).
			Interface("proverId", entry.Owner).
			Dur("executionTime", executionTime).
			Msg("Task execution timeout, rescheduling")

		entry.Modified = time.Now()
		entry.Status = types.WaitingForProver
		entry.Owner = types.UnknownProverId
		return putTaskEntry(tx, entry)
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

func iterateOverTaskEntries(tx db.RoTx, action func(entry *types.ProverTaskEntry) error) error {
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
		entry := &types.ProverTaskEntry{}
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
