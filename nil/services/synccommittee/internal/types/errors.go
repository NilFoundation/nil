package types

import (
	"errors"
	"fmt"
)

var (
	ErrBlockMismatch   = errors.New("block mismatch")
	ErrBlockProcessing = errors.New("block processing error")
)

var (
	ErrTaskInvalidStatus    = errors.New("task has invalid status")
	ErrTaskWrongExecutor    = errors.New("task belongs to another executor")
	ErrUnexpectedTaskType   = errors.New("unexpected task type")
	ErrBlockProofTaskFailed = errors.New("block proof task failed")
)

func UnexpectedTaskType(task *Task) error {
	return fmt.Errorf("%w: taskType=%d, taskId=%s", ErrUnexpectedTaskType, task.TaskType, task.Id)
}
