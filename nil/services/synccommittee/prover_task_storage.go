package synccommittee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/db"
)

// BadgerDB tables, ProverTaskId is used as a key
const (
	TaskEntriesTable = "TaskEntries"
)

// Interface for storing and accessing tasks from DB
type ProverTaskStorage interface {
	AddTaskEntry(ctx context.Context, entry ProverTaskEntry) error
	RemoveTaskEntry(ctx context.Context, id ProverTaskId) error
	RequestTaskToExecute(ctx context.Context, executor ProverId) (*ProverTask, error)
	ProcessTaskResult(ctx context.Context, res ProverTaskResult) error
	RescheduleTask(ctx context.Context, id ProverTaskId) error
}
type proverTaskStorage struct {
	database db.DB
}

func NewTaskStorage(db db.DB) ProverTaskStorage {
	return &proverTaskStorage{database: db}
}

// Helper to get and decode task entry from DB
func extractTaskEntry(tx db.RwTx, id ProverTaskId) (*ProverTaskEntry, error) {
	encoded, err := tx.Get(TaskEntriesTable, id.Bytes())
	if err != nil {
		return nil, err
	}

	entry := &ProverTaskEntry{}
	if err = gob.NewDecoder(bytes.NewBuffer(encoded)).Decode(&entry); err != nil {
		return nil, fmt.Errorf("failed to decode task with id %v: %w", id, err)
	}
	return entry, nil
}

// Helper to encode and put task entry into DB
func putTaskEntry(tx db.RwTx, entry *ProverTaskEntry) error {
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

// Store new task entry into DB
func (st *proverTaskStorage) AddTaskEntry(ctx context.Context, entry ProverTaskEntry) error {
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

// Delete existing task entry from DB
func (st *proverTaskStorage) RemoveTaskEntry(ctx context.Context, id ProverTaskId) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = tx.Delete(TaskEntriesTable, id.Bytes())
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// Helper to update task status when it's ready to be executed or needs to be rescheduled
func updateTaskStatus(tx db.RwTx, id ProverTaskId, newStatus ProverTaskStatus, newOwner ProverId) error {
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
func findHigherPriorityTask(tx db.RwTx) (*ProverTask, error) {
	var res *ProverTask = nil
	iter, err := tx.Range(TaskEntriesTable, nil, nil)
	if err != nil {
		return res, err
	}
	defer iter.Close()

	// Iterate over DB and check for status and priority of entries
	for iter.HasNext() {
		key, val, err := iter.Next()
		if err != nil {
			return nil, err
		}
		entry := &ProverTaskEntry{}
		if err = gob.NewDecoder(bytes.NewBuffer(val)).Decode(&entry); err != nil {
			return nil, fmt.Errorf("failed to decode task with id %v: %w", string(key), err)
		}
		if entry.Status == WaitingForProver {
			if res == nil || HigherPriority(entry.Task, *res) {
				res = &entry.Task
			}
		}
	}
	return res, nil
}

// Find task with no dependencies and higher priority and assign it to prover
func (st *proverTaskStorage) RequestTaskToExecute(ctx context.Context, executor ProverId) (*ProverTask, error) {
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
	err = updateTaskStatus(tx, resultTask.Id, Running, executor)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return resultTask, nil
}

// Check task result, update dependencies in case of success
func (st *proverTaskStorage) ProcessTaskResult(ctx context.Context, res ProverTaskResult) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// First we check the result and set status to failed if unsuccessful
	entry, err := extractTaskEntry(tx, res.TaskId)
	if err != nil {
		return err
	}
	if res.Err != nil {
		entry.Modified = time.Now()
		entry.Status = Failed
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
			depEntry.Status = WaitingForProver
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

// Make task available for prover requests
func (st *proverTaskStorage) RescheduleTask(ctx context.Context, id ProverTaskId) error {
	tx, err := st.database.CreateRwTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = updateTaskStatus(tx, id, WaitingForProver, UnknownProverId); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}
