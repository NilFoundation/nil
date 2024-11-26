package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core"
	"github.com/spf13/cobra"
)

type cmdConfig struct {
	*core.Config
	DbPath string
}

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil sync committee node",
	}

	cfg := &cmdConfig{
		Config: core.NewDefaultConfig(),
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the sync committee service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	addFlags(runCmd, cfg)

	rootCmd.AddCommand(runCmd)

	return rootCmd.Execute()
}

func addFlags(cmd *cobra.Command, cfg *cmdConfig) {
	cmd.Flags().StringVar(&cfg.RpcEndpoint, "endpoint", cfg.RpcEndpoint, "rpc endpoint")
	cmd.Flags().StringVar(&cfg.TaskListenerRpcEndpoint, "own-endpoint", cfg.TaskListenerRpcEndpoint, "own rpc server endpoint")
	cmd.Flags().DurationVar(&cfg.PollingDelay, "polling-delay", cfg.PollingDelay, "delay between new block polling")
	cmd.Flags().StringVar(&cfg.DbPath, "db-path", "sync_committee.db", "path to database")
	cmd.Flags().StringVar(&cfg.ProposerParams.Endpoint, "l1-endpoint", cfg.ProposerParams.Endpoint, "L1 endpoint")
	cmd.Flags().StringVar(&cfg.ProposerParams.ChainId, "l1-chain-id", cfg.ProposerParams.ChainId, "L1 chain id")
	cmd.Flags().StringVar(&cfg.ProposerParams.PrivateKey, "l1-private-key", cfg.ProposerParams.PrivateKey, "L1 account private key")
	cmd.Flags().StringVar(&cfg.ProposerParams.ContractAddress, "l1-contract-address", cfg.ProposerParams.ContractAddress, "L1 update state contract address")
	cmd.Flags().StringVar(&cfg.ProposerParams.SelfAddress, "l1-account-address", cfg.ProposerParams.SelfAddress, "L1 self account address")
	logLevel := cmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	// Telemetry flags
	cmd.Flags().BoolVar(&cfg.Telemetry.ExportMetrics, "metrics", cfg.Telemetry.ExportMetrics, "export metrics via grpc")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func run(cfg *cmdConfig) error {
	profiling.Start(profiling.DefaultPort)

	database, err := openDB(cfg.DbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	service, err := core.New(cfg.Config, database)
	if err != nil {
		return fmt.Errorf("can't create sync committee service: %w", err)
	}

	err = service.Run(context.Background())
	if err != nil {
		return fmt.Errorf("service exited with error: %w", err)
	}

	return nil
}

func openDB(dbPath string) (db.DB, error) {
	badger, err := db.NewBadgerDb(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create new BadgerDB: %w", err)
	}
	return badger, nil
}
