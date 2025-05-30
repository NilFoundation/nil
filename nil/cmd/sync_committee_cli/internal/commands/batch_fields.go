package commands

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type BatchField = string

var BatchViewFields = map[BatchField]struct {
	Getter func(batch *public.BatchViewCompact) string
}{
	"Id": {func(batch *public.BatchViewCompact) string { return batch.Id.String() }},
	"ParentId": {func(batch *public.BatchViewCompact) string {
		if batch.ParentId != nil {
			return batch.ParentId.String()
		}
		return output.EmptyCell
	}},
	"IsSealed":    {func(batch *public.BatchViewCompact) string { return strconv.FormatBool(batch.IsSealed) }},
	"CreatedAt":   {func(batch *public.BatchViewCompact) string { return batch.CreatedAt.Format(output.TimeFormat) }},
	"UpdatedAt":   {func(batch *public.BatchViewCompact) string { return batch.UpdatedAt.Format(output.TimeFormat) }},
	"BlocksCount": {func(batch *public.BatchViewCompact) string { return strconv.Itoa(batch.BlocksCount) }},
}

func AllBatchFields() []BatchField {
	fields := slices.Collect(maps.Keys(BatchViewFields))
	sortBatchFields(fields)
	return fields
}

func sortBatchFields(fields []BatchField) {
	slices.SortFunc(fields, func(l, r BatchField) int {
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
