package types

import (
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/google/uuid"
)

// TaskType Tasks have different types, it affects task input and priority
type TaskType uint8

const (
	_ TaskType = iota
	ProofBlock
	GenerateAssignment
	Preprocess
	PartialProve
	AggregatedFRI
	FRIConsistencyChecks
	MergeProof
)

type CircuitType uint8

const (
	Bytecode CircuitType = iota
	ReadWrite
	MPT
	ZKEVM
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
	PreprocessedCommonData
	Preprocessed
	PartialProof
	Commitment
	CommitmentPoly
	CommitmentState
	PartialAggregatedFriProof
	Challenges
	FriConsistencyProof
	FinalProof
)

type TaskExecutorId uint32

const UnknownExecutorId TaskExecutorId = 0

// todo: declare separate task types for ProofProvider and Prover
// https://www.notion.so/nilfoundation/Generic-Tasks-in-SyncCommittee-10ac614852608028b7ffcfd910deeef7?pvs=4

// TaskResult Prover returns this struct as task result
type TaskResult struct {
	Type        ProverResultType `json:"type"`
	TaskId      TaskId           `json:"taskId"`
	IsSuccess   bool             `json:"isSuccess"`
	ErrorText   string           `json:"errorText"`
	Sender      TaskExecutorId   `json:"sender"`
	DataAddress string           `json:"dataAddress"`
}

func SuccessProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
) TaskResult {
	return TaskResult{
		TaskId:    taskId,
		IsSuccess: true,
		Sender:    proofProviderId,
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
	resultType ProverResultType,
	dataAddress string,
) TaskResult {
	return TaskResult{
		TaskId:      taskId,
		IsSuccess:   true,
		Sender:      sender,
		Type:        resultType,
		DataAddress: dataAddress,
	}
}

func FailureProverTaskResult(
	taskId TaskId,
	sender TaskExecutorId,
	err error,
) TaskResult {
	return TaskResult{
		TaskId:    taskId,
		Sender:    sender,
		IsSuccess: false,
		ErrorText: fmt.Sprintf("failed to generate proof: %v", err),
	}
}

// Task contains all the necessary data for either Prover or ProofProvider to perform computation
type Task struct {
	Id            TaskId                `json:"id"`
	BatchNum      uint32                `json:"batchNum"`
	BlockNum      types.BlockNumber     `json:"blockNum"`
	TaskType      TaskType              `json:"taskType"`
	CircuitType   CircuitType           `json:"circuitType"`
	ParentTaskId  *TaskId               `json:"parentTaskId"`
	Dependencies  map[TaskId]TaskResult `json:"dependencies"`
	DependencyNum uint8                 `json:"dependencyNum"`
}

func (t *Task) AddDependencyResult(res TaskResult) {
	t.Dependencies[res.TaskId] = res
}

func EmptyDependencies() map[TaskId]TaskResult {
	return make(map[TaskId]TaskResult)
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
	Task        Task
	PendingDeps []TaskId
	Created     time.Time
	Modified    time.Time
	Owner       TaskExecutorId
	Status      TaskStatus
}

// HigherPriority Priority comparator for tasks
func HigherPriority(t1 Task, t2 Task) bool {
	if t1.BatchNum != t2.BatchNum {
		return t1.BatchNum < t2.BatchNum
	}
	if t1.BlockNum != t2.BlockNum {
		return t1.BlockNum < t2.BlockNum
	}
	return t1.TaskType < t2.TaskType
}

func NewBlockProofTaskEntry(blockNum types.BlockNumber) *TaskEntry {
	task := Task{
		Id:            NewTaskId(),
		BlockNum:      blockNum,
		TaskType:      ProofBlock,
		Dependencies:  EmptyDependencies(),
		DependencyNum: 0,
	}
	return &TaskEntry{
		Task:     task,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForExecutor,
	}
}

func NewPartialProveTaskEntry(batchNum uint32, blockNum types.BlockNumber, circuitType CircuitType) *TaskEntry {
	task := Task{
		Id:            NewTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      PartialProve,
		CircuitType:   circuitType,
		Dependencies:  EmptyDependencies(),
		DependencyNum: 0,
	}
	return &TaskEntry{
		Task:     task,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForExecutor,
	}
}

func NewAggregateFRITaskEntry(batchNum uint32, blockNum types.BlockNumber) *TaskEntry {
	aggFRITask := Task{
		Id:            NewTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      AggregatedFRI,
		DependencyNum: 4,
		Dependencies:  EmptyDependencies(),
	}

	return &TaskEntry{
		Task:     aggFRITask,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForInput,
	}
}

func NewFRIConsistencyCheckTaskEntry(batchNum uint32, blockNum types.BlockNumber, circuitType CircuitType) *TaskEntry {
	task := Task{
		Id:            NewTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      FRIConsistencyChecks,
		CircuitType:   circuitType,
		Dependencies:  EmptyDependencies(),
		DependencyNum: 2, // aggregate FRI and corresponding partial proof
	}
	return &TaskEntry{
		Task:     task,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForInput,
	}
}

func NewMergeProofTaskEntry(batchNum uint32, blockNum types.BlockNumber) *TaskEntry {
	mergeProofTask := Task{
		Id:            NewTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      MergeProof,
		DependencyNum: 9, // agg FRI + 4 partial proofs + 4 FRI consistency checks
		Dependencies:  EmptyDependencies(),
	}

	return &TaskEntry{
		Task:     mergeProofTask,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForInput,
	}
}
