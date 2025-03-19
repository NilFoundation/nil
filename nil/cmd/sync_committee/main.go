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
	cmd.Flags().StringVar(
		&cfg.RpcEndpoint,
		"endpoint",
		cfg.RpcEndpoint,
		"rpc endpoint")
	cmd.Flags().StringVar(
		&cfg.TaskListenerRpcEndpoint,
		"own-endpoint",
		cfg.TaskListenerRpcEndpoint,
		"own rpc server endpoint")
	cmd.Flags().DurationVar(
		&cfg.AggregatorConfig.RpcPollingInterval,
		"polling-delay",
		cfg.AggregatorConfig.RpcPollingInterval,
		"delay between new block polling")
	cmd.Flags().StringVar(
		&cfg.DbPath,
		"db-path",
		"sync_committee.db",
		"path to database")
	cmd.Flags().StringVar(
		&cfg.ContractWrapperConfig.Endpoint,
		"l1-endpoint",
		cfg.ContractWrapperConfig.Endpoint,
		"L1 endpoint")
	cmd.Flags().StringVar(
		&cfg.ContractWrapperConfig.PrivateKeyHex,
		"l1-private-key",
		cfg.ContractWrapperConfig.PrivateKeyHex,
		"L1 account private key")
	cmd.Flags().StringVar(
		&cfg.ContractWrapperConfig.ContractAddressHex,
		"l1-contract-address",
		cfg.ContractWrapperConfig.ContractAddressHex,
		"L1 update state contract address")
	cmd.Flags().DurationVar(
		&cfg.ContractWrapperConfig.RequestsTimeout,
		"l1-client-timeout",
		cfg.ContractWrapperConfig.RequestsTimeout,
		"L1 client timeout")
	cmd.Flags().BoolVar(
		&cfg.ContractWrapperConfig.DisableL1,
		"disable-l1",
		cfg.ContractWrapperConfig.DisableL1,
		"Disable send trancations to L1")
	logLevel := cmd.Flags().String(
		"log-level",
		"info",
		"log level: trace|debug|info|warn|error|fatal|panic")

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
