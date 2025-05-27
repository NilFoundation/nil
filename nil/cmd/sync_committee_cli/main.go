package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/commands"
	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/flags"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/cobrax"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

const appTitle = "=nil; Sync Committee CLI"

func main() {
	check.PanicIfNotCancelledErr(execute())
}

type ParamsWithEndpoint struct {
	commands.ExecutorParams
	RpcEndpoint string
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run Sync Committee CLI Tool",
	}

	params := &ParamsWithEndpoint{
		ExecutorParams: commands.ExecutorParamsDefault(),
		RpcEndpoint:    core.DefaultOwnRpcEndpoint,
	}

	logging.SetupGlobalLogger("info")
	logger := logging.NewLogger("sync_committee_cli")

	getTasksCmd := buildGetTasksCmd(params, logger)

	getTaskTreeCmd, err := buildGetTaskTreeCmd(params, logger)
	if err != nil {
		return err
	}
	resetContractCmd, err := buildRollbackContractCmd(logger)
	if err != nil {
		return err
	}

	decodeBatchCmd := buildDecodeBatchCmd(logger)

	versionCmd := cobrax.VersionCmd(appTitle)

	rootCmd.AddCommand(getTasksCmd, getTaskTreeCmd, decodeBatchCmd, resetContractCmd, versionCmd)
	return rootCmd.Execute()
}

func buildGetTasksCmd(params *ParamsWithEndpoint, logger logging.Logger) *cobra.Command {
	cmdParams := &commands.GetTasksParams{
		ExecutorParams:   params.ExecutorParams,
		TaskDebugRequest: public.DefaultTaskDebugRequest(),
		FieldsToInclude:  commands.DefaultFields(),
	}

	cmd := &cobra.Command{
		Use:   "get-tasks",
		Short: "Get tasks from the node's storage based on provided filter and ordering parameters",
		RunE: func(*cobra.Command, []string) error {
			executor := commands.NewExecutor(os.Stdout, logger, cmdParams)
			client := debug.NewTasksClient(params.RpcEndpoint, logger)
			return executor.Run(func(ctx context.Context) (commands.CmdOutput, error) {
				return commands.GetTasks(ctx, cmdParams, client)
			})
		},
	}

	addRpcParamsWithRefresh(cmd, params)
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
			public.DebugMinLimit, public.DebugMaxLimit,
		),
	)

	cmdFlags.Var(
		flags.TaskFieldsFlag{FieldsToInclude: &cmdParams.FieldsToInclude},
		"fields",
		"comma separated list of fields to include in the output table; pass 'all' value to include every field",
	)

	return cmd
}

func buildGetTaskTreeCmd(params *ParamsWithEndpoint, logger logging.Logger) (*cobra.Command, error) {
	cmdParams := &commands.GetTaskTreeParams{
		ExecutorParams: params.ExecutorParams,
	}

	cmd := &cobra.Command{
		Use:   "get-task-tree",
		Short: "Retrieve full task tree structure for a specific task",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := commands.NewExecutor(os.Stdout, logger, cmdParams)
			client := debug.NewTasksClient(params.RpcEndpoint, logger)
			return executor.Run(func(ctx context.Context) (commands.CmdOutput, error) {
				return commands.GetTaskTree(ctx, cmdParams, client)
			})
		},
	}

	addRpcParamsWithRefresh(cmd, params)

	const taskIdFlag = "task-id"
	cmd.Flags().Var(&cmdParams.TaskId, taskIdFlag, "root task id")
	if err := cmd.MarkFlagRequired(taskIdFlag); err != nil {
		return nil, err
	}

	return cmd, nil
}

func buildDecodeBatchCmd(logger logging.Logger) *cobra.Command {
	params := &commands.DecodeBatchParams{}

	cmd := &cobra.Command{
		Use:   "decode-batch",
		Short: "Deserialize L1 stored batch with nil transactions into human readable format",
		RunE: func(*cobra.Command, []string) error {
			executor := commands.NewExecutor(os.Stdout, logger, params)
			return executor.Run(func(ctx context.Context) (commands.CmdOutput, error) {
				return commands.DecodeBatch(ctx, params, logger)
			})
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

func buildRollbackContractCmd(logger logging.Logger) (*cobra.Command, error) {
	params := &commands.RollbackStateParams{}

	cmd := &cobra.Command{
		Use:   "rollback-state",
		Short: "Rollback L1 state to specified root",
		RunE: func(cmd *cobra.Command, args []string) error {
			executor := commands.NewExecutor(os.Stdout, logger, params)
			return executor.Run(func(ctx context.Context) (commands.CmdOutput, error) {
				return commands.RollbackState(ctx, params, logger)
			})
		},
	}

	endpointFlag := "l1-endpoint"
	cmd.Flags().StringVar(
		&params.L1Endpoint,
		endpointFlag,
		params.L1Endpoint,
		"L1 endpoint")
	privateKeyFlag := "l1-private-key"
	cmd.Flags().StringVar(
		&params.PrivateKeyHex,
		privateKeyFlag,
		params.PrivateKeyHex,
		"L1 account private key")
	addressFlag := "l1-contract-address"
	cmd.Flags().StringVar(
		&params.ContractAddressHex,
		addressFlag,
		params.ContractAddressHex,
		"L1 update state contract address")
	targetRootFlag := "target-root"
	cmd.Flags().StringVar(
		&params.TargetStateRootHex,
		"target-root",
		params.TargetStateRootHex,
		"target state root in HEX")

	// make all flags required
	for _, flagId := range []string{endpointFlag, privateKeyFlag, addressFlag, targetRootFlag} {
		if err := cmd.MarkFlagRequired(flagId); err != nil {
			return nil, err
		}
	}

	return cmd, nil
}

func addRpcParamsWithRefresh(cmd *cobra.Command, params *ParamsWithEndpoint) {
	cmd.Flags().StringVar(&params.RpcEndpoint, "endpoint", params.RpcEndpoint, "target rpc endpoint")
	cmd.Flags().BoolVar(&params.AutoRefresh, "refresh", params.AutoRefresh, "should the received data be refreshed")
	cmd.Flags().DurationVar(
		&params.RefreshInterval,
		"refresh-interval",
		params.RefreshInterval,
		fmt.Sprintf("refresh interval, min value is %s", commands.RefreshIntervalMinimal),
	)
}
