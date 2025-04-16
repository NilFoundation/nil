package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
)

const (
	// TaskEntriesTable BadgerDB tables, TaskId is used as a key
	taskEntriesTable db.TableName = "task_entries"

	// FailedTaskEntriesTable BadgerDB tables, TaskId is used as a key
	failedTaskEntriesTable db.TableName = "failed_task_entries"

	// rescheduledTasksPerTxLimit defines the maximum number of tasks that can be rescheduled
	// in a single transaction of TaskStorage.RescheduleHangingTasks.
	rescheduledTasksPerTxLimit = 100
)

type TaskStorageMetrics interface {
	SetStatsProvider(provider types.TaskStatsProvider)

	RecordTaskAdded(ctx context.Context, task *types.TaskEntry)
	RecordTaskTerminated(ctx context.Context, taskEntry *types.TaskEntry, taskResult *types.TaskResult)
	RecordTaskRescheduled(ctx context.Context, taskType types.TaskType, previousExecutor types.TaskExecutorId)
}

// TaskStorage defines a type for managing tasks and their lifecycle operations.
type TaskStorage struct {
	commonStorage
	clock   clockwork.Clock
	metrics TaskStorageMetrics
}

func NewTaskStorage(
	db db.DB,
	clock clockwork.Clock,
	metrics TaskStorageMetrics,
	logger logging.Logger,
) *TaskStorage {
	taskStorage := &TaskStorage{
		commonStorage: makeCommonStorage(
			db,
			logger,
			common.DoNotRetryIf(types.ErrTaskWrongExecutor, types.ErrTaskInvalidStatus, ErrTaskAlreadyExists),
		),
		clock:   clock,
		metrics: metrics,
	}

	metrics.SetStatsProvider(taskStorage)
	return taskStorage
}

func (st *TaskStorage) GetTaskStats(ctx context.Context) (*types.TaskStats, error) {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stats := types.NewEmptyTaskStats()

	for entry, err := range st.getStoredTasksSeq(tx) {
		if err != nil {
			return nil, err
		}
		stats.Add(entry)
	}

	return stats, nil
}

// Helper to get and decode task entry from DB
func (*TaskStorage) extractTaskEntry(tx db.RoTx, id types.TaskId) (*types.TaskEntry, error) {
	encoded, err := tx.Get(taskEntriesTable, id.Bytes())
	if err != nil {
		return nil, err
	}

	entry := &types.TaskEntry{}
	if err = gob.NewDecoder(bytes.NewBuffer(encoded)).Decode(&entry); err != nil {
		return nil, fmt.Errorf("%w: failed to decode task with id %v: %w", ErrSerializationFailed, id, err)
	}
	return entry, nil
}

// Helper to get and decode failed task entry from DB
func (*TaskStorage) extractFailedTaskEntry(tx db.RoTx, id types.TaskId) (*types.TaskEntry, error) {
	encoded, err := tx.Get(failedTaskEntriesTable, id.Bytes())
	if err != nil {
		return nil, err
	}

	entry := &types.TaskEntry{}
	if err = gob.NewDecoder(bytes.NewBuffer(encoded)).Decode(&entry); err != nil {
		return nil, fmt.Errorf("%w: failed to decode task with id %v: %w", ErrSerializationFailed, id, err)
	}
	return entry, nil
}

// Helper to encode and put task entry into DB
func (st *TaskStorage) putTaskEntry(tx db.RwTx, entry *types.TaskEntry, isFailedTask bool) error {
	var inputBuffer bytes.Buffer
	err := gob.NewEncoder(&inputBuffer).Encode(entry)
	if err != nil {
		return fmt.Errorf("%w: failed to encode task with id %s: %w", ErrSerializationFailed, entry.Task.Id, err)
	}
	key := st.makeTaskKey(entry)

	tableName := taskEntriesTable
	if isFailedTask {
		tableName = failedTaskEntriesTable
	}
	if err := tx.Put(tableName, key, inputBuffer.Bytes()); err != nil {
		return fmt.Errorf("failed to put task with id %s: %w", entry.Task.Id, err)
	}
	return nil
}

