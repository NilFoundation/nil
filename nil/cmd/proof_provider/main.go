package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/services/synccommittee/proofprovider"
	"github.com/spf13/cobra"
)

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil proof provider node",
	}

	cfg := &proofprovider.Config{}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the proof provider service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	addFlags(runCmd, cfg)

	rootCmd.AddCommand(runCmd)

	return rootCmd.Execute()
}

func addFlags(cmd *cobra.Command, cfg *proofprovider.Config) {
	cmd.Flags().StringVar(&cfg.SyncCommitteeRpcEndpoint, "sync-committee-endpoint", "tcp://127.0.0.1:8530", "sync committee rpc endpoint")
	cmd.Flags().StringVar(&cfg.OwnRpcEndpoint, "own-endpoint", "tcp://127.0.0.1:8531", "own rpc server endpoint")
	cmd.Flags().StringVar(&cfg.DbPath, "db-path", "proof_provider.db", "path to database")
	cmd.Flags().BoolVar(&cfg.Telemetry.ExportMetrics, "metrics", cfg.Telemetry.ExportMetrics, "export metrics via grpc")
	logLevel := cmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func run(cfg *proofprovider.Config) error {
	profiling.Start(profiling.DefaultPort)

	service, err := proofprovider.New(*cfg)
	if err != nil {
		return fmt.Errorf("failed to create proof provider service: %w", err)
	}

	err = service.Run(context.Background())
	if err != nil {
		return fmt.Errorf("service exited with error: %w", err)
	}

	return nil
}
