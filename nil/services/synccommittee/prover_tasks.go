package synccommittee

import (
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
)

// Prover tasks have different types, it affects task input and priority
type ProverTaskType uint8

const (
	GenerateAssignment ProverTaskType = iota
	Preprocess
	PartialProof
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

// Task results can have different types
type ProverResultType uint8

const (
	AssignmentTable ProverResultType = iota
	IntermediateProof
	FRIConsistencyProof
	FinalProof
)

type ProverId uint32

// Prover returns this struct as task result
type ProverTaskResult struct {
	Type   ProverResultType
	TaskId ProverTaskId
	Err    error
	Sender ProverId
	Data   []byte
}

// Task contains all the necessary data for a prover to perform computation
type ProverTask struct {
	Id           ProverTaskId
	BatchNum     uint32
	BlockNum     types.BlockNumber
	TaskType     ProverTaskType
	CircuitType  CircuitType
	Dependencies []ProverTaskResult
}

func (t *ProverTask) AddDependencyResult(res ProverTaskResult) {
	t.Dependencies = append(t.Dependencies, res)
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

// Declarations below will be moved into ProverTaskStorage

// BadgerDB tables, ProverTaskId will be used as a key
const (
	TaskEntriesTable         = "TaskEntries"
	ReadyToExecuteTasksTable = "ReadyToExecute"
)

// Provers write results into this channel.
// Then the separate goroutine collects results and updates task status
var ResultChannel chan ProverTaskResult

// Both TaskEntries and ReadyToExecute tables must be guarded by a mutex,
// since we have multiple actors: observer, provers, and result handler
var ProverTaskMutex sync.Mutex
