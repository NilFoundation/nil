package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/google/uuid"
)

// TaskType Tasks have different types, it affects task input and priority
type TaskType uint8

const (
	_ TaskType = iota
	AggregateProofs
	ProofBlock
	PartialProve
	AggregatedChallenge
	CombinedQ
	AggregatedFRI
	FRIConsistencyChecks
	MergeProof
)

type CircuitType uint8

const (
	None CircuitType = iota
	Bytecode
	ReadWrite
	MPT
	ZKEVM

	CircuitAmount uint8 = iota - 1
)

// TaskId Unique ID of a task, serves as a key in DB
type TaskId uuid.UUID

func NewTaskId() TaskId          { return TaskId(uuid.New()) }
func (id TaskId) String() string { return uuid.UUID(id).String() }
func (id TaskId) Bytes() []byte  { return []byte(id.String()) }

// MarshalText implements the encoding.TextMarshller interface for TaskId.
func (t TaskId) MarshalText() ([]byte, error) {
	uuidValue := uuid.UUID(t)
	return []byte(uuidValue.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for TaskId.
func (t *TaskId) UnmarshalText(data []byte) error {
	uuidValue, err := uuid.Parse(string(data))
	if err != nil {
		return err
	}
	*t = TaskId(uuidValue)
	return nil
}

// Task results can have different types
type ProverResultType uint8

const (
	_ ProverResultType = iota
	PartialProof
	CommitmentState
	PartialProofChallenges
	AssignmentTableDescription
	AggregatedChallenges
	CombinedQPolynomial
	AggregatedFRIProof
	ProofOfWork
	ConsistencyCheckChallenges
	LPCConsistencyCheckProof
	FinalProof
	BlockProof
	AggregatedProof
)

type TaskExecutorId uint32

const UnknownExecutorId TaskExecutorId = 0

type TaskResultAddresses map[ProverResultType]string

type TaskIdSet map[TaskId]bool

func NewTaskIdSet() TaskIdSet {
	return make(TaskIdSet)
}

func (s TaskIdSet) Put(id TaskId) {
	s[id] = true
}

// todo: declare separate task types for ProofProvider and Prover
// https://www.notion.so/nilfoundation/Generic-Tasks-in-SyncCommittee-10ac614852608028b7ffcfd910deeef7?pvs=4
type TaskResultData []byte

// TaskResult Prover returns this struct as task result
type TaskResult struct {
	TaskId        TaskId              `json:"taskId"`
	Type          TaskType            `json:"type"`
	IsSuccess     bool                `json:"isSuccess"`
	ErrorText     string              `json:"errorText"`
	Sender        TaskExecutorId      `json:"sender"`
	DataAddresses TaskResultAddresses `json:"dataAddresses"`
	Data          TaskResultData      `json:"binaryData"`
}

func SuccessProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
	taskType TaskType,
	dataAddresses TaskResultAddresses,
	binaryData TaskResultData,
) TaskResult {
	return TaskResult{
		TaskId:        taskId,
		IsSuccess:     true,
		Sender:        proofProviderId,
		Type:          taskType,
		DataAddresses: dataAddresses,
		Data:          binaryData,
	}
}

func FailureProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
	err error,
) TaskResult {
	return TaskResult{
		TaskId:    taskId,
		IsSuccess: false,
		Sender:    proofProviderId,
		ErrorText: fmt.Sprintf("failed to proof block: %v", err),
	}
}

func SuccessProverTaskResult(
	taskId TaskId,
	sender TaskExecutorId,
	taskType TaskType,
	dataAddresses TaskResultAddresses,
	binaryData TaskResultData,
) TaskResult {
	return TaskResult{
		TaskId:        taskId,
		IsSuccess:     true,
		Sender:        sender,
		Type:          taskType,
		DataAddresses: dataAddresses,
		Data:          binaryData,
	}
}

func FailureProverTaskResult(
	taskId TaskId,
	sender TaskExecutorId,
	err error,
) TaskResult {
	return TaskResult{
		TaskId:        taskId,
		Sender:        sender,
		DataAddresses: TaskResultAddresses{},
		IsSuccess:     false,
		ErrorText:     fmt.Sprintf("failed to generate proof: %v", err),
	}
}

// Task contains all the necessary data for either Prover or ProofProvider to perform computation
type Task struct {
	Id           TaskId            `json:"id"`
	BatchId      BatchId           `json:"batchId"`
	ShardId      types.ShardId     `json:"shardId"`
	BlockNum     types.BlockNumber `json:"blockNum"`
	BlockHash    common.Hash       `json:"blockHash"`
	TaskType     TaskType          `json:"taskType"`
	CircuitType  CircuitType       `json:"circuitType"`
	ParentTaskId *TaskId           `json:"parentTaskId"`

	// DependencyResults tracks the set of task results on which current task depends
	DependencyResults map[TaskId]TaskResult `json:"dependencyResults"`
	// PendingDependencies tracks the set of not completed dependencies
	PendingDependencies TaskIdSet `json:"pendingDependencies"`
}

