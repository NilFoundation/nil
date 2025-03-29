package commands

import (
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type TaskField = string

const (
	timeFormat = time.RFC3339
	emptyCell  = "nil"
)

var TaskViewFields = map[TaskField]struct {
	Getter           func(task *public.TaskView) string
	IncludeByDefault bool
}{
	"Id":          {func(task *public.TaskView) string { return task.Id.String() }, true},
	"BatchId":     {func(task *public.TaskView) string { return task.BatchId.String() }, false},
	"Type":        {func(task *public.TaskView) string { return task.Type.String() }, true},
	"CircuitType": {func(task *public.TaskView) string { return task.CircuitType.String() }, true},
	"CreatedAt":   {func(task *public.TaskView) string { return task.CreatedAt.Format(timeFormat) }, true},
	"StartedAt": {func(task *public.TaskView) string {
		if task.StartedAt != nil {
			return task.StartedAt.Format(timeFormat)
		}
		return emptyCell
	}, false},
	"ExecutionTime": {func(task *public.TaskView) string {
		if task.ExecutionTime != nil {
			return task.ExecutionTime.String()
		}
		return emptyCell
	}, true},
	"Owner":  {func(task *public.TaskView) string { return task.Owner.String() }, true},
	"Status": {func(task *public.TaskView) string { return task.Status.String() }, true},
}

func AllFields() []TaskField {
	fields := slices.Collect(maps.Keys(TaskViewFields))
	sortFields(fields)
	return fields
}

func DefaultFields() []TaskField {
	var fields []TaskField
	for field, data := range TaskViewFields {
		if data.IncludeByDefault {
			fields = append(fields, field)
		}
	}
	sortFields(fields)
	return fields
}

func sortFields(fields []TaskField) {
	slices.SortFunc(fields, func(l, r TaskField) int {
		switch {
		// The `Id` field always goes first
		case l == "Id":
			return -1
		case r == "Id":
			return 1
		// All others are sorted alphabetically
		default:
			return strings.Compare(l, r)
		}
	})
}
