package types

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strconv"
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
	TaskTypeNone TaskType = iota
	AggregateProofs
	ProofBlock
	PartialProve
	AggregatedChallenge
	CombinedQ
	AggregatedFRI
	FRIConsistencyChecks
	MergeProof
)

var TaskTypes = map[string]TaskType{
	"AggregateProofs":      AggregateProofs,
	"ProofBlock":           ProofBlock,
	"PartialProve":         PartialProve,
	"AggregatedChallenge":  AggregatedChallenge,
	"CombinedQ":            CombinedQ,
	"AggregatedFRI":        AggregatedFRI,
	"FRIConsistencyChecks": FRIConsistencyChecks,
	"MergeProof":           MergeProof,
}

func (t *TaskType) Set(str string) error {
	if v, ok := TaskTypes[str]; ok {
		*t = v
		return nil
	}
	return fmt.Errorf("unknown task type: %s", str)
}

func (*TaskType) Type() string {
	return "TaskType"
}

func (*TaskType) PossibleValues() []string {
	return slices.Collect(maps.Keys(TaskTypes))
}

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

// Task results can have different types
type ProverResultType uint8

const (
	_ ProverResultType = iota
	PartialProof
	CommitmentState
	PartialProofChallenges
	AssignmentTableDescription
	ThetaPower
	AggregatedThetaPowers
	PreprocessedCommonData
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

func (e TaskExecutorId) String() string {
	return strconv.FormatUint(uint64(e), 10)
}

func (e *TaskExecutorId) Set(str string) error {
	parsedValue, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return fmt.Errorf("%w: invalid value for TaskExecutorId, got %s", err, str)
	}
	*e = TaskExecutorId(parsedValue)
	return nil
}

func (*TaskExecutorId) Type() string {
	return "TaskExecutorId"
}

type TaskOutputArtifacts map[ProverResultType]string

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

// TaskResult represents the result of a task provided via RPC by the executor with id = TaskResult.Sender.
type TaskResult struct {
	TaskId          TaskId              `json:"taskId"`
	IsSuccess       bool                `json:"isSuccess"`
	ErrorText       string              `json:"errorText,omitempty"`
	Sender          TaskExecutorId      `json:"sender"`
	OutputArtifacts TaskOutputArtifacts `json:"dataAddresses,omitempty"`
	Data            TaskResultData      `json:"binaryData,omitempty"`
}

func NewSuccessProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
	outputArtifacts TaskOutputArtifacts,
	binaryData TaskResultData,
) *TaskResult {
	return &TaskResult{
		TaskId:          taskId,
		IsSuccess:       true,
		Sender:          proofProviderId,
		OutputArtifacts: outputArtifacts,
		Data:            binaryData,
	}
}

func NewFailureProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
	err error,
) *TaskResult {
	return &TaskResult{
		TaskId:    taskId,
		IsSuccess: false,
		Sender:    proofProviderId,
		ErrorText: fmt.Sprintf("failed to proof block: %v", err),
	}
}

func NewSuccessProverTaskResult(
	taskId TaskId,
	sender TaskExecutorId,
	outputArtifacts TaskOutputArtifacts,
	binaryData TaskResultData,
) *TaskResult {
	return &TaskResult{
		TaskId:          taskId,
		IsSuccess:       true,
		Sender:          sender,
		OutputArtifacts: outputArtifacts,
		Data:            binaryData,
	}
}

func NewFailureProverTaskResult(
	taskId TaskId,
	sender TaskExecutorId,
	err error,
) *TaskResult {
	return &TaskResult{
		TaskId:    taskId,
		Sender:    sender,
		IsSuccess: false,
		ErrorText: fmt.Sprintf("failed to generate proof: %v", err),
	}
}

// TaskResultDetails represents the result of a task, extending TaskResult with additional task-specific metadata.
type TaskResultDetails struct {
	TaskResult
	TaskType      TaskType      `json:"type"`
	CircuitType   CircuitType   `json:"circuitType"`
	ExecutionTime time.Duration `json:"executionTime"`
}

