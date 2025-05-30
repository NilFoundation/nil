package commands

import (
	"maps"
	"slices"
	"strings"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type TaskField = string

var TaskViewFields = map[TaskField]struct {
	Getter           func(task *public.TaskView) string
	IncludeByDefault bool
}{
	"Id":          {func(task *public.TaskView) string { return task.Id.String() }, true},
	"BatchId":     {func(task *public.TaskView) string { return task.BatchId.String() }, false},
	"Type":        {func(task *public.TaskView) string { return task.Type.String() }, true},
	"CircuitType": {func(task *public.TaskView) string { return task.CircuitType.String() }, true},
	"CreatedAt":   {func(task *public.TaskView) string { return task.CreatedAt.Format(output.TimeFormat) }, true},
	"StartedAt": {func(task *public.TaskView) string {
		if task.StartedAt != nil {
			return task.StartedAt.Format(output.TimeFormat)
		}
		return output.EmptyCell
	}, false},
	"ExecutionTime": {func(task *public.TaskView) string {
		if task.ExecutionTime != nil {
			return task.ExecutionTime.String()
		}
		return output.EmptyCell
	}, true},
	"Owner":  {func(task *public.TaskView) string { return task.Owner.String() }, true},
	"Status": {func(task *public.TaskView) string { return task.Status.String() }, true},
}

func AllTaskFields() []TaskField {
	fields := slices.Collect(maps.Keys(TaskViewFields))
	sortTaskFields(fields)
	return fields
}

func DefaultTaskFields() []TaskField {
	var fields []TaskField
	for field, data := range TaskViewFields {
		if data.IncludeByDefault {
			fields = append(fields, field)
		}
	}
	sortTaskFields(fields)
	return fields
}

func sortTaskFields(fields []TaskField) {
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
