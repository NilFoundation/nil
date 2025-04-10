package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
)

var (
	ErrBlockNotFound = errors.New("block with the specified id is not found")
	ErrBatchNotFound = errors.New("batch with the specified id is not found")
)

var (
	ErrTaskInvalidStatus = errors.New("task has invalid status")
	ErrTaskWrongExecutor = errors.New("task belongs to another executor")
)

type TaskErrType int8

const (
	_ TaskErrType = iota

	// TaskErrTimeout indicates that a task failed as it exceeded the specified timeout duration.
	TaskErrTimeout

	// TaskErrRpc indicates an error related to remote procedure calls (RPC) during task execution.
	TaskErrRpc

	// TaskErrIO indicates an error related to input/output operations (Disk storage access / Network).
	TaskErrIO

	// TaskErrInvalidTask indicates an error caused by an invalid or improperly structured task.
	TaskErrInvalidTask

	// TaskErrInvalidInputData indicates an error caused by invalid or malformed input data to a task.
	TaskErrInvalidInputData

	// TaskErrProofGenerationFailed indicates that a proof generation stage failed due to block inconsistency.
	TaskErrProofGenerationFailed

	// TaskErrTerminated indicates that the task was explicitly terminated prior to completion.
	TaskErrTerminated

	// TaskErrNotSupportedType indicates that an unsupported or unrecognized type was encountered by the executor.
	TaskErrNotSupportedType

	// TaskErrOutOfMemory indicated that managed memory allocation failure occurred during the proof generation
	TaskErrOutOfMemory

	// TaskErrCancelled indicated that child task was cancelled becuse parent task not exists any more
	TaskErrCancelled

	// TaskErrUnknown indicates an unspecified task error.
	TaskErrUnknown
)

var RetryableErrors = map[TaskErrType]bool{
	TaskErrTimeout:     true,
	TaskErrRpc:         true,
	TaskErrIO:          true,
	TaskErrTerminated:  true,
	TaskErrOutOfMemory: true,
	TaskErrUnknown:     true,
}

type TaskExecError struct {
	ErrType TaskErrType `json:"errCode"`
	ErrText string      `json:"errText"`
}

func (e *TaskExecError) Error() string {
	return fmt.Sprintf("%s: %s", e.ErrType, e.ErrText)
}

func (e *TaskExecError) CanBeRetried() bool {
	return RetryableErrors[e.ErrType]
}

func NewTaskExecError(errType TaskErrType, errText string) *TaskExecError {
	return &TaskExecError{ErrType: errType, ErrText: errText}
}

func NewTaskExecErrorf(errType TaskErrType, format string, args ...interface{}) *TaskExecError {
	return &TaskExecError{ErrType: errType, ErrText: fmt.Sprintf(format, args...)}
}

func NewTaskErrTimeout(execTime, execTimeout time.Duration) *TaskExecError {
	return NewTaskExecErrorf(
		TaskErrTimeout,
		"execution timeout exceeded: execTime=%s, execTimeout=%s", execTime, execTimeout,
	)
}

func NewTaskErrNotSupportedType(taskType TaskType) *TaskExecError {
	return NewTaskExecErrorf(TaskErrNotSupportedType, "taskType=%s", taskType)
}

func NewTaskErrChildFailed(childResult *TaskResult) *TaskExecError {
	check.PanicIff(childResult.IsSuccess(), "childResult is not failed")

	return NewTaskExecErrorf(
		childResult.Error.ErrType,
		"child prover task failed: childTaskId=%s, errorText=%s", childResult.TaskId, childResult.Error.ErrText,
	)
}

func NewTaskErrUnknown(cause error) *TaskExecError {
	return NewTaskExecErrorf(TaskErrUnknown, "%s", cause)
}
