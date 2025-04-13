package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/cobrax"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/services/synccommittee/proofprovider"
	"github.com/spf13/cobra"
)

const appTitle = "=nil; Proof Provider"

type cmdConfig struct {
	*proofprovider.Config `yaml:",inline"`

	DbPath string `yaml:"dbPath"`
}

func main() {
	check.PanicIfNotCancelledErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil proof provider node",
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the proof provider service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}
	addFlags(runCmd, cfg)

	versionCmd := cobrax.VersionCmd(appTitle)
	rootCmd.AddCommand(runCmd, versionCmd)
	return rootCmd.Execute()
}

func loadConfig() (*cmdConfig, error) {
	cfg := &cmdConfig{
		Config: proofprovider.NewDefaultConfig(),
		DbPath: "proof_provider.db",
	}

	if err := cobrax.LoadConfigFromFile(cobrax.GetConfigNameFromArgs(), cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func addFlags(cmd *cobra.Command, cfg *cmdConfig) {
	cobrax.AddConfigFlag(cmd.Flags())
	cmd.Flags().StringVar(
		&cfg.SyncCommitteeRpcEndpoint,
		"sync-committee-endpoint",
		cfg.SyncCommitteeRpcEndpoint,
		"sync committee rpc endpoint")
	cmd.Flags().StringVar(
		&cfg.TaskListenerRpcEndpoint,
		"own-endpoint",
		cfg.TaskListenerRpcEndpoint,
		"own rpc server endpoint")
	cmd.Flags().StringVar(
		&cfg.DbPath,
		"db-path",
		cfg.DbPath,
		"path to database")
	cmd.Flags().BoolVar(
		&cfg.Telemetry.ExportMetrics,
		"metrics",
		cfg.Telemetry.ExportMetrics,
		"export metrics via grpc")
	cmd.Flags().IntVar(
		&cfg.SkipRate,
		"skip",
		cfg.SkipRate,
		"rate of skip tasks, will skip N from 10, where N is value of option (0 means no skip)."+
			" Possible values: [0,10]")
	cmd.Flags().Uint32Var(
		&cfg.MaxConcurrentBatches,
		"max-concurrent-batches",
		cfg.MaxConcurrentBatches,
		"maximum value of batches that proof provider can handle concurrently",
	)
	logLevel := cmd.Flags().String(
		"log-level",
		"info",
		"log level: trace|debug|info|warn|error|fatal|panic")

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}
}

func run(cfg *cmdConfig) error {
	profiling.Start(profiling.DefaultPort)

	database, err := db.NewBadgerDb(cfg.DbPath)
	if err != nil {
		return fmt.Errorf("failed to create new BadgerDB: %w", err)
	}
	defer database.Close()

	service, err := proofprovider.New(cfg.Config, database)
	if err != nil {
		return fmt.Errorf("failed to create proof provider service: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	err = service.Run(ctx)
	if err != nil {
		return fmt.Errorf("service exited with error: %w", err)
	}

	return nil
}
