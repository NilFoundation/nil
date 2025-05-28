package output

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
)

const (
	TimeFormat = time.RFC3339
	EmptyCell  = "nil"
)

type StrCell string

func (c StrCell) String() string {
	return string(c)
}

type TableRow []string

func NewTableRow(cells ...fmt.Stringer) TableRow {
	return newTableRowWithConverter(cells, func(cell fmt.Stringer) string {
		if cell == nil {
			return EmptyCell
		}
		if v := reflect.ValueOf(cell); v.Kind() == reflect.Pointer && v.IsNil() {
			return EmptyCell
		}
		return cell.String()
	})
}

func NewTableRowStr(cells ...string) TableRow {
	return newTableRowWithConverter(cells, func(cell string) string { return cell })
}

func newTableRowWithConverter[T any](cells []T, converter func(T) string) TableRow {
	row := make(TableRow, len(cells))
	for i, cell := range cells {
		row[i] = converter(cell)
	}
	return row
}

type Table struct {
	header TableRow
	rows   []TableRow
}

func NewTable(header TableRow, rows []TableRow) (*Table, error) {
	if len(header) == 0 {
		return nil, errors.New("header must not be empty")
	}
	if len(rows) == 0 {
		return nil, errors.New("rows must not be empty")
	}
	if len(header) != len(rows[0]) {
		return nil, errors.New("header and rows must have the same length")
	}

	return &Table{header: header, rows: rows}, nil
}

func (t *Table) AsCmdOutput() exec.CmdOutput {
	var builder Builder

	colWidths := make([]int, len(t.header))
	for colIdx, cell := range t.header {
		colWidths[colIdx] = len(cell)
	}
	for _, row := range t.rows {
		for colIdx, cell := range row {
			if len(cell) > colWidths[colIdx] {
				colWidths[colIdx] = len(cell)
			}
		}
	}

	printRow := func(row []string) {
		builder.WriteString("|")
		for colIdx, cell := range row {
			padding := strings.Repeat(" ", colWidths[colIdx]-len(cell))
			builder.WriteString(" " + cell + padding + " |")
		}
		builder.WriteString("\n")
	}

	printRow(t.header)

	// print header separator
	builder.WriteString("|")
	for _, width := range colWidths {
		builder.WriteString(strings.Repeat("-", width+2))
		builder.WriteString("|")
	}
	builder.WriteString("\n")

	for _, row := range t.rows {
		printRow(row)
	}

	return builder.String()
}
