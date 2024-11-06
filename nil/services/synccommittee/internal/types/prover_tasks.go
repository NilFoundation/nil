package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
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

// BatchId Unique ID of a batch of blocks.
type BatchId uuid.UUID

var EmptyBatchId = BatchId{}

func NewBatchId() BatchId         { return BatchId(uuid.New()) }
func (id BatchId) String() string { return uuid.UUID(id).String() }
func (id BatchId) Bytes() []byte  { return []byte(id.String()) }

// MarshalText implements the encoding.TextMarshller interface for BatchId.
func (b BatchId) MarshalText() ([]byte, error) {
	uuidValue := uuid.UUID(b)
	return []byte(uuidValue.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for BatchId.
func (b *BatchId) UnmarshalText(data []byte) error {
	uuidValue, err := uuid.Parse(string(data))
	if err != nil {
		return err
	}
	*b = BatchId(uuidValue)
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
	Id            TaskId                `json:"id"`
	BatchId       BatchId               `json:"batchId"`
	ShardId       types.ShardId         `json:"shardId"`
	BlockNum      types.BlockNumber     `json:"blockNum"`
	BlockHash     common.Hash           `json:"blockHash"`
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
	Started     *time.Time
	Owner       TaskExecutorId
	Status      TaskStatus
}

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

func (t *TaskEntry) ResetRunning() error {
	if t.Status != Running {
		return fmt.Errorf("task with id=%s has invalid status: %s", t.Task.Id, t.Status)
	}

	t.Started = nil
	t.Status = WaitingForExecutor
	t.Owner = UnknownExecutorId
	return nil
}

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
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       mainShardBlock.ShardId,
		BlockNum:      mainShardBlock.Number,
		BlockHash:     mainShardBlock.Hash,
		TaskType:      AggregateProofs,
		Dependencies:  EmptyDependencies(),
		DependencyNum: uint8(len(mainShardBlock.ChildBlocks)),
	}
	return &TaskEntry{
		Task:    task,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}

func NewBlockProofTaskEntry(batchId BatchId, parentTaskId TaskId, blockHash common.Hash) *TaskEntry {
	task := Task{
		Id:            NewTaskId(),
		BatchId:       batchId,
		BlockHash:     blockHash,
		TaskType:      ProofBlock,
		ParentTaskId:  &parentTaskId,
		Dependencies:  EmptyDependencies(),
		DependencyNum: 0,
	}
	return &TaskEntry{
		Task:        task,
		Created:     time.Now(),
		Status:      WaitingForExecutor,
		PendingDeps: []TaskId{parentTaskId},
	}
}

func NewPartialProveTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	circuitType CircuitType,
) *TaskEntry {
	task := Task{
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       shardId,
		BlockNum:      blockNum,
		BlockHash:     blockHash,
		TaskType:      PartialProve,
		CircuitType:   circuitType,
		Dependencies:  EmptyDependencies(),
		DependencyNum: 0,
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
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       shardId,
		BlockNum:      blockNum,
		BlockHash:     blockHash,
		TaskType:      AggregatedChallenge,
		DependencyNum: CircuitAmount,
		Dependencies:  EmptyDependencies(),
	}

	return &TaskEntry{
		Task:    aggChallengeTask,
		Created: time.Now(),

		Status: WaitingForInput,
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
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       shardId,
		BlockNum:      blockNum,
		BlockHash:     blockHash,
		CircuitType:   circuitType,
		TaskType:      CombinedQ,
		DependencyNum: 2, // partial prove of corresponding circuit plus agg challenges
		Dependencies:  EmptyDependencies(),
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
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       shardId,
		BlockNum:      blockNum,
		BlockHash:     blockHash,
		TaskType:      AggregatedFRI,
		DependencyNum: CircuitAmount*2 + 1, // all the partial proofs, combinedQ and agg challenges
		Dependencies:  EmptyDependencies(),
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
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       shardId,
		BlockNum:      blockNum,
		BlockHash:     blockHash,
		TaskType:      FRIConsistencyChecks,
		CircuitType:   circuitType,
		Dependencies:  EmptyDependencies(),
		DependencyNum: 3, // aggregate FRI and corresponding partial proof and combinedQ
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
		Id:            NewTaskId(),
		BatchId:       batchId,
		ShardId:       shardId,
		BlockNum:      blockNum,
		BlockHash:     blockHash,
		TaskType:      MergeProof,
		DependencyNum: 1 + CircuitAmount*2, // agg FRI + 4 partial proofs + 4 FRI consistency checks
		Dependencies:  EmptyDependencies(),
	}

	return &TaskEntry{
		Task:    mergeProofTask,
		Created: time.Now(),
		Status:  WaitingForInput,
	}
}
