package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/client/rpc"
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
	CommonConfig
	ShardID  types.ShardId
	BlockID  transport.BlockReference
	FileName string
}

type PrintConfig struct {
	FileName string
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

	generateTraceCmd := &cobra.Command{
		Use:   "trace [shard_id] [block_id] [file_name]",
		Short: "Collect traces for a block, dump into file",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			traceConfig := GenerateTraceConfig{CommonConfig: *commonCfg}
			var err error
			traceConfig.ShardID, err = types.ParseShardIdFromString(args[0])
			if err != nil {
				return err
			}
			traceConfig.BlockID, err = transport.AsBlockReference(args[1])
			if err != nil {
				return err
			}
			traceConfig.FileName = args[2]
			return generateTrace(&traceConfig)
		},
	}
	addCommonFlags(generateTraceCmd, commonCfg)
	rootCmd.AddCommand(generateTraceCmd)

	printTraceCmd := &cobra.Command{
		Use:   "print [file_name]",
		Short: "Read serialized traces from file, print them into console",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return readTrace(&PrintConfig{FileName: args[0]})
		},
	}
	addCommonFlags(printTraceCmd, commonCfg)
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
	client := rpc.NewClient(cfg.NilRpcEndpoint, logging.NewLogger("client"))
	remoteTracer, err := tracer.NewRemoteTracer(client, logging.NewLogger("tracer"))
	if err != nil {
		return err
	}

	blockTraces, err := remoteTracer.GetBlockTraces(context.Background(), cfg.ShardID, cfg.BlockID)
	if err != nil {
		return err
	}

	return tracer.SerializeToFile(&blockTraces, cfg.FileName)
}

func readTrace(cfg *PrintConfig) error {
	blockTraces, err := tracer.DeserializeFromFile(cfg.FileName)
	if err != nil {
		return err
	}
	fmt.Printf("%+v", blockTraces)
	return nil
}
