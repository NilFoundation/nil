package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

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

// TaskContainer is an interface for storing task entries
type TaskContainer interface {
	// Add inserts a new TaskEntry into the container
	Add(taskEntry *types.TaskEntry)
}

// TaskStorage Interface for storing and accessing tasks from DB
type TaskStorage interface {
	// AddSingleTaskEntry Store new task entry into DB, if already exist - just update it
	AddSingleTaskEntry(ctx context.Context, entry types.TaskEntry) error

	// AddTaskEntries Store set of task entries as a single transaction
	AddTaskEntries(ctx context.Context, tasks []*types.TaskEntry) error

	// TryGetTaskEntry Retrieve a task entry by its id. In case if task does not exist, method returns nil
	TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error)

	// GetTasks Retrieve tasks that match the given predicate function and pushes them to the destination container.
	GetTasks(ctx context.Context, destination TaskContainer, predicate func(*types.TaskEntry) bool) error

	// GetTaskTree retrieves the full hierarchical structure of a task and its dependencies by the given task id.
	GetTaskTree(ctx context.Context, taskId types.TaskId) (*types.TaskTree, error)

	// RequestTaskToExecute Find task with no dependencies and higher priority and assign it to the executor
	RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error)

	// ProcessTaskResult Check task result, update dependencies in case of success
	ProcessTaskResult(ctx context.Context, res types.TaskResult) error

	// RescheduleHangingTasks Identify tasks that exceed execution timeout and reschedule them to be re-executed
	RescheduleHangingTasks(ctx context.Context, currentTime time.Time, taskExecutionTimeout time.Duration) error
}

type TaskStorageMetrics interface {
	RecordTaskAdded(ctx context.Context, task *types.TaskEntry)
	RecordTaskStarted(ctx context.Context, taskEntry *types.TaskEntry)
	RecordTaskTerminated(ctx context.Context, taskEntry *types.TaskEntry, taskResult *types.TaskResult)
	RecordTaskRescheduled(ctx context.Context, taskEntry *types.TaskEntry)
}

type taskStorage struct {
	database    db.DB
	retryRunner common.RetryRunner
	metrics     TaskStorageMetrics
	logger      zerolog.Logger
}

func NewTaskStorage(
	db db.DB,
	metrics TaskStorageMetrics,
	logger zerolog.Logger,
) TaskStorage {
	return &taskStorage{
		database:    db,
		retryRunner: badgerRetryRunner(logger),
		metrics:     metrics,
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
		return fmt.Errorf("failed to encode task with id %s: %w", entry.Task.Id, err)
	}
	if err := tx.Put(TaskEntriesTable, entry.Task.Id.Bytes(), inputBuffer.Bytes()); err != nil {
		return fmt.Errorf("failed to put task with id %s: %w", entry.Task.Id, err)
	}
	return nil
}

func (st *taskStorage) AddSingleTaskEntry(ctx context.Context, entry types.TaskEntry) error {
	err := st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.addSingleTaskEntryImpl(ctx, entry)
	})
	if err != nil {
		return err
	}

	st.metrics.RecordTaskAdded(ctx, &entry)
	return nil
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

	return tx.Commit()
}

func (st *taskStorage) AddTaskEntries(ctx context.Context, tasks []*types.TaskEntry) error {
	err := st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.addTaskEntriesImpl(ctx, tasks)
	})
	if err != nil {
		return err
	}

	for _, entry := range tasks {
		st.metrics.RecordTaskAdded(ctx, entry)
	}
	return nil
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

