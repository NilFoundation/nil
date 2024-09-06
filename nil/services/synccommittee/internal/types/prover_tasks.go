package types

import (
	"strconv"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
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
type ProverTaskId uint32

const InvalidProverTaskId ProverTaskId = 0

func (id ProverTaskId) String() string { return strconv.FormatUint(uint64(id), 10) }
func (id ProverTaskId) Bytes() []byte  { return []byte(id.String()) }

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

type ProverId uint32

const UnknownProverId ProverId = 0

// Prover returns this struct as task result
type ProverTaskResult struct {
	Type        ProverResultType `json:"type"`
	TaskId      ProverTaskId     `json:"taskId"`
	Err         error            `json:"err"`
	Sender      ProverId         `json:"sender"`
	DataAddress string           `json:"dataAddress"`
}

func SuccessTaskResult(
	taskId ProverTaskId,
	sender ProverId,
	resultType ProverResultType,
	dataAddress string,
) ProverTaskResult {
	return ProverTaskResult{
		Type:        resultType,
		TaskId:      taskId,
		Sender:      sender,
		DataAddress: dataAddress,
	}
}

func FailureTaskResult(
	taskId ProverTaskId,
	sender ProverId,
	err error,
) ProverTaskResult {
	return ProverTaskResult{
		TaskId: taskId,
		Sender: sender,
		Err:    err,
	}
}

// Task contains all the necessary data for a prover to perform computation
type ProverTask struct {
	Id            ProverTaskId                      `json:"id"`
	BatchNum      uint32                            `json:"batchNum"`
	BlockNum      types.BlockNumber                 `json:"blockNum"`
	TaskType      ProverTaskType                    `json:"taskType"`
	CircuitType   CircuitType                       `json:"circuitType"`
	Dependencies  map[ProverTaskId]ProverTaskResult `json:"dependencies"`
	DependencyNum uint8                             `json:"dependencyNum"`
}

func (t *ProverTask) AddDependencyResult(res ProverTaskResult) {
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
	Owner       ProverId
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
