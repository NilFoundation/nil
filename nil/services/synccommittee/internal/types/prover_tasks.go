package types

import (
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/google/uuid"
)

// Prover tasks have different types, it affects task input and priority
type ProverTaskType uint8

const (
	GenerateAssignment ProverTaskType = iota
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

// Unique ID of a task, serves as a key in DB
type ProverTaskId uuid.UUID

func NewProverTaskId() ProverTaskId    { return ProverTaskId(uuid.New()) }
func (id ProverTaskId) String() string { return uuid.UUID(id).String() }
func (id ProverTaskId) Bytes() []byte  { return []byte(id.String()) }

// MarshalText implements the encoding.TextMarshaler interface for ProverTaskId.
func (t ProverTaskId) MarshalText() ([]byte, error) {
	uuidValue := uuid.UUID(t)
	return []byte(uuidValue.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for ProverTaskId.
func (t *ProverTaskId) UnmarshalText(data []byte) error {
	uuidValue, err := uuid.Parse(string(data))
	if err != nil {
		return err
	}
	*t = ProverTaskId(uuidValue)
	return nil
}

// Task results can have different types
type ProverResultType uint8

const (
	PreprocessedCommonData ProverResultType = iota
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

const UnknownProverId TaskExecutorId = 0

// Prover returns this struct as task result
type TaskResult struct {
	Type        ProverResultType `json:"type"`
	TaskId      ProverTaskId     `json:"taskId"`
	IsSuccess   bool             `json:"isSuccess"`
	ErrorText   string           `json:"errorText"`
	Sender      TaskExecutorId   `json:"sender"`
	DataAddress string           `json:"dataAddress"`
}

func SuccessTaskResult(
	taskId ProverTaskId,
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

func FailureTaskResult(
	taskId ProverTaskId,
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

// Task contains all the necessary data for a prover to perform computation
type ProverTask struct {
	Id            ProverTaskId                `json:"id"`
	BatchNum      uint32                      `json:"batchNum"`
	BlockNum      types.BlockNumber           `json:"blockNum"`
	TaskType      ProverTaskType              `json:"taskType"`
	CircuitType   CircuitType                 `json:"circuitType"`
	Dependencies  map[ProverTaskId]TaskResult `json:"dependencies"`
	DependencyNum uint8                       `json:"dependencyNum"`
}

func (t *ProverTask) AddDependencyResult(res TaskResult) {
	t.Dependencies[res.TaskId] = res
}

type ProverTaskStatus uint8

const (
	WaitingForInput ProverTaskStatus = iota
	WaitingForProver
	Running
	Failed
)

// This is a wrapper for task to hold metadata like task status and dependencies
type ProverTaskEntry struct {
	Task        ProverTask
	PendingDeps []ProverTaskId
	Created     time.Time
	Modified    time.Time
	Owner       TaskExecutorId
	Status      ProverTaskStatus
}

// Priority comparator for tasks
func HigherPriority(t1 ProverTask, t2 ProverTask) bool {
	if t1.BatchNum != t2.BatchNum {
		return t1.BatchNum < t2.BatchNum
	}
	if t1.BlockNum != t2.BlockNum {
		return t1.BlockNum < t2.BlockNum
	}
	return t1.TaskType < t2.TaskType
}

func NewPartialProveTaskEntry(batchNum uint32, blockNum types.BlockNumber, circuitType CircuitType) *ProverTaskEntry {
	task := ProverTask{
		Id:            NewProverTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      PartialProve,
		CircuitType:   circuitType,
		Dependencies:  make(map[ProverTaskId]TaskResult),
		DependencyNum: 0,
	}
	return &ProverTaskEntry{
		Task:     task,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForProver,
	}
}

func NewAggregateFRITaskEntry(batchNum uint32, blockNum types.BlockNumber) *ProverTaskEntry {
	aggFRITask := ProverTask{
		Id:            NewProverTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      AggregatedFRI,
		DependencyNum: 4,
		Dependencies:  make(map[ProverTaskId]TaskResult),
	}

	return &ProverTaskEntry{
		Task:     aggFRITask,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForInput,
	}
}

func NewFRIConsistencyCheckTaskEntry(batchNum uint32, blockNum types.BlockNumber, circuitType CircuitType) *ProverTaskEntry {
	task := ProverTask{
		Id:            NewProverTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      FRIConsistencyChecks,
		CircuitType:   circuitType,
		Dependencies:  make(map[ProverTaskId]TaskResult),
		DependencyNum: 2, // aggregate FRI and corresponding partial proof
	}
	return &ProverTaskEntry{
		Task:     task,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForInput,
	}
}

func NewMergeProofTaskEntry(batchNum uint32, blockNum types.BlockNumber) *ProverTaskEntry {
	mergeProofTask := ProverTask{
		Id:            NewProverTaskId(),
		BatchNum:      batchNum,
		BlockNum:      blockNum,
		TaskType:      MergeProof,
		DependencyNum: 9, // agg FRI + 4 partial proofs + 4 FRI consistency checks
		Dependencies:  make(map[ProverTaskId]TaskResult),
	}

	return &ProverTaskEntry{
		Task:     mergeProofTask,
		Created:  time.Now(),
		Modified: time.Now(),
		Status:   WaitingForInput,
	}
}