func (st *taskStorage) GetTasks(ctx context.Context, destination TaskContainer, predicate func(*types.TaskEntry) bool) error {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = iterateOverTaskEntries(tx, func(entry *types.TaskEntry) error {
		if predicate(entry) {
			destination.Add(entry)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve tasks based on predicate: %w", err)
	}

	return nil
}

func (st *taskStorage) GetTaskTree(ctx context.Context, rootTaskId types.TaskId) (*types.TaskTree, error) {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// track seen tasks to not extract them with dependencies more than once from the storage
	seen := make(map[types.TaskId]*types.TaskTree)
	const depthLimit = 50

	var getTaskTreeRec func(taskId types.TaskId, currentDepth int) (*types.TaskTree, error)
	getTaskTreeRec = func(taskId types.TaskId, currentDepth int) (*types.TaskTree, error) {
		if currentDepth > depthLimit {
			return nil, fmt.Errorf("tree depth limit exceeded (%d) for task with id=%s", depthLimit, taskId)
		}

		if seenTree, ok := seen[taskId]; ok {
			return seenTree, nil
		}

		entry, err := extractTaskEntry(tx, taskId)

		if errors.Is(err, db.ErrKeyNotFound) && taskId == rootTaskId {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("failed to get task with id=%s", taskId)
		}

		tree := types.NewTaskTree(entry)
		seen[taskId] = tree

		for dependencyId := range entry.Task.PendingDependencies {
			subtree, err := getTaskTreeRec(dependencyId, currentDepth+1)
			if err != nil {
				return nil, fmt.Errorf("failed to get task subtree with id=%s: %w", dependencyId, err)
			}
			tree.AddDependency(subtree)
		}

		for dependencyId, result := range entry.Task.DependencyResults {
			subtree, err := getTaskTreeRec(dependencyId, currentDepth+1)
			if err != nil {
				return nil, fmt.Errorf("failed to get task subtree with id=%s: %w", dependencyId, err)
			}
			subtree.Result = &result
			tree.AddDependency(subtree)
		}

		return tree, nil
	}

	return getTaskTreeRec(rootTaskId, 0)
}

// Helper to find available task with higher priority
func findHigherPriorityTask(tx db.RoTx) (*types.TaskEntry, error) {
	var res *types.TaskEntry = nil

	err := iterateOverTaskEntries(tx, func(entry *types.TaskEntry) error {
		if entry.Status == types.WaitingForExecutor {
			if res == nil || types.HigherPriority(entry, res) {
				res = entry
			}
		}
		return nil
	})

	return res, err
}

func (st *taskStorage) RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error) {
	var taskEntry *types.TaskEntry
	err := st.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		taskEntry, err = st.requestTaskToExecuteImpl(ctx, executor)
		return err
	})
	if err != nil {
		return nil, err
	}

	if taskEntry == nil {
		return nil, nil
	}

	st.metrics.RecordTaskStarted(ctx, taskEntry)
	return &taskEntry.Task, nil
}

func (st *taskStorage) requestTaskToExecuteImpl(ctx context.Context, executor types.TaskExecutorId) (*types.TaskEntry, error) {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	taskEntry, err := findHigherPriorityTask(tx)
	if err != nil {
		return nil, err
	}
	if taskEntry == nil {
		// No task available
		return nil, nil
	}

	if err := taskEntry.Start(executor); err != nil {
		return nil, fmt.Errorf("failed to start task: %w", err)
	}
	if err := putTaskEntry(tx, taskEntry); err != nil {
		return nil, fmt.Errorf("failed to update task entry: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return taskEntry, nil
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
		entry.Status = types.Failed
		now := time.Now().UTC()
		entry.Finished = &now
		if err := putTaskEntry(tx, entry); err != nil {
			return fmt.Errorf("failed to set task entry with id=%s as failed: %w", entry.Task.Id, err)
		}
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
	for taskId := range entry.Dependents {
		depEntry, err := extractTaskEntry(tx, taskId)
		if err != nil {
			return err
		}
		if err = depEntry.AddDependencyResult(res); err != nil {
			return fmt.Errorf("failed to add dependency result to task with id=%s: %w", depEntry.Task.Id, err)
		}
		err = putTaskEntry(tx, depEntry)
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	st.metrics.RecordTaskTerminated(ctx, entry, &res)
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

		executionTime := currentTime.Sub(*entry.Started)
		if executionTime <= taskExecutionTimeout {
			return nil
		}

		st.metrics.RecordTaskRescheduled(ctx, entry)

		if err := st.rescheduleTaskTx(tx, entry, executionTime); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (st *taskStorage) rescheduleTaskTx(tx db.RwTx, entry *types.TaskEntry, executionTime time.Duration) error {
	st.logger.Warn().
		Interface("taskId", entry.Task.Id).
		Interface("executorId", entry.Owner).
		Dur("executionTime", executionTime).
		Msg("Task execution timeout, rescheduling")

	if err := entry.ResetRunning(); err != nil {
		return fmt.Errorf("failed to reset task: %w", err)
	}

	return putTaskEntry(tx, entry)
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
