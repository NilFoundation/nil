package storage

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
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
	"github.com/rs/zerolog"
)

const (
	// taskEntriesTable is used for storing task entries.
	// Key: scTypes.TaskId (task's own id), Value: scTypes.TaskEntry.
	taskEntriesTable db.TableName = "task_entries"

	// blockParentIdxTable is used for indexing tasks by their parent batch ids.
	// Key: scTypes.BatchId (task's parent batch id), Value: scTypes.TaskIdSet (task identifiers);
	taskParentBatchIdxTable db.TableName = "task_parent_batch_idx"
)

const (
	// rescheduledTasksPerTxLimit defines the maximum number of tasks that can be rescheduled
	// in a single transaction of TaskStorage.RescheduleHangingTasks.
	rescheduledTasksPerTxLimit = 100
)

type TaskStorageMetrics interface {
	RecordTaskAdded(ctx context.Context, task *types.TaskEntry)
	RecordTaskStarted(ctx context.Context, taskEntry *types.TaskEntry)
	RecordTaskTerminated(ctx context.Context, taskEntry *types.TaskEntry, taskResult *types.TaskResult)
	RecordTaskRescheduled(ctx context.Context, taskType types.TaskType, previousExecutor types.TaskExecutorId)
}

// TaskStorage defines a type for managing tasks and their lifecycle operations.
type TaskStorage struct {
	commonStorage
	timer   common.Timer
	metrics TaskStorageMetrics
}

func NewTaskStorage(
	db db.DB,
	timer common.Timer,
	metrics TaskStorageMetrics,
	logger zerolog.Logger,
) *TaskStorage {
	return &TaskStorage{
		commonStorage: makeCommonStorage(
			db,
			logger,
			common.DoNotRetryIf(
				types.ErrTaskWrongExecutor, types.ErrTaskInvalidStatus, types.ErrTaskNotFound, ErrTaskAlreadyExists,
			),
		),
		timer:   timer,
		metrics: metrics,
	}
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

	return st.putTaskEntry(tx, entry)
}

// TryGetTaskEntry Retrieve a task entry by its id. In case if task does not exist, method returns nil
func (st *TaskStorage) TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error) {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return st.getTaskEntry(tx, id, false)
}

