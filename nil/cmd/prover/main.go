package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer"
	"github.com/spf13/cobra"
)

func main() {
	check.PanicIfErr(execute())
}

type CommonConfig struct {
	prover.Config
}
type RunConfig struct {
	CommonConfig
}

type GenerateTraceConfig struct {
	*CommonConfig
	ShardID      types.ShardId
	BlockIDs     []transport.BlockReference
	BaseFileName string
	MarshalMode  string
}

type PrintConfig struct {
	BaseFileName string
	MarshalMode  string
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil prover node",
	}

	commonCfg := &CommonConfig{Config: *prover.NewDefaultConfig()}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the prover service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(&RunConfig{CommonConfig: *commonCfg})
		},
	}
	addCommonFlags(runCmd, commonCfg)
	rootCmd.AddCommand(runCmd)

	traceConfig := GenerateTraceConfig{CommonConfig: commonCfg}
	generateTraceCmd := &cobra.Command{
		Use:   "trace [base_file_name] [shard_id] [block_ids...]",
		Short: "Collect traces for a block, dump into file",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			traceConfig.BaseFileName = args[0]
			var err error
			traceConfig.ShardID, err = types.ParseShardIdFromString(args[1])
			if err != nil {
				return err
			}
			traceConfig.BlockIDs = make([]transport.BlockReference, len(args)-2)
			for i, blockArg := range args[2:] {
				traceConfig.BlockIDs[i], err = transport.AsBlockReference(blockArg)
				if err != nil {
					return err
				}
			}
			return generateTrace(&traceConfig)
		},
	}
	addCommonFlags(generateTraceCmd, traceConfig.CommonConfig)
	addMarshalModeFlag(generateTraceCmd, &traceConfig.MarshalMode)
	rootCmd.AddCommand(generateTraceCmd)

	var printConfig PrintConfig
	printTraceCmd := &cobra.Command{
		Use:   "print [file_name]",
		Short: "Read serialized traces from files, print them into console",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			printConfig.BaseFileName = args[0]
			return readTrace(&printConfig)
		},
	}
	addCommonFlags(printTraceCmd, commonCfg)
	addMarshalModeFlag(printTraceCmd, &printConfig.MarshalMode)
	rootCmd.AddCommand(printTraceCmd)

	return rootCmd.Execute()
}

func addCommonFlags(cmd *cobra.Command, cfg *CommonConfig) {
	cmd.Flags().StringVar(&cfg.ProofProviderRpcEndpoint, "proof-provider-endpoint", cfg.ProofProviderRpcEndpoint, "proof provider rpc endpoint")
	cmd.Flags().StringVar(&cfg.NilRpcEndpoint, "nil-endpoint", cfg.NilRpcEndpoint, "nil rpc endpoint")
	logLevel := cmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func addMarshalModeFlag(cmd *cobra.Command, placeholder *string) {
	cmd.Flags().StringVar(placeholder, "marshal-mode", tracer.MarshalModeBinary.String(), "marshal modes (bin,json) for trace files separated by ','")
}

func run(cfg *RunConfig) error {
	profiling.Start(profiling.DefaultPort)

	service, err := prover.New(prover.Config{
		NilRpcEndpoint:           cfg.NilRpcEndpoint,
		ProofProviderRpcEndpoint: cfg.ProofProviderRpcEndpoint,
	})
	if err != nil {
		return fmt.Errorf("failed to create prover service: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	err = service.Run(ctx)
	if err != nil {
		return fmt.Errorf("service exited with error: %w", err)
	}

	return nil
}

func generateTrace(cfg *GenerateTraceConfig) error {
	client := prover.NewRPCClient(cfg.NilRpcEndpoint, logging.NewLogger("client"))
	remoteTracer, err := tracer.NewRemoteTracer(client, logging.NewLogger("tracer"))
	if err != nil {
		return err
	}
	aggTraces := tracer.NewExecutionTraces()
	for _, blockID := range cfg.BlockIDs {
		err := remoteTracer.GetBlockTraces(context.Background(), aggTraces, cfg.ShardID, blockID)
		if err != nil {
			return err
		}
	}

	mode, err := tracer.MarshalModeFromString(cfg.MarshalMode)
	if err != nil {
		return err
	}

	return tracer.SerializeToFile(aggTraces, mode, cfg.BaseFileName)
}

func readTrace(cfg *PrintConfig) error {
	mode, err := tracer.MarshalModeFromString(cfg.MarshalMode)
	if err != nil {
		return err
	}

	blockTraces, err := tracer.DeserializeFromFile(cfg.BaseFileName, mode)
	if err != nil {
		return err
	}
	fmt.Printf("%+v", blockTraces)
	return nil
}
