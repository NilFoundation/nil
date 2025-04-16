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
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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

var cfgFile string

func initConfig() {
	if cfgFile == "" {
		return
	}

	viper.SetConfigFile(cfgFile)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("failed to read config file '%s': %v\nonly CLI arguments are going to be applied", cfgFile, err)
	}
}

func main() {
	check.PanicIfErr(execute())
}

func execute() error {
	cobra.OnInitialize(initConfig)

	rootCmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "Run nil L1<->L2 relayer",
	}

	logLevel := rootCmd.PersistentFlags().String("log-level", "info", "app log level")
	rootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		logging.SetupGlobalLogger(*logLevel)
	}

	runCfg := Config{
		RelayerConfig: relayer.DefaultRelayerConfig(),
	}
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run relayer service",
		PreRun: func(cmd *cobra.Command, args []string) {
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				if f.Changed || !viper.IsSet(f.Name) {
					return
				}
				check.PanicIfErr(f.Value.Set(viper.GetString(f.Name)))
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runService(cmd.Context(), &runCfg); err != nil {
				return err
			}
			if len(cfgFile) > 0 {
				if err := viper.SafeWriteConfigAs(cfgFile); err != nil {
					if _, ok := err.(viper.ConfigFileAlreadyExistsError); !ok { //nolint:errorlint
						return err
					}
				}
			}
			return nil
		},
	}
	if err := addRunCommandFlags(runCmd, &runCfg); err != nil {
		return err
	}

	rootCmd.AddCommand(runCmd)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	return rootCmd.ExecuteContext(ctx)
}

func addRunCommandFlags(runCmd *cobra.Command, cfg *Config) error {
	runCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file")

	runCmd.Flags().StringVar(&cfg.DbPath, "db-path", "relayer.db", "path to database")

	runCmd.Flags().StringVar(&cfg.L1ClientConfig.Endpoint,
		"l1-endpoint", "", "URL for ETH L1 client",
	)
	runCmd.Flags().StringVar(
		&cfg.EventListenerConfig.BridgeMessengerContractAddress,
		"l1-contract-addr",
		"",
		"Address of L1BridgeMessenger contract to fetch events from",
	)
	runCmd.Flags().DurationVar(&cfg.L1ClientConfig.Timeout,
		"l1-timeout", time.Second, "Max timeout for ETH client to timeout",
	)
	runCmd.Flags().IntVar(
		&cfg.EventListenerConfig.BatchSize,
		"l1-fetcher-batch-size",
		cfg.EventListenerConfig.BatchSize,
		"Block range len used in event listener to fetch historical data",
	)

	runCmd.Flags().DurationVar(
		&cfg.EventListenerConfig.PollInterval,
		"l1-fetcher-poll-interval",
		cfg.EventListenerConfig.PollInterval,
		"Pause which l1 fetcher takes between fetching historical data batches",
	)

	runCmd.Flags().StringVar(
		&cfg.L2ContractConfig.PrivateKeyPath,
		"l2-private-key-path",
		cfg.L2ContractConfig.PrivateKeyPath,
		"Path to private key file for L2 smart account",
	)

	runCmd.Flags().StringVar(
		&cfg.L2ContractConfig.Endpoint, "l2-endpoint", "", "URL for nil L2 client",
	)
	runCmd.Flags().StringVar(
		&cfg.L2ContractConfig.ContractAddress,
		"l2-contract-addr",
		cfg.L2ContractConfig.ContractAddress,
		"Address of L2BridgeMessenger contract to forward events to",
	)
	runCmd.Flags().StringVar(
		&cfg.L2ContractConfig.SmartAccountAddress,
		"l2-smart-account-addr",
		cfg.L2ContractConfig.SmartAccountAddress,
		"Smart account address for relayer to operate on L2",
	)
	runCmd.Flags().DurationVar(
		&cfg.TransactionSenderConfig.DbPollInterval,
		"l2-transaction-sender-db-poll-interval",
		cfg.TransactionSenderConfig.DbPollInterval,
		"Poll interval for L2 transaction sender",
	)

	// L2 debug mode flags
	runCmd.Flags().BoolVar(&cfg.L2ContractConfig.DebugMode,
		"l2-debug-mode", false, "Enable debug mode for L2 transaction sender",
	)

	runCmd.Flags().StringVar(
		&cfg.L2ContractConfig.SmartAccountSalt,
		"l2-smart-account-salt",
		"",
		"Salt for L2 smart account (debug-only)",
	)

	runCmd.Flags().StringVar(
		&cfg.L2ContractConfig.FaucetAddress,
		"l2-faucet-address",
		"",
		"Faucet address for L2 transaction sender (debug-only)",
	)

	return viper.BindPFlags(runCmd.Flags())
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
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
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

func connectToEthClient(ctx context.Context, url string, timeout time.Duration) (*ethclient.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ethClient, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connecting to ETH RPC node: %w", err)
	}
	return ethClient, nil
}
