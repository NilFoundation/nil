package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type GetTasksParams struct {
	ExecutorParams
	public.TaskDebugRequest
	FieldsToInclude []TaskField
}

func (p *GetTasksParams) Validate() error {
	if err := p.ExecutorParams.Validate(); err != nil {
		return err
	}

	if err := p.TaskDebugRequest.Validate(); err != nil {
		return err
	}

	for _, value := range p.FieldsToInclude {
		if _, ok := TaskViewFields[value]; !ok {
			return fmt.Errorf("unknown task field: %s", value)
		}
	}

	return nil
}

func (p *GetTasksParams) GetExecutorParams() *ExecutorParams {
	return &p.ExecutorParams
}

func GetTasks(ctx context.Context, params *GetTasksParams, api public.TaskDebugApi) (CmdOutput, error) {
	tasks, err := api.GetTasks(ctx, &params.TaskDebugRequest)
	if err != nil {
		return EmptyOutput, fmt.Errorf("failed to get tasks from debug API: %w", err)
	}

	if len(tasks) == 0 {
		return EmptyOutput, fmt.Errorf("%w: no tasks satisfying the request were found", ErrNoDataFound)
	}

	tasksTable := toTasksTable(tasks, params.FieldsToInclude)
	tableOutput := buildTableOutput(tasksTable)
	return tableOutput, nil
}

type table struct {
	header []TaskField
	rows   [][]string
}

func toTasksTable(tasks []*public.TaskView, fieldsToInclude []TaskField) *table {
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		row := toTasksTableRow(task, fieldsToInclude)
		rows = append(rows, row)
	}

	return &table{header: fieldsToInclude, rows: rows}
}

func toTasksTableRow(task *public.TaskView, fieldsToInclude []TaskField) []string {
	row := make([]string, 0, len(fieldsToInclude))

	for _, fieldName := range fieldsToInclude {
		fieldData := TaskViewFields[fieldName]
		strValue := fieldData.Getter(task)
		row = append(row, strValue)
	}

	return row
}

func buildTableOutput(table *table) CmdOutput {
	var builder outputBuilder

	colWidths := make([]int, len(table.header))
	for colIdx, cell := range table.header {
		colWidths[colIdx] = len(cell)
	}
	for _, row := range table.rows {
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

	printRow(table.header)

	// print header separator
	builder.WriteString("|")
	for _, width := range colWidths {
		builder.WriteString(strings.Repeat("-", width+2))
		builder.WriteString("|")
	}
	builder.WriteString("\n")

	for _, row := range table.rows {
		printRow(row)
	}

	return builder.String()
}