// GetTaskViews Retrieve tasks that match the given predicate function and pushes them to the destination container.
func (st *TaskStorage) GetTaskViews(ctx context.Context, destination interface{ Add(task *public.TaskView) }, predicate func(*public.TaskView) bool) error {
	tx, err := st.database.CreateRoTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	currentTime := st.timer.NowTime()

	err = st.iterateOverTaskEntries(tx, func(entry *types.TaskEntry) (bool, error) {
		taskView := public.NewTaskView(entry, currentTime)
		if predicate(taskView) {
			destination.Add(taskView)
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve tasks based on predicate: %w", err)
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

	currentTime := st.timer.NowTime()

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

		entry, err := st.getTaskEntry(tx, taskId, false)

		if entry == nil && taskId == rootTaskId {
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
			tree.AddDependency(subtree)
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
	var topPriorityTask *types.TaskEntry = nil

	err := st.iterateOverTaskEntries(tx, func(entry *types.TaskEntry) (bool, error) {
		if entry.Status != types.WaitingForExecutor {
			return true, nil
		}

		if entry.HasHigherPriorityThan(topPriorityTask) {
			topPriorityTask = entry
		}

		return true, nil
	})

	return topPriorityTask, err
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

	st.metrics.RecordTaskStarted(ctx, taskEntry)
	return &taskEntry.Task, nil
}

func (st *TaskStorage) requestTaskToExecuteImpl(ctx context.Context, executor types.TaskExecutorId) (*types.TaskEntry, error) {
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

	currentTime := st.timer.NowTime()
	if err := taskEntry.Start(executor, currentTime); err != nil {
		return nil, fmt.Errorf("failed to start task: %w", err)
	}
	if err := st.putTaskEntry(tx, taskEntry); err != nil {
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

	entry, err := st.getTaskEntry(tx, res.TaskId, false)
	if entry == nil {
		st.logger.Warn().Err(err).Stringer(logging.FieldTaskId, res.TaskId).Msg("Task entry was not found")
		return nil
	}
	if err != nil {
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

	if res.HasCriticalError() {
		err := st.cancelNextBatchesTasks(tx, entry.Task.BatchId, res.Sender)
		if err != nil {
			return fmt.Errorf("failed to cancel tasks starting from batchId=%s: %w", entry.Task.BatchId, err)
		}
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
	currentTime := st.timer.NowTime()

	if err := entry.Terminate(res, currentTime); err != nil {
		return err
	}

	if res.IsSuccess() {
		// We don't keep finished tasks in DB
		log.NewTaskResultEvent(st.logger, zerolog.DebugLevel, res).
			Msg("Task execution is completed successfully, removing it from the storage")

		if err := st.deleteTaskTx(tx, entry); err != nil {
			return err
		}
	} else if err := st.putTaskEntry(tx, entry); err != nil {
		return err
	}

	if err := st.updateDependentsTx(tx, entry, res, currentTime); err != nil {
		return err
	}

	return nil
}

func (st *TaskStorage) cancelNextBatchesTasks(
	tx db.RwTx,
	batchId types.BatchId,
	failedExecutor types.TaskExecutorId,
) error {
	for entry, err := range st.getBatchTasksSeqTx(tx, batchId) {
		if err != nil {
			return err
		}
		if err := st.cancelTaskTx(tx, entry, failedExecutor); err != nil {
			return fmt.Errorf("failed to cancel task with id=%s: %w", entry.Task.Id, err)
		}
	}
	return nil
}

func (st *TaskStorage) cancelTaskTx(tx db.RwTx, entry *types.TaskEntry, initiator types.TaskExecutorId) error {
	result := types.NewTaskCancelledResult(entry.Task.Id, initiator)
	return st.terminateTaskTx(tx, entry, result)
}

// getBatchTasksSeqTx traverses tasks tree in BFS using parentBatchId as a starting point.
// It uses taskParentBatchIdxTable to retrieve parent-child connections between tasks.
func (st *TaskStorage) getBatchTasksSeqTx(tx db.RoTx, parentBatchId types.BatchId) iter.Seq2[*types.TaskEntry, error] {
	seen := make(map[types.BatchId]bool)

	return func(yield func(*types.TaskEntry, error) bool) {
		bfsQueue := []types.BatchId{parentBatchId}

		for len(bfsQueue) > 0 {
			nextParentId := bfsQueue[0]
			bfsQueue = bfsQueue[1:]
			seen[nextParentId] = true

			taskIds, err := st.getTaskIdsFromIndexTx(tx, nextParentId)
			if err != nil {
				yield(nil, fmt.Errorf("failed to get parent idx entry, parentId=%s: %w", nextParentId, err))
				return
			}

			for taskId := range taskIds.Values() {
				taskEntry, err := st.getTaskEntry(tx, taskId, true)
				if err != nil {
					yield(nil, err)
					return
				}
				if !yield(taskEntry, nil) {
					return
				}

				if !seen[taskEntry.Task.BatchId] {
					bfsQueue = append(bfsQueue, taskEntry.Task.BatchId)
				}
			}
		}
	}
}

func (st *TaskStorage) updateDependentsTx(
	tx db.RwTx,
	entry *types.TaskEntry,
	res *types.TaskResult,
	currentTime time.Time,
) error {
	for taskId := range entry.Dependents {
		depEntry, err := st.getTaskEntry(tx, taskId, true)
		if err != nil {
			return err
		}

		resultEntry := types.NewTaskResultDetails(res, entry, currentTime)

		if err = depEntry.AddDependencyResult(*resultEntry); err != nil {
			return fmt.Errorf("failed to add dependency result to task with id=%s: %w", depEntry.Task.Id, err)
		}
		err = st.putTaskEntry(tx, depEntry)
		if err != nil {
			return err
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

	currentTime := st.timer.NowTime()

	err = st.iterateOverTaskEntries(tx, func(entry *types.TaskEntry) (bool, error) {
		if entry.Status != types.Running {
			return true, nil
		}

		executionTime := currentTime.Sub(*entry.Started)
		if executionTime <= taskExecutionTimeout {
			return true, nil
		}

		previousExecutor := entry.Owner
		timeoutErr := types.NewTaskErrTimeout(executionTime, taskExecutionTimeout)
		if err := st.rescheduleTaskTx(tx, entry, timeoutErr); err != nil {
			return false, err
		}

		rescheduled = append(rescheduled, rescheduledTask{entry.Task.TaskType, previousExecutor})
		shouldContinue := len(rescheduled) < rescheduledTasksPerTxLimit
		return shouldContinue, nil
	})
	if err != nil {
		return nil, err
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

	if err := st.putTaskEntry(tx, entry); err != nil {
		return fmt.Errorf("failed to put rescheduled task: %w", err)
	}

	return nil
}

func (*TaskStorage) iterateOverTaskEntries(
	tx db.RoTx,
	action func(entry *types.TaskEntry) (shouldContinue bool, err error),
) error {
	txIter, err := tx.Range(taskEntriesTable, nil, nil)
	if err != nil {
		return err
	}
	defer txIter.Close()

	for txIter.HasNext() {
		key, val, err := txIter.Next()
		if err != nil {
			return err
		}
		entry := &types.TaskEntry{}
		if err = gob.NewDecoder(bytes.NewBuffer(val)).Decode(&entry); err != nil {
			return fmt.Errorf("%w: failed to decode task with id %v: %w", ErrSerializationFailed, string(key), err)
		}
		shouldContinue, err := action(entry)
		if err != nil {
			return err
		}
		if !shouldContinue {
			return nil
		}
	}

	return nil
}

func (st *TaskStorage) getTaskEntry(tx db.RoTx, id types.TaskId, required bool) (*types.TaskEntry, error) {
	return st.getTaskEntryBytesId(tx, id.Bytes(), required)
}

// Helper to get and decode task entry from DB
func (*TaskStorage) getTaskEntryBytesId(tx db.RoTx, idBytes []byte, required bool) (*types.TaskEntry, error) {
	encoded, err := tx.Get(taskEntriesTable, idBytes)
	hexId := func() string {
		return hex.EncodeToString(idBytes)
	}
	switch {
	case err == nil:
		break
	case errors.Is(err, db.ErrKeyNotFound) && required:
		return nil, fmt.Errorf("%w, id=%s", types.ErrTaskNotFound, hexId())
	case errors.Is(err, db.ErrKeyNotFound):
		return nil, nil
	default:
		return nil, fmt.Errorf("failed to get task with id=%s: %w", hexId(), err)
	}

	entry := &types.TaskEntry{}
	if err = gob.NewDecoder(bytes.NewBuffer(encoded)).Decode(&entry); err != nil {
		return nil, fmt.Errorf("%w: failed to decode task with id %s: %w", ErrSerializationFailed, hexId(), err)
	}
	return entry, nil
}

// Helper to encode and put task entry into DB
func (st *TaskStorage) putTaskEntry(tx db.RwTx, entry *types.TaskEntry) error {
	var inputBuffer bytes.Buffer
	err := gob.NewEncoder(&inputBuffer).Encode(entry)
	if err != nil {
		return fmt.Errorf("%w: failed to encode task with id %s: %w", ErrSerializationFailed, entry.Task.Id, err)
	}
	key := st.makeTaskKey(entry)
	if err := tx.Put(taskEntriesTable, key, inputBuffer.Bytes()); err != nil {
		return fmt.Errorf("failed to put task with id %s: %w", entry.Task.Id, err)
	}

	if err := st.putToBatchIndexTx(tx, entry); err != nil {
		return err
	}

	return nil
}

func (st *TaskStorage) deleteTaskTx(tx db.RwTx, entry *types.TaskEntry) error {
	if err := tx.Delete(taskEntriesTable, entry.Task.Id.Bytes()); err != nil {
		return fmt.Errorf("failed to delete task with id=%s: %w", entry.Task.Id, err)
	}

	if err := st.deleteFromBatchIndexTx(tx, entry); err != nil {
		return err
	}

	return nil
}

func (st *TaskStorage) putToBatchIndexTx(tx db.RwTx, entry *types.TaskEntry) error {
	if entry.Task.ParentBatchId == nil {
		return nil
	}

	taskIds, err := st.getTaskIdsFromIndexTx(tx, *entry.Task.ParentBatchId)
	if err != nil {
		return err
	}

	taskIds.Put(entry.Task.Id)

	return st.putTaskIdsToIndexTx(tx, *entry.Task.ParentBatchId, taskIds)
}

func (st *TaskStorage) deleteFromBatchIndexTx(tx db.RwTx, entry *types.TaskEntry) error {
	if entry.Task.ParentBatchId == nil {
		return nil
	}

	taskIds, err := st.getTaskIdsFromIndexTx(tx, *entry.Task.ParentBatchId)
	if err != nil {
		return err
	}

	delete(taskIds, entry.Task.Id)

	if len(taskIds) > 0 {
		return st.putTaskIdsToIndexTx(tx, *entry.Task.ParentBatchId, taskIds)
	}

	parentBatchIdBytes := entry.Task.ParentBatchId.Bytes()
	if err := tx.Delete(taskParentBatchIdxTable, parentBatchIdBytes); err != nil {
		return fmt.Errorf("failed to delete idx entry, key %s: %w", entry.Task.ParentBatchId, err)
	}

	return nil
}

func (st *TaskStorage) putTaskIdsToIndexTx(tx db.RwTx, parentBatchId types.BatchId, taskIds types.TaskIdSet) error {
	newSetBytes, err := taskIds.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal parent idx entry, key=%s: %w", parentBatchId, err)
	}

	parentBatchIdBytes := parentBatchId.Bytes()
	if err := tx.Put(taskParentBatchIdxTable, parentBatchIdBytes, newSetBytes); err != nil {
		return fmt.Errorf("failed to put idx entry, key %s: %w", parentBatchId, err)
	}
	return nil
}

func (st *TaskStorage) getTaskIdsFromIndexTx(tx db.RoTx, parentBatchId types.BatchId) (types.TaskIdSet, error) {
	parentBatchIdBytes := parentBatchId.Bytes()

	setBytes, err := tx.Get(taskParentBatchIdxTable, parentBatchIdBytes)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed to get parent idx entry, parentId=%s: %w", parentBatchId, err)
	}
	var taskIds types.TaskIdSet
	if err := taskIds.UnmarshalBinary(setBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parent idx entry, parentId=%s: %w", parentBatchId, err)
	}
	return taskIds, nil
}

func (*TaskStorage) makeTaskKey(entry *types.TaskEntry) []byte {
	return entry.Task.Id.Bytes()
}