type TaskStatus uint8

const (
	WaitingForInput TaskStatus = iota
	WaitingForExecutor
	Running
	Failed
)

// TaskEntry Wrapper for task to hold metadata like task status and dependencies
type TaskEntry struct {
	// Task: task to be executed
	Task Task `json:"task"`

	// Dependents: list of tasks which depend on the current one
	Dependents TaskIdSet `json:"dependents"`

	// Created: task object creation time
	Created time.Time `json:"created"`

	// Started: time when the executor acquired the task for execution
	Started *time.Time `json:"started"`

	// Finished time when the task execution was completed (successfully or not)
	Finished *time.Time `json:"finished"`

	// Owner: identifier of the current task executor
	Owner TaskExecutorId `json:"owner"`

	// Status: current status of the task
	Status TaskStatus `json:"status"`
}

// TaskTree represents a full hierarchical structure of tasks with dependencies among them.
type TaskTree struct {
	TaskEntry    TaskEntry   `json:"task"`
	Result       *TaskResult `json:"taskResult"`
	Dependencies []*TaskTree `json:"dependencies"`
}

func NewTaskTree(entry *TaskEntry) *TaskTree {
	return &TaskTree{
		TaskEntry:    *entry,
		Result:       nil,
		Dependencies: make([]*TaskTree, 0),
	}
}

func (t *TaskTree) AddDependency(dependency *TaskTree) {
	t.Dependencies = append(t.Dependencies, dependency)
}

// AddDependency adds a dependency to the current task entry and updates the dependents and pending dependencies.
func (t *TaskEntry) AddDependency(dependency *TaskEntry) {
	check.PanicIfNotf(dependency != nil, "dependency cannot be nil")

	if dependency.Dependents == nil {
		dependency.Dependents = NewTaskIdSet()
	}
	dependency.Dependents.Put(t.Task.Id)

	if t.Task.PendingDependencies == nil {
		t.Task.PendingDependencies = NewTaskIdSet()
	}
	t.Task.PendingDependencies.Put(dependency.Task.Id)
}

// AddDependencyResult updates the task's dependency result and adjusts pending dependencies and task status accordingly.
func (t *TaskEntry) AddDependencyResult(res TaskResult) error {
	if t.Task.PendingDependencies == nil || !t.Task.PendingDependencies[res.TaskId] {
		return fmt.Errorf("task with id=%s has no pending dependency with id=%s", t.Task.Id, res.TaskId)
	}

	if t.Task.DependencyResults == nil {
		t.Task.DependencyResults = make(map[TaskId]TaskResult)
	}
	t.Task.DependencyResults[res.TaskId] = res

	if res.IsSuccess {
		delete(t.Task.PendingDependencies, res.TaskId)
	}
	if len(t.Task.PendingDependencies) == 0 {
		t.Status = WaitingForExecutor
	}

	return nil
}

// Start assigns an executor to a task and changes its status from WaitingForExecutor to Running.
// It requires a non-zero executorId and only transitions tasks that are in WaitingForExecutor status.
// Returns an error if the executorId is unknown or if the task has an invalid status.
func (t *TaskEntry) Start(executorId TaskExecutorId) error {
	if executorId == UnknownExecutorId {
		return errors.New("unknown executor id")
	}
	if t.Status != WaitingForExecutor {
		return fmt.Errorf("task with id=%s has invalid status: %s", t.Task.Id, t.Status)
	}

	t.Status = Running
	t.Owner = executorId
	now := time.Now()
	t.Started = &now
	return nil
}

