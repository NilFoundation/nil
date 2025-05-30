package types

import (
	"encoding/binary"
	"errors"
	"fmt"
	"iter"
	"math/rand"
	"strconv"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/google/uuid"
)

type CircuitType uint8

const (
	None CircuitType = iota
	CircuitBytecode
	CircuitReadWrite
	CircuitZKEVM
	CircuitCopy

	CircuitAmount     uint8 = iota - 1
	CircuitStartIndex uint8 = uint8(CircuitBytecode)
)

func Circuits() iter.Seq[CircuitType] {
	return func(yield func(CircuitType) bool) {
		for i := range CircuitAmount {
			if !yield(CircuitType(i + CircuitStartIndex)) {
				return
			}
		}
	}
}

// TaskId Unique ID of a task, serves as a key in DB
type TaskId uuid.UUID

func NewTaskId() TaskId          { return TaskId(uuid.New()) }
func (id TaskId) String() string { return uuid.UUID(id).String() }
func (id TaskId) Bytes() []byte  { return []byte(id.String()) }

// MarshalText implements the encoding.TextMarshller interface for TaskId.
func (id TaskId) MarshalText() ([]byte, error) {
	uuidValue := uuid.UUID(id)
	return []byte(uuidValue.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for TaskId.
func (id *TaskId) UnmarshalText(data []byte) error {
	uuidValue, err := uuid.Parse(string(data))
	if err != nil {
		return err
	}
	*id = TaskId(uuidValue)
	return nil
}

func (id *TaskId) Set(str string) error {
	parsed, err := uuid.Parse(str)
	if err != nil {
		return fmt.Errorf("invalid UUID '%s': %w", str, err)
	}

	*id = TaskId(parsed)
	return nil
}

func (*TaskId) Type() string {
	return "TaskId"
}

type TaskExecutorId uint32

const UnknownExecutorId TaskExecutorId = 0

func NewRandomExecutorId() TaskExecutorId {
	var randInt uint32
	for randInt == 0 {
		randInt = rand.Uint32() //nolint:gosec
	}
	return TaskExecutorId(randInt)
}

func (id TaskExecutorId) MarshalBinary() ([]byte, error) {
	intValue := uint32(id)
	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, intValue)
	return value, nil
}

func (id *TaskExecutorId) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		return fmt.Errorf("invalid bytes length for TaskExecutorId, got %d, expected 4", len(data))
	}
	intValue := binary.LittleEndian.Uint32(data)
	*id = TaskExecutorId(intValue)
	return nil
}

func (id TaskExecutorId) String() string {
	return strconv.FormatUint(uint64(id), 10)
}

func (id *TaskExecutorId) Set(str string) error {
	parsedValue, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return fmt.Errorf("%w: invalid value for TaskExecutorId, got %s", err, str)
	}
	*id = TaskExecutorId(parsedValue)
	return nil
}

func (*TaskExecutorId) Type() string {
	return "TaskExecutorId"
}

type TaskIdSet map[TaskId]bool

func NewTaskIdSet() TaskIdSet {
	return make(TaskIdSet)
}

func (s TaskIdSet) Put(id TaskId) {
	s[id] = true
}

// todo: declare separate task types for ProofProvider and Prover
// https://www.notion.so/nilfoundation/Generic-Tasks-in-SyncCommittee-10ac614852608028b7ffcfd910deeef7?pvs=4

// Task contains all the necessary data for either Prover or ProofProvider to perform computation
type Task struct {
	Id           TaskId      `json:"id"`
	BatchId      BatchId     `json:"batchId"`
	BlockIds     []BlockId   `json:"blockIds"`
	TaskType     TaskType    `json:"taskType"`
	CircuitType  CircuitType `json:"circuitType"`
	ParentTaskId *TaskId     `json:"parentTaskId"`

	// DependencyResults tracks the set of task results on which current task depends
	DependencyResults map[TaskId]TaskResultDetails `json:"dependencyResults"`
}

