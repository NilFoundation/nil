package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/flags"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/output"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type GetTasksParams struct {
	exec.Params
	public.TaskDebugRequest
	FieldsToInclude []TaskField
}

func (p *GetTasksParams) Validate() error {
	if err := p.Params.Validate(); err != nil {
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

type getTasks struct {
	logger logging.Logger
}

func NewGetTasksCmd(logger logging.Logger) *getTasks {
	return &getTasks{
		logger: logger,
	}
}

func (c *getTasks) Build() (*cobra.Command, error) {
	paramsWithEndpoint := defaultParamsWithEndpoint()

	cmdParams := &GetTasksParams{
		Params:           paramsWithEndpoint.Params,
		TaskDebugRequest: public.DefaultTaskDebugRequest(),
		FieldsToInclude:  DefaultTaskFields(),
	}

	cmd := &cobra.Command{
		Use:   "get-tasks",
		Short: "Get tasks from the node's storage based on provided filter and ordering parameters",
		RunE: func(*cobra.Command, []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, cmdParams)
			client := debug.NewTasksClient(paramsWithEndpoint.RpcEndpoint, c.logger)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.getTasks(ctx, cmdParams, client)
			})
		},
	}

	cmdFlags := cmd.Flags()

	flags.EnumVar(cmdFlags, &cmdParams.Status, "status", "current task status")
	flags.EnumVar(cmdFlags, &cmdParams.Type, "type", "task type")
	cmdFlags.Var(&cmdParams.Owner, "owner", "id of the current task executor")

	flags.EnumVar(cmd.Flags(), &cmdParams.Order, "order", "output tasks sorting order")
	cmdFlags.BoolVar(&cmdParams.Ascending, "ascending", cmdParams.Ascending, "ascending/descending order")

	cmdFlags.Var(
		TaskFieldsFlag{FieldsToInclude: &cmdParams.FieldsToInclude},
		"fields",
		"comma separated list of fields to include in the output table; pass 'all' value to include every field",
	)

	paramsWithEndpoint.bind(cmd)
	bindListRequest(&cmdParams.ListRequest, cmd)
	return cmd, nil
}

func (c *getTasks) getTasks(
	ctx context.Context, params *GetTasksParams, api public.TaskDebugApi,
) (exec.CmdOutput, error) {
	tasks, err := api.GetTasks(ctx, &params.TaskDebugRequest)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to get tasks from debug API: %w", err)
	}

	if len(tasks) == 0 {
		return exec.EmptyOutput, fmt.Errorf("%w: no tasks satisfying the request were found", exec.ErrNoDataFound)
	}

	tasksTable, err := c.toTasksTable(tasks, params.FieldsToInclude)
	if err != nil {
		return exec.EmptyOutput, fmt.Errorf("failed to build tasks table: %w", err)
	}
	tableOutput := tasksTable.AsCmdOutput()
	return tableOutput, nil
}

func (c *getTasks) toTasksTable(tasks []*public.TaskView, fieldsToInclude []TaskField) (*output.Table, error) {
	rows := make([]output.TableRow, 0, len(tasks))
	for _, task := range tasks {
		row := c.toTasksTableRow(task, fieldsToInclude)
		rows = append(rows, row)
	}

	return output.NewTable(fieldsToInclude, rows)
}

func (*getTasks) toTasksTableRow(task *public.TaskView, fieldsToInclude []TaskField) output.TableRow {
	row := make(output.TableRow, 0, len(fieldsToInclude))

	for _, fieldName := range fieldsToInclude {
		fieldData := TaskViewFields[fieldName]
		strValue := fieldData.Getter(task)
		row = append(row, strValue)
	}

	return row
}