// ResetRunning resets a task's status from Running to WaitingForExecutor, clearing its start time and executor ownership.
func (t *TaskEntry) ResetRunning() error {
	if t.Status != Running {
		return fmt.Errorf("task with id=%s has invalid status: %s", t.Task.Id, t.Status)
	}

	t.Started = nil
	t.Status = WaitingForExecutor
	t.Owner = UnknownExecutorId
	return nil
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

// AsNewChildEntry creates a new TaskEntry with a new TaskId and sets the ParentTaskId to the current task's Id.
func (t *Task) AsNewChildEntry() *TaskEntry {
	newTask := common.CopyPtr(t)
	newTask.Id = NewTaskId()
	newTask.ParentTaskId = &t.Id

	return &TaskEntry{
		Task:    *newTask,
		Status:  WaitingForExecutor,
		Created: time.Now(),
	}
}

// HigherPriority Priority comparator for tasks
func HigherPriority(t1, t2 *TaskEntry) bool {
	// AggregateProofs task can be created later thant DFRI step tasks for the next batch
	if t1.Task.TaskType != t2.Task.TaskType && t1.Task.TaskType == AggregateProofs {
		return true
	}
	if t1.Created != t2.Created {
		return t1.Created.Before(t2.Created)
	}
	return t1.Task.TaskType < t2.Task.TaskType
}

func NewAggregateProofsTaskEntry(batchId BatchId, mainShardBlock *jsonrpc.RPCBlock) *TaskEntry {
	task := Task{
		Id:        NewTaskId(),
		BatchId:   batchId,
		ShardId:   mainShardBlock.ShardId,
		BlockNum:  mainShardBlock.Number,
		BlockHash: mainShardBlock.Hash,
		TaskType:  AggregateProofs,
	}
	return &TaskEntry{
		Task:    task,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}

func NewBlockProofTaskEntry(batchId BatchId, aggregateProofsTask *TaskEntry, execShardBlock *jsonrpc.RPCBlock) (*TaskEntry, error) {
	if aggregateProofsTask == nil {
		return nil, errors.New("aggregateProofsTask cannot be nil")
	}
	if aggregateProofsTask.Task.TaskType != AggregateProofs {
		return nil, fmt.Errorf("aggregateProofsTask has invalid type: %s", aggregateProofsTask.Task.TaskType)
	}
	if execShardBlock == nil {
		return nil, errors.New("execShardBlock cannot be nil")
	}

	task := Task{
		Id:           NewTaskId(),
		BatchId:      batchId,
		ShardId:      execShardBlock.ShardId,
		BlockNum:     execShardBlock.Number,
		BlockHash:    execShardBlock.Hash,
		TaskType:     ProofBlock,
		ParentTaskId: &aggregateProofsTask.Task.Id,
	}
	blockProofEntry := &TaskEntry{
		Task:    task,
		Created: time.Now(),
		Status:  WaitingForExecutor,
	}

	aggregateProofsTask.AddDependency(blockProofEntry)
	return blockProofEntry, nil
}

func NewPartialProveTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	circuitType CircuitType,
) *TaskEntry {
	task := Task{
		Id:          NewTaskId(),
		BatchId:     batchId,
		ShardId:     shardId,
		BlockNum:    blockNum,
		BlockHash:   blockHash,
		TaskType:    PartialProve,
		CircuitType: circuitType,
	}
	return &TaskEntry{
		Task:    task,
		Created: time.Now(),
		Status:  WaitingForExecutor,
	}
}

func NewAggregateChallengeTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
) *TaskEntry {
	aggChallengeTask := Task{
		Id:        NewTaskId(),
		BatchId:   batchId,
		ShardId:   shardId,
		BlockNum:  blockNum,
		BlockHash: blockHash,
		TaskType:  AggregatedChallenge,
	}

	return &TaskEntry{
		Task:    aggChallengeTask,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}

func NewCombinedQTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	circuitType CircuitType,
) *TaskEntry {
	combinedQTask := Task{
		Id:          NewTaskId(),
		BatchId:     batchId,
		ShardId:     shardId,
		BlockNum:    blockNum,
		BlockHash:   blockHash,
		CircuitType: circuitType,
		TaskType:    CombinedQ,
	}

	return &TaskEntry{
		Task:    combinedQTask,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}

func NewAggregateFRITaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
) *TaskEntry {
	aggFRITask := Task{
		Id:        NewTaskId(),
		BatchId:   batchId,
		ShardId:   shardId,
		BlockNum:  blockNum,
		BlockHash: blockHash,
		TaskType:  AggregatedFRI,
	}

	return &TaskEntry{
		Task:    aggFRITask,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}

func NewFRIConsistencyCheckTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	circuitType CircuitType,
) *TaskEntry {
	task := Task{
		Id:          NewTaskId(),
		BatchId:     batchId,
		ShardId:     shardId,
		BlockNum:    blockNum,
		BlockHash:   blockHash,
		TaskType:    FRIConsistencyChecks,
		CircuitType: circuitType,
	}
	return &TaskEntry{
		Task:    task,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}

func NewMergeProofTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
) *TaskEntry {
	mergeProofTask := Task{
		Id:        NewTaskId(),
		BatchId:   batchId,
		ShardId:   shardId,
		BlockNum:  blockNum,
		BlockHash: blockHash,
		TaskType:  MergeProof,
	}

	return &TaskEntry{
		Task:    mergeProofTask,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}
