package types

import (
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
)

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

type TaskOutputArtifacts map[ProverResultType]string

type TaskResultData []byte

// TaskResult represents the result of a task provided via RPC by the executor with id = TaskResult.Sender.
type TaskResult struct {
	TaskId          TaskId              `json:"taskId"`
	Sender          TaskExecutorId      `json:"sender"`
	Error           *TaskExecError      `json:"error,omitempty"`
	OutputArtifacts TaskOutputArtifacts `json:"dataAddresses,omitempty"`
	Data            TaskResultData      `json:"binaryData,omitempty"`
}

// IsSuccess determines if the task result indicates success.
func (r *TaskResult) IsSuccess() bool {
	return r.Error == nil
}

// HasRetryableError determines if the task result contains an error eligible for retry.
func (r *TaskResult) HasRetryableError() bool {
	return !r.IsSuccess() && r.Error.CanBeRetried()
}

// HasCriticalError returns true if the task result indicates failure and the error cannot be retried.
func (r *TaskResult) HasCriticalError() bool {
	return !r.IsSuccess() && !r.Error.CanBeRetried()
}

func (r *TaskResult) Cancelled() bool {
	return r.Error != nil && r.Error.ErrType == TaskErrCancelled
}

// ValidateForTask checks the correctness of the TaskResult
// against the given TaskEntry and returns an error if invalid.
func (r *TaskResult) ValidateForTask(entry *TaskEntry) error {
	if r.TaskId != entry.Task.Id {
		return fmt.Errorf("task result's taskId=%s does not match task entry's taskId=%s", r.TaskId, entry.Task.Id)
	}

	if r.Cancelled() {
		// Cancellation can be triggered by a different executor for a task in any status
		return nil
	}

	if r.Sender != entry.Owner {
		return fmt.Errorf(
			"%w: taskId=%v, taskStatus=%v, taskOwner=%v, requestSenderId=%v",
			ErrTaskWrongExecutor, entry.Task.Id, entry.Status, entry.Owner, r.Sender,
		)
	}

	if entry.Status != Running {
		return errTaskInvalidStatus(entry, "Validate")
	}

	return nil
}

func NewSuccessProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
	outputArtifacts TaskOutputArtifacts,
	binaryData TaskResultData,
) *TaskResult {
	return &TaskResult{
		TaskId:          taskId,
		Sender:          proofProviderId,
		OutputArtifacts: outputArtifacts,
		Data:            binaryData,
	}
}

func NewFailureProviderTaskResult(
	taskId TaskId,
	proofProviderId TaskExecutorId,
	err *TaskExecError,
) *TaskResult {
	check.PanicIff(err == nil, "err cannot be nil")

	return &TaskResult{
		TaskId: taskId,
		Sender: proofProviderId,
		Error:  err,
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
		Sender:          sender,
		OutputArtifacts: outputArtifacts,
		Data:            binaryData,
	}
}

func NewFailureProverTaskResult(
	taskId TaskId,
	sender TaskExecutorId,
	err *TaskExecError,
) *TaskResult {
	check.PanicIff(err == nil, "err cannot be nil")

	return &TaskResult{
		TaskId: taskId,
		Sender: sender,
		Error:  err,
	}
}

func NewTaskCancelledResult(
	taskId TaskId,
	sender TaskExecutorId,
) *TaskResult {
	return &TaskResult{
		TaskId: taskId,
		Sender: sender,
		Error:  NewTaskExecError(TaskErrCancelled, "task cancellation requested"),
	}
}

// TaskResultDetails represents the result of a task, extending TaskResult with additional task-specific metadata.
type TaskResultDetails struct {
	TaskResult
	TaskType      TaskType      `json:"type"`
	CircuitType   CircuitType   `json:"circuitType"`
	ExecutionTime time.Duration `json:"executionTime"`
}

func NewTaskResultDetails(result *TaskResult, taskEntry *TaskEntry, currentTime time.Time) *TaskResultDetails {
	return &TaskResultDetails{
		TaskResult:    *result,
		TaskType:      taskEntry.Task.TaskType,
		CircuitType:   taskEntry.Task.CircuitType,
		ExecutionTime: *taskEntry.ExecutionTime(currentTime),
	}
}
