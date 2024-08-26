package synccommitteeservice

import (
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common"
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

// Currently we are going to store intermediate results on separate node.
// This type should serve as an address of the data on that node. For now we use just id
type InputAddress uint64

// Input address must be able to be computed from a task result
func getResultAddress(result ProverTaskResult) InputAddress {
	return InputAddress(result.proverId)
}

// Depending on task type, the input must contains either shardId+blockHash,
// or stored input address, or maybe some arbitrary data
type ProverTaskInput struct {
	shardId      types.ShardId
	blockHash    common.Hash
	storedInputs []InputAddress
	data         []byte
}

// Unique ID of a task, serves as a key in DB
type ProverTaskId uint32

// Task contains all the necessary data for a prover to perform computation
type ProverTask struct {
	id          ProverTaskId
	batchNum    uint32
	blockNum    types.BlockNumber
	taskType    ProverTaskType
	circuitType CircuitType
	inputs      []ProverTaskInput
}

type ProverTaskStatus uint8

const (
	WaitingForInput ProverTaskStatus = iota
	WaitingForProver
	Running
)

type ProverTaskEntry struct {
	task        ProverTask
	pendingDeps []ProverTaskId
	isExecuting bool
	created     time.Time
	modified    time.Time
	status      ProverTaskStatus
}

type ProverTaskResult struct {
	taskId   ProverTaskId
	err      error
	proverId uint32
	data     []byte
}

// Priority comparator for tasks
func higherPriority(t1 ProverTask, t2 ProverTask) bool {
	if t1.batchNum != t2.batchNum {
		return t1.batchNum < t2.batchNum
	}
	if t1.blockNum != t2.blockNum {
		return t1.blockNum < t2.blockNum
	}
	return t1.taskType < t2.taskType
}

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
