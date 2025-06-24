package types

import (
	"fmt"
	"maps"
	"slices"
)

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