// AddTaskEntries saves set of task entries.
// If at least one task with a given id already exists, method returns ErrTaskAlreadyExists.
func (st *TaskStorage) AddTaskEntries(ctx context.Context, tasks ...*types.TaskEntry) error {
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

func (st *TaskStorage) addTaskEntriesImpl(ctx context.Context, tasks []*types.TaskEntry) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, entry := range tasks {
		if entry == nil {
			return errNilTaskEntry
		}
		if err := st.addSingleTaskEntryTx(tx, entry); err != nil {
			return err
		}
	}
	return st.commit(tx)
}

func (st *TaskStorage) addSingleTaskEntryTx(tx db.RwTx, entry *types.TaskEntry) error {
	key := st.makeTaskKey(entry)
	exists, err := tx.Exists(taskEntriesTable, key)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w: taskId=%s", ErrTaskAlreadyExists, entry.Task.Id)
	}

	return st.putTaskEntry(tx, entry, false)
}

// TryGetTaskEntry Retrieve a task entry by its id. In case if task does not exist, method returns nil
func (st *TaskStorage) TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error) {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	entry, err := st.extractTaskEntry(tx, id)

	if errors.Is(err, db.ErrKeyNotFound) {
		// check in the failed task table
		entry, err = st.extractFailedTaskEntry(tx, id)
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
	}

	return entry, err
}

// GetTaskViews Retrieve tasks that match the given predicate function and pushes them to the destination container.
func (st *TaskStorage) GetTaskViews(
	ctx context.Context,
	destination interface{ Add(task *public.TaskView) },
	predicate func(*public.TaskView) bool,
) error {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentTime := st.clock.Now()

	for entry, err := range st.getStoredTasksSeq(tx) {
		if err != nil {
			return err
		}
		taskView := public.NewTaskView(entry, currentTime)
		if predicate(taskView) {
			destination.Add(taskView)
		}
	}

	return nil
}

// GetTaskTreeView retrieves the full hierarchical structure of a task and its dependencies by the given task id.
func (st *TaskStorage) GetTaskTreeView(ctx context.Context, rootTaskId types.TaskId) (*public.TaskTreeView, error) {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentTime := st.clock.Now()

	// track seen tasks to not extract them with dependencies more than once from the storage
	seen := make(map[types.TaskId]*public.TaskTreeView)

	var getTaskTreeRec func(taskId types.TaskId, currentDepth int) (*public.TaskTreeView, error)
	getTaskTreeRec = func(taskId types.TaskId, currentDepth int) (*public.TaskTreeView, error) {
		if currentDepth > public.TreeViewDepthLimit {
			return nil, public.TreeDepthExceededErr(taskId)
		}

		if seenTree, ok := seen[taskId]; ok {
			return seenTree, nil
		}

		entry, err := st.extractTaskEntry(tx, taskId)

		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("failed to get task with id=%s: %w", taskId, err)
		}

		tree := public.NewTaskTreeFromEntry(entry, currentTime)
		seen[taskId] = tree

		for dependencyId := range entry.PendingDependencies {
			subtree, err := getTaskTreeRec(dependencyId, currentDepth+1)
			if err != nil {
				return nil, fmt.Errorf("failed to get task subtree with id=%s: %w", dependencyId, err)
			}
			if subtree != nil {
				tree.AddDependency(subtree)
			}
		}

		for _, result := range entry.Task.DependencyResults {
			subtree := public.NewTaskTreeFromResult(&result)
			tree.AddDependency(subtree)
		}

		return tree, nil
	}

	return getTaskTreeRec(rootTaskId, 0)
}

// Helper to find available task with higher priority
func (st *TaskStorage) findTopPriorityTask(tx db.RoTx) (*types.TaskEntry, error) {
	var topPriorityTask *types.TaskEntry

	for entry, err := range st.getStoredTasksSeq(tx) {
		if err != nil {
			return nil, err
		}

		if entry.Status != types.WaitingForExecutor {
			continue
		}

		if entry.HasHigherPriorityThan(topPriorityTask) {
			topPriorityTask = entry
		}
	}

	return topPriorityTask, nil
}

