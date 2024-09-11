package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee"
	"github.com/spf13/cobra"
)

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil sync committee node",
	}

	cfg := &synccommittee.Config{
		Telemetry: &telemetry.Config{
			ServiceName: "sync_committee",
		},
	}
	var dbPath string

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the sync committee service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg, dbPath)
		},
	}

	addFlags(runCmd, cfg, &dbPath)

	rootCmd.AddCommand(runCmd)

	return rootCmd.Execute()
}

func addFlags(cmd *cobra.Command, cfg *synccommittee.Config, dbPath *string) {
	cmd.Flags().StringVar(&cfg.RpcEndpoint, "endpoint", "http://127.0.0.1:8529/", "rpc endpoint")
	cmd.Flags().StringVar(&cfg.OwnRpcEndpoint, "own-endpoint", "http://127.0.0.1:8530/", "own rpc server endpoint")
	cmd.Flags().Uint16Var(&cfg.ProversCount, "provers-count", 0, "number of concurrent prover workers")
	cmd.Flags().DurationVar(&cfg.PollingDelay, "polling-delay", 500*time.Millisecond, "delay between new block polling")
	cmd.Flags().StringVar(dbPath, "db-path", "sync_committee.db", "path to database")
	logLevel := cmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	// Telemetry flags
	cmd.Flags().BoolVar(&cfg.Telemetry.ExportMetrics, "metrics", cfg.Telemetry.ExportMetrics, "export metrics via grpc")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func run(cfg *synccommittee.Config, dbPath string) error {
	profiling.Start(profiling.DefaultPort)

	database, err := openDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	service, err := synccommittee.New(cfg, database)
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
