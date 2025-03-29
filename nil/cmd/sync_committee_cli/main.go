package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/commands"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/flags"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run Sync Committee CLI Tool",
	}

	executorParams := commands.DefaultExecutorParams()

	logging.SetupGlobalLogger("info")
	logger := logging.NewLogger("sync_committee_cli")

	getTasksCmd := buildGetTasksCmd(executorParams, logger)
	rootCmd.AddCommand(getTasksCmd)

	getTaskTreeCmd, err := buildGetTaskTreeCmd(executorParams, logger)
	if err != nil {
		return err
	}
	rootCmd.AddCommand(getTaskTreeCmd)

	decodeBatchCmd := buildDecodeBatchCmd(executorParams, logger)
	rootCmd.AddCommand(decodeBatchCmd)

	return rootCmd.Execute()
}

func buildGetTasksCmd(commonParam *commands.ExecutorParams, logger logging.Logger) *cobra.Command {
	cmdParams := &commands.GetTasksParams{
		ExecutorParams:   *commonParam,
		TaskDebugRequest: public.DefaultTaskDebugRequest(),
		FieldsToInclude:  commands.DefaultFields(),
	}

	cmd := &cobra.Command{
		Use:   "get_tasks",
		Short: "Get tasks from the node's storage based on provided filter and ordering parameters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.NewExecutor(os.Stdout, cmdParams, logger).Run(commands.GetTasks)
		},
	}

	addCommonFlags(cmd, &cmdParams.ExecutorParams)
	cmdFlags := cmd.Flags()

	flags.EnumVar(cmdFlags, &cmdParams.Status, "status", "current task status")
	flags.EnumVar(cmdFlags, &cmdParams.Type, "type", "task type")
	cmdFlags.Var(&cmdParams.Owner, "owner", "id of the current task executor")

	flags.EnumVar(cmd.Flags(), &cmdParams.Order, "order", "output tasks sorting order")
	cmdFlags.BoolVar(&cmdParams.Ascending, "ascending", cmdParams.Ascending, "ascending/descending order")

	cmdFlags.IntVar(
		&cmdParams.Limit,
		"limit",
		cmdParams.Limit,
		fmt.Sprintf(
			"limit the number of tasks returned, should be in range [%d, %d]",
			public.TaskDebugMinLimit, public.TaskDebugMaxLimit,
		),
	)

	cmdFlags.Var(
		flags.TaskFieldsFlag{FieldsToInclude: &cmdParams.FieldsToInclude},
		"fields",
		"comma separated list of fields to include in the output table; pass 'all' value to include every field",
	)

	return cmd
}

func buildGetTaskTreeCmd(commonParam *commands.ExecutorParams, logger logging.Logger) (*cobra.Command, error) {
	cmdParams := &commands.GetTaskTreeParams{
		ExecutorParams: *commonParam,
	}

	cmd := &cobra.Command{
		Use:   "get_task_tree",
		Short: "Retrieve full task tree structure for a specific task",
		RunE: func(cmd *cobra.Command, args []string) error {
			eventLoop := commands.NewExecutor(os.Stdout, cmdParams, logger)
			return eventLoop.Run(commands.GetTaskTree)
		},
	}

	addCommonFlags(cmd, &cmdParams.ExecutorParams)

	const taskIdFlag = "task-id"
	cmd.Flags().Var(&cmdParams.TaskId, taskIdFlag, "root task id")
	if err := cmd.MarkFlagRequired(taskIdFlag); err != nil {
		return nil, err
	}

	return cmd, nil
}

func buildDecodeBatchCmd(_ *commands.ExecutorParams, logger logging.Logger) *cobra.Command {
	params := &commands.DecodeBatchParams{}

	cmd := &cobra.Command{
		Use:   "decode-batch",
		Short: "Deserialize L1 stored batch with nil transactions into human readable format",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.DecodeBatch(context.Background(), params, logger)
		},
	}

	cmd.Flags().Var(&params.BatchId, "batch-id", "unique ID of L1-stored batch")
	cmd.Flags().StringVar(
		&params.BatchFile,
		"batch-file",
		"",
		"file with binary content of concatenated blobs of the batch")
	cmd.Flags().StringVar(&params.OutputFile, "output-file", "", "target file to keep decoded batch data")

	return cmd
}

func addCommonFlags(cmd *cobra.Command, params *commands.ExecutorParams) {
	cmd.Flags().StringVar(&params.DebugRpcEndpoint, "endpoint", params.DebugRpcEndpoint, "debug rpc endpoint")
	cmd.Flags().BoolVar(&params.AutoRefresh, "refresh", params.AutoRefresh, "should the received data be refreshed")
	cmd.Flags().DurationVar(
		&params.RefreshInterval,
		"refresh-interval",
		params.RefreshInterval,
		fmt.Sprintf("refresh interval, min value is %s", commands.MinRefreshInterval),
	)
}
