package types

import (
	"errors"
	"fmt"
)

var (
	ErrUnexpectedTaskType   = errors.New("unexpected task type")
	ErrBlockProofTaskFailed = errors.New("block proof task failed")
)

func UnexpectedTaskType(task *Task) error {
	return fmt.Errorf("%w: taskType=%d, taskId=%s", ErrUnexpectedTaskType, task.TaskType, task.Id)
}
