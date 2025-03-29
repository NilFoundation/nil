package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/services/relayer"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jonboulle/clockwork"
	"github.com/spf13/cobra"
)

type EthRpcConfig struct {
	Endpoint string
	Timeout  time.Duration
}

type Config struct {
	DbPath         string
	L1ClientConfig EthRpcConfig
	*relayer.RelayerConfig
}

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil L1<->L2 relayer",
	}

	logLevel := rootCmd.Flags().String("log-level", "info", "app log level")
	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}

	runCfg := Config{
		RelayerConfig: relayer.DefaultRelayerConfig(),
	}
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run relayer service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runService(cmd.Context(), &runCfg)
		},
	}
	addRunCommandFlags(runCmd, &runCfg)

	rootCmd.AddCommand(runCmd)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	return rootCmd.ExecuteContext(ctx)
}

func addRunCommandFlags(runCmd *cobra.Command, cfg *Config) {
	runCmd.Flags().StringVar(&cfg.DbPath, "db-path", "relayer.db", "path to database")
	runCmd.Flags().StringVar(&cfg.L1ClientConfig.Endpoint, "l1-endpoint", "", "URL for ETH L1 client")
	runCmd.Flags().DurationVar(&cfg.L1ClientConfig.Timeout,
		"l1-timeout", time.Second, "Max timeout for ETH client to timeout")

	runCmd.Flags().StringVar(
		&cfg.RelayerConfig.EventListenerConfig.BridgeMessengerContractAddress,
		"l1-contract-addr",
		"",
		"Address of L1BridgeMessenger contract to fetch events from",
	)
	runCmd.Flags().StringVar(
		&cfg.RelayerConfig.EventListenerConfig.BridgeMessengerContractAddress,
		"l2-contract-addr",
		"",
		"Address of L2BridgeMessenger contract to forward events to",
	)

	runCmd.Flags().IntVar(
		&cfg.RelayerConfig.EventListenerConfig.BatchSize,
		"l1-fetcher-batch-size",
		cfg.RelayerConfig.EventListenerConfig.BatchSize,
		"Block range len used in event listener to fetch historical data",
	)

	runCmd.Flags().DurationVar(
		&cfg.RelayerConfig.EventListenerConfig.PollInterval,
		"l1-fetcher-poll-interval",
		cfg.RelayerConfig.EventListenerConfig.PollInterval,
		"Pause which l1 fetcher takes between fetching historical data batches",
	)
}

func runService(ctx context.Context, cfg *Config) error {
	profiling.Start(profiling.DefaultPort)

	database, err := openDB(cfg.DbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	l1Client, err := connectToEthClient(ctx, cfg.L1ClientConfig.Endpoint, cfg.L1ClientConfig.Timeout)
	if err != nil {
		return err
	}

	sysClock := clockwork.NewRealClock()

	svc, err := relayer.New(ctx, database, sysClock, cfg.RelayerConfig, l1Client)
	if err != nil {
		return fmt.Errorf("failed to initialize relayer service: %w", err)
	}

	err = svc.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func openDB(dbPath string) (db.DB, error) {
	badger, err := db.NewBadgerDb(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create new BadgerDB: %w", err)
	}
	return badger, nil
}

func connectToEthClient(ctx context.Context, url string, timeout time.Duration) (*ethclient.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ethClient, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connecting to ETH RPC node: %w", err)
	}
	return ethClient, nil
}
