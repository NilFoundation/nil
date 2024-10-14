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
	"github.com/NilFoundation/nil/nil/services/synccommittee/core"
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

	cfg := &core.Config{
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

func addFlags(cmd *cobra.Command, cfg *core.Config, dbPath *string) {
	cmd.Flags().StringVar(&cfg.RpcEndpoint, "endpoint", "https://api.devnet.nil.foundation/api/nil_user/TEK83KSDZH58AIK9PCYSNU4G86DU55I9", "rpc endpoint")
	cmd.Flags().StringVar(&cfg.OwnRpcEndpoint, "own-endpoint", "tcp://127.0.0.1:8530", "own rpc server endpoint")
	cmd.Flags().DurationVar(&cfg.PollingDelay, "polling-delay", 500*time.Millisecond, "delay between new block polling")
	cmd.Flags().StringVar(dbPath, "db-path", "sync_committee.db", "path to database")
	cmd.Flags().StringVar(&cfg.L1Endpoint, "l1-endpoint", "http://rpc2.sepolia.org", "L1 endpoint")
	cmd.Flags().StringVar(&cfg.L1ChainId, "l1-chain-id", "11155111", "L1 chain id")
	cmd.Flags().StringVar(&cfg.PrivateKey, "private-key", "0000000000000000000000000000000000000000000000000000000000000001", "L1 account private key")
	cmd.Flags().StringVar(&cfg.L1ContractAddress, "l1-contract-address", "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4", "L1 update state contract address")
	cmd.Flags().StringVar(&cfg.SelfAddress, "l1-account-address", "0x7A2f4530b5901AD1547AE892Bafe54c5201D1206", "L1 self account address")
	logLevel := cmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	// Telemetry flags
	cmd.Flags().BoolVar(&cfg.Telemetry.ExportMetrics, "metrics", cfg.Telemetry.ExportMetrics, "export metrics via grpc")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func run(cfg *core.Config, dbPath string) error {
	profiling.Start(profiling.DefaultPort)

	database, err := openDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	service, err := core.New(cfg, database)
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