// RequestTaskToExecute Find task with no dependencies and higher priority and assign it to the executor
func (st *TaskStorage) RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error) {
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

	return &taskEntry.Task, nil
}

func (st *TaskStorage) requestTaskToExecuteImpl(
	ctx context.Context,
	executor types.TaskExecutorId,
) (*types.TaskEntry, error) {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	taskEntry, err := st.findTopPriorityTask(tx)
	if err != nil {
		return nil, err
	}
	if taskEntry == nil {
		// No task available
		return nil, nil
	}

	currentTime := st.clock.Now()
	if err := taskEntry.Start(executor, currentTime); err != nil {
		return nil, fmt.Errorf("failed to start task: %w", err)
	}
	if err := st.putTaskEntry(tx, taskEntry, false); err != nil {
		return nil, fmt.Errorf("failed to update task entry: %w", err)
	}
	if err = st.commit(tx); err != nil {
		return nil, err
	}
	return taskEntry, nil
}

// ProcessTaskResult checks task result and updates dependencies in case of success
func (st *TaskStorage) ProcessTaskResult(ctx context.Context, res *types.TaskResult) error {
	return st.retryRunner.Do(ctx, func(ctx context.Context) error {
		return st.processTaskResultImpl(ctx, res)
	})
}

func (st *TaskStorage) processTaskResultImpl(ctx context.Context, res *types.TaskResult) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// First we check the result and set status to failed if unsuccessful
	entry, err := st.extractTaskEntry(tx, res.TaskId)
	if err != nil {
		// ErrKeyNotFound is not considered an error because of possible re-invocations
		if errors.Is(err, db.ErrKeyNotFound) {
			st.logger.Warn().Err(err).Stringer(logging.FieldTaskId, res.TaskId).Msg("Task entry was not found")
			return nil
		}

		return err
	}

	if err := res.ValidateForTask(entry); err != nil {
		return err
	}

	if res.HasRetryableError() {
		if err := st.rescheduleTaskTx(tx, entry, res.Error); err != nil {
			return err
		}

		if err := st.commit(tx); err != nil {
			return err
		}

		st.metrics.RecordTaskRescheduled(ctx, entry.Task.TaskType, res.Sender)
		return nil
	}

	if err := st.terminateTaskTx(tx, entry, res); err != nil {
		return err
	}

	if err := st.commit(tx); err != nil {
		return err
	}

	st.metrics.RecordTaskTerminated(ctx, entry, res)
	return nil
}

func (st *TaskStorage) terminateTaskTx(tx db.RwTx, entry *types.TaskEntry, res *types.TaskResult) error {
	currentTime := st.clock.Now()

	if err := entry.Terminate(res, currentTime); err != nil {
		return err
	}

	log.NewTaskResultEvent(st.logger, zerolog.DebugLevel, res).
		Msgf("Task execution is completed with status %s, removing it from the storage", res.StatusStr())

	// We don't keep finished tasks in DB
	if err := tx.Delete(taskEntriesTable, res.TaskId.Bytes()); err != nil {
		return err
	}

	if !res.IsSuccess() {
		if err := st.putTaskEntry(tx, entry, true); err != nil {
			return err
		}
	}

	return st.updateDependentsTx(tx, entry, res, currentTime)
}

