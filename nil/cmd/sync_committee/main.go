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
	"github.com/NilFoundation/nil/nil/services/synccommittee/core"
	"github.com/ethereum/go-ethereum/ethclient"
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
		&cfg.ProposerParams.Endpoint,
		"l1-endpoint",
		cfg.ProposerParams.Endpoint,
		"L1 endpoint")
	cmd.Flags().StringVar(
		&cfg.ProposerParams.PrivateKey,
		"l1-private-key",
		cfg.ProposerParams.PrivateKey,
		"L1 account private key")
	cmd.Flags().StringVar(
		&cfg.ProposerParams.ContractAddress,
		"l1-contract-address",
		cfg.ProposerParams.ContractAddress,
		"L1 update state contract address")
	cmd.Flags().DurationVar(
		&cfg.ProposerParams.EthClientTimeout,
		"l1-client-timeout",
		cfg.ProposerParams.EthClientTimeout,
		"L1 client timeout")
	cmd.Flags().BoolVar(
		&cfg.ProposerParams.DisableL1,
		"disable-l1",
		cfg.ProposerParams.DisableL1,
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

	ethClient, err := connectToEthClient(cfg.ProposerParams.Endpoint, cfg.ProposerParams.EthClientTimeout)
	if err != nil {
		return err
	}

	service, err := core.New(cfg.Config, database, ethClient)
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

func connectToEthClient(url string, timeout time.Duration) (*ethclient.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ethClient, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connecting to ETH RPC node: %w", err)
	}
	return ethClient, nil
}