// TaskEntry Wrapper for task to hold metadata like task status and dependencies
type TaskEntry struct {
	// Task: task to be executed
	Task Task

	// Dependents: list of tasks which depend on the current one
	Dependents TaskIdSet

	// PendingDependencies tracks the set of not completed dependencies
	PendingDependencies TaskIdSet

	// Created: task object creation time
	Created time.Time

	// Started: time when the executor acquired the task for execution
	Started *time.Time

	// Finished time when the task execution was completed (successfully or not)
	Finished *time.Time

	// Owner: identifier of the current task executor
	Owner TaskExecutorId

	// Status: current status of the task
	Status TaskStatus

	// RetryCount specifies the number of times the task execution has been retried
	RetryCount int
}

// AddDependency adds a dependency to the current task entry and updates the dependents and pending dependencies.
func (t *TaskEntry) AddDependency(dependency *TaskEntry) {
	check.PanicIfNotf(dependency != nil, "dependency cannot be nil")

	if dependency.Dependents == nil {
		dependency.Dependents = NewTaskIdSet()
	}
	dependency.Dependents.Put(t.Task.Id)

	if t.PendingDependencies == nil {
		t.PendingDependencies = NewTaskIdSet()
	}
	t.PendingDependencies.Put(dependency.Task.Id)
}

// AddDependencyResult updates the task's dependency result and adjusts pending dependencies
// and task status accordingly.
func (t *TaskEntry) AddDependencyResult(res TaskResultDetails) error {
	if t.PendingDependencies == nil || !t.PendingDependencies[res.TaskId] {
		return fmt.Errorf("task with id=%s has no pending dependency with id=%s", t.Task.Id, res.TaskId)
	}

	if t.Task.DependencyResults == nil {
		t.Task.DependencyResults = make(map[TaskId]TaskResultDetails)
	}
	t.Task.DependencyResults[res.TaskId] = res

	if res.IsSuccess() {
		delete(t.PendingDependencies, res.TaskId)
	}
	if len(t.PendingDependencies) == 0 {
		t.Status = WaitingForExecutor
	}

	return nil
}

// Start assigns an executor to a task and changes its status from WaitingForExecutor to Running.
// It requires a non-zero executorId and only transitions tasks that are in WaitingForExecutor status.
// Returns an error if the executorId is unknown or if the task has an invalid status.
func (t *TaskEntry) Start(executorId TaskExecutorId, currentTime time.Time) error {
	if executorId == UnknownExecutorId {
		return errors.New("unknown executor id")
	}
	if t.Status != WaitingForExecutor {
		return errTaskInvalidStatus(t, "Start")
	}

	t.Status = Running
	t.Owner = executorId
	t.Started = &currentTime
	return nil
}

// Terminate transitions the status of a running task to Completed or Failed based on the input.
func (t *TaskEntry) Terminate(result *TaskResult, currentTime time.Time) error {
	if err := result.ValidateForTask(t); err != nil {
		return err
	}

	var newStatus TaskStatus
	if result.IsSuccess() {
		newStatus = Completed
	} else {
		newStatus = Failed
	}

	t.Status = newStatus
	t.Finished = &currentTime
	return nil
}

// ResetRunning resets a task's status from Running to WaitingForExecutor, clearing its start time
// and executor ownership.
func (t *TaskEntry) ResetRunning() error {
	if t.Status != Running {
		return errTaskInvalidStatus(t, "ResetRunning")
	}

	t.Started = nil
	t.Status = WaitingForExecutor
	t.Owner = UnknownExecutorId
	t.RetryCount++
	return nil
}

func errTaskInvalidStatus(task *TaskEntry, methodName string) error {
	return fmt.Errorf("%w: id=%s, status=%s, operation=%s", ErrTaskInvalidStatus, task.Task.Id, task.Status, methodName)
}