func (st *TaskStorage) updateDependentsTx(
	tx db.RwTx,
	entry *types.TaskEntry,
	res *types.TaskResult,
	currentTime time.Time,
) error {
	for taskId := range entry.Dependents {
		depEntry, err := st.extractTaskEntry(tx, taskId)
		// skip update dependency if it was removed
		if !errors.Is(err, db.ErrKeyNotFound) {
			if err != nil {
				return err
			}

			resultEntry := types.NewTaskResultDetails(res, entry, currentTime)

			if err = depEntry.AddDependencyResult(*resultEntry); err != nil {
				return fmt.Errorf("failed to add dependency result to task with id=%s: %w", depEntry.Task.Id, err)
			}
			err = st.putTaskEntry(tx, depEntry, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type rescheduledTask struct {
	taskType         types.TaskType
	previousExecutor types.TaskExecutorId
}

// RescheduleHangingTasks finds tasks that exceed execution timeout and reschedules them to be re-executed later.
func (st *TaskStorage) RescheduleHangingTasks(ctx context.Context, taskExecutionTimeout time.Duration) error {
	var rescheduled []rescheduledTask
	err := st.retryRunner.Do(ctx, func(ctx context.Context) error {
		var err error
		rescheduled, err = st.rescheduleHangingTasksImpl(ctx, taskExecutionTimeout)
		return err
	})
	if err != nil {
		return err
	}

	for _, entry := range rescheduled {
		st.metrics.RecordTaskRescheduled(ctx, entry.taskType, entry.previousExecutor)
	}
	return nil
}

func (st *TaskStorage) rescheduleHangingTasksImpl(
	ctx context.Context,
	taskExecutionTimeout time.Duration,
) (rescheduled []rescheduledTask, err error) {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentTime := st.clock.Now()

loop:
	for entry, err := range st.getStoredTasksSeq(tx) {
		switch {
		case err != nil:
			return nil, err

		case len(rescheduled) == rescheduledTasksPerTxLimit:
			break loop

		case entry.Status != types.Running:
			continue

		case *entry.ExecutionTime(currentTime) <= taskExecutionTimeout:
			continue

		default:
			previousExecutor := entry.Owner
			timeoutErr := types.NewTaskErrTimeout(*entry.ExecutionTime(currentTime), taskExecutionTimeout)
			if err := st.rescheduleTaskTx(tx, entry, timeoutErr); err != nil {
				return nil, err
			}

			rescheduled = append(rescheduled, rescheduledTask{entry.Task.TaskType, previousExecutor})
		}
	}

	if err := st.commit(tx); err != nil {
		return nil, err
	}

	return rescheduled, nil
}

func (st *TaskStorage) rescheduleTaskTx(
	tx db.RwTx,
	entry *types.TaskEntry,
	cause *types.TaskExecError,
) error {
	log.NewTaskEvent(st.logger, zerolog.WarnLevel, &entry.Task).
		Err(cause).
		Stringer(logging.FieldTaskExecutorId, entry.Owner).
		Int("retryCount", entry.RetryCount).
		Msg("Task execution error, rescheduling")

	if err := entry.ResetRunning(); err != nil {
		return fmt.Errorf("failed to reset task: %w", err)
	}

	if err := st.putTaskEntry(tx, entry, false); err != nil {
		return fmt.Errorf("failed to put rescheduled task: %w", err)
	}

	return nil
}

func (st *TaskStorage) CancelTasksByParentId(
	ctx context.Context,
	isActive func(context.Context, types.TaskId) (bool, error),
) (uint, error) {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var count uint

	for entry, err := range st.getStoredTasksSeq(tx) {
		if err != nil {
			return 0, err
		}

		if entry.Task.ParentTaskId != nil {
			parentTaskIsActive, err := isActive(ctx, *entry.Task.ParentTaskId)
			if err != nil {
				return 0, err
			}
			if !parentTaskIsActive {
				res := types.NewCancelTaskResult(entry.Task.Id, entry.Owner)
				err = st.terminateTaskTx(tx, entry, res)
				if err != nil {
					return 0, err
				}
				count++
			}
		}
	}

	if err := st.commit(tx); err != nil {
		return 0, err
	}

	return count, nil
}

func (*TaskStorage) getStoredTasksSeq(tx db.RoTx) iter.Seq2[*types.TaskEntry, error] {
	return func(yield func(*types.TaskEntry, error) bool) {
		txIter, err := tx.Range(taskEntriesTable, nil, nil)
		if err != nil {
			yield(nil, err)
			return
		}
		defer txIter.Close()

		for txIter.HasNext() {
			key, val, err := txIter.Next()
			if err != nil {
				yield(nil, err)
				return
			}
			entry := &types.TaskEntry{}
			if err = gob.NewDecoder(bytes.NewBuffer(val)).Decode(&entry); err != nil {
				err = fmt.Errorf("%w: failed to decode task with id %v: %w", ErrSerializationFailed, string(key), err)
				yield(nil, err)
				return
			}

			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (*TaskStorage) makeTaskKey(entry *types.TaskEntry) []byte {
	return entry.Task.Id.Bytes()
}