func NewTaskResultEntry(result *TaskResult, taskEntry *TaskEntry, currentTime time.Time) *TaskResultDetails {
	return &TaskResultDetails{
		TaskResult:    *result,
		TaskType:      taskEntry.Task.TaskType,
		CircuitType:   taskEntry.Task.CircuitType,
		ExecutionTime: *taskEntry.ExecutionTime(currentTime),
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
	DependencyResults map[TaskId]TaskResultDetails `json:"dependencyResults"`
}

type TaskStatus uint8

const (
	TaskStatusNone TaskStatus = iota
	WaitingForInput
	WaitingForExecutor
	Running
	Failed
	Completed
)

var TaskStatuses = map[string]TaskStatus{
	"WaitingForInput":    WaitingForInput,
	"WaitingForExecutor": WaitingForExecutor,
	"Running":            Running,
	"Failed":             Failed,
}

func (t *TaskStatus) Set(str string) error {
	if v, ok := TaskStatuses[str]; ok {
		*t = v
		return nil
	}
	return fmt.Errorf("unknown task status: %s", str)
}

func (*TaskStatus) Type() string {
	return "TaskStatus"
}

func (*TaskStatus) PossibleValues() []string {
	return slices.Collect(maps.Keys(TaskStatuses))
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

// AddDependencyResult updates the task's dependency result and adjusts pending dependencies and task status accordingly.
func (t *TaskEntry) AddDependencyResult(res TaskResultDetails) error {
	if t.PendingDependencies == nil || !t.PendingDependencies[res.TaskId] {
		return fmt.Errorf("task with id=%s has no pending dependency with id=%s", t.Task.Id, res.TaskId)
	}

	if t.Task.DependencyResults == nil {
		t.Task.DependencyResults = make(map[TaskId]TaskResultDetails)
	}
	t.Task.DependencyResults[res.TaskId] = res

	if res.IsSuccess {
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
	if t.Owner != result.Sender {
		return fmt.Errorf(
			"%w: taskId=%v, taskStatus=%v, taskOwner=%v, requestSenderId=%v",
			ErrTaskWrongExecutor, t.Task.Id, t.Status, t.Owner, result.Sender,
		)
	}

	if t.Status != Running {
		return errTaskInvalidStatus(t, "Terminate")
	}

	var newStatus TaskStatus
	if result.IsSuccess {
		newStatus = Completed
	} else {
		newStatus = Failed
	}

	t.Status = newStatus
	t.Finished = &currentTime
	return nil
}

// ResetRunning resets a task's status from Running to WaitingForExecutor, clearing its start time and executor ownership.
func (t *TaskEntry) ResetRunning() error {
	if t.Status != Running {
		return errTaskInvalidStatus(t, "ResetRunning")
	}

	t.Started = nil
	t.Status = WaitingForExecutor
	t.Owner = UnknownExecutorId
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

func NewAggregateProofsTaskEntry(
	batchId BatchId, mainShardBlock *jsonrpc.RPCBlock, currentTime time.Time,
) *TaskEntry {
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
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewBlockProofTaskEntry(
	batchId BatchId, aggregateProofsTask *TaskEntry, execShardBlock *jsonrpc.RPCBlock, currentTime time.Time,
) (*TaskEntry, error) {
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
		Created: currentTime,
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
	currentTime time.Time,
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
		Created: currentTime,
		Status:  WaitingForExecutor,
	}
}

func NewAggregateChallengeTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	currentTime time.Time,
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
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewCombinedQTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	circuitType CircuitType,
	currentTime time.Time,
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
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewAggregateFRITaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	currentTime time.Time,
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
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewFRIConsistencyCheckTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	circuitType CircuitType,
	currentTime time.Time,
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
		Created: currentTime,
		Status:  WaitingForInput,
	}
}

func NewMergeProofTaskEntry(
	batchId BatchId,
	shardId types.ShardId,
	blockNum types.BlockNumber,
	blockHash common.Hash,
	currentTime time.Time,
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
		Created: currentTime,
		Status:  WaitingForInput,
	}
}