func (t *TaskEntry) ExecutionTime(currentTime time.Time) *time.Duration {
	if t.Started == nil {
		return nil
	}
	var rightBound time.Time
	if t.Finished == nil {
		rightBound = currentTime
	} else {
		rightBound = *t.Finished
	}
	execTime := rightBound.Sub(*t.Started)
	return &execTime
}

// HasHigherPriorityThan determines if the current task has a higher priority than another one.
func (t *TaskEntry) HasHigherPriorityThan(other *TaskEntry) bool {
	if other == nil {
		return true
	}

	if !t.Created.Equal(other.Created) {
		return t.Created.Before(other.Created)
	}
	return t.Task.TaskType < other.Task.TaskType
}

// AsNewChildEntry creates a new TaskEntry with a new TaskId and sets the ParentTaskId to the current task's Id.
func (t *Task) AsNewChildEntry(currentTime time.Time) *TaskEntry {
	newTask := common.CopyPtr(t)
	newTask.Id = NewTaskId()
	newTask.ParentTaskId = &t.Id

	return &TaskEntry{
		Task:    *newTask,
		Status:  WaitingForExecutor,
		Created: currentTime,
	}
}

func NewBatchProofTaskEntry(
	batchId BatchId, blockIds []BlockId, currentTime time.Time,
) (*TaskEntry, error) {
	if len(blockIds) == 0 {
		return nil, fmt.Errorf("batch with id=%s is empty, cannot create proof task", batchId)
	}

	task := Task{
		Id:       NewTaskId(),
		BatchId:  batchId,
		BlockIds: blockIds,
		TaskType: ProofBatch,
	}

	batchProofEntry := &TaskEntry{
		Task:    task,
		Created: currentTime,
		Status:  WaitingForExecutor,
	}

	return batchProofEntry, nil
}

func NewPartialProveTaskEntry(
	batchId BatchId,
	blockIds []BlockId,
	circuitType CircuitType,
	currentTime time.Time,
) *TaskEntry {
	task := Task{
		Id:          NewTaskId(),
		BatchId:     batchId,
		BlockIds:    blockIds,
		TaskType:    PartialProve,
		CircuitType: circuitType,
	}
	return &TaskEntry{
		Task:    task,
		Created: currentTime,
		Status:  WaitingForExecutor,
	}
}

func NewAggregateChallengeTaskEntry(
	batchId BatchId,
	currentTime time.Time,
) *TaskEntry {
	aggChallengeTask := Task{
		Id:       NewTaskId(),
		BatchId:  batchId,
		TaskType: AggregatedChallenge,
	}

	return &TaskEntry{
		Task:    aggChallengeTask,
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewCombinedQTaskEntry(
	batchId BatchId,
	circuitType CircuitType,
	currentTime time.Time,
) *TaskEntry {
	combinedQTask := Task{
		Id:          NewTaskId(),
		BatchId:     batchId,
		CircuitType: circuitType,
		TaskType:    CombinedQ,
	}

	return &TaskEntry{
		Task:    combinedQTask,
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewAggregateFRITaskEntry(
	batchId BatchId,
	currentTime time.Time,
) *TaskEntry {
	aggFRITask := Task{
		Id:       NewTaskId(),
		BatchId:  batchId,
		TaskType: AggregatedFRI,
	}

	return &TaskEntry{
		Task:    aggFRITask,
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewFRIConsistencyCheckTaskEntry(
	batchId BatchId,
	circuitType CircuitType,
	currentTime time.Time,
) *TaskEntry {
	task := Task{
		Id:          NewTaskId(),
		BatchId:     batchId,
		TaskType:    FRIConsistencyChecks,
		CircuitType: circuitType,
	}
	return &TaskEntry{
		Task:    task,
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewMergeProofTaskEntry(
	batchId BatchId,
	currentTime time.Time,
) *TaskEntry {
	mergeProofTask := Task{
		Id:       NewTaskId(),
		BatchId:  batchId,
		TaskType: MergeProof,
	}

	return &TaskEntry{
		Task:    mergeProofTask,
		Created: currentTime,
		Status:  WaitingForInput,
	}
}
