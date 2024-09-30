package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover"
	"github.com/spf13/cobra"
)

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil prover node",
	}

	cfg := &prover.Config{}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the prover service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	addFlags(runCmd, cfg)

	rootCmd.AddCommand(runCmd)

	return rootCmd.Execute()
}

func addFlags(cmd *cobra.Command, cfg *prover.Config) {
	cmd.Flags().StringVar(&cfg.ProofProviderRpcEndpoint, "proof-provider-endpoint", "tcp://127.0.0.1:8531", "proof provider rpc endpoint")
	logLevel := cmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func run(cfg *prover.Config) error {
	profiling.Start(profiling.DefaultPort)

	service, err := prover.New(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create prover service: %w", err)
	}

	err = service.Run(context.Background())
	if err != nil {
		return fmt.Errorf("service exited with error: %w", err)
	}

	return nil
}
