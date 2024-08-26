package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/profiling"
	"github.com/NilFoundation/nil/nil/internal/readthroughdb"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func main() {
	logger := logging.NewLogger("nild")

	cfg, dbOpts := parseArgs()

	profiling.Start(profiling.DefaultPort)

	database, err := openDb(dbOpts.Path, dbOpts.AllowDrop, logger)
	check.PanicIfErr(err)

	if len(dbOpts.DbAddr) != 0 {
		database, err = readthroughdb.NewReadThroughWithEndpoint(context.Background(), dbOpts.DbAddr, database, transport.BlockNumber(dbOpts.StartBlock))
		check.PanicIfErr(err)
	}

	exitCode := nilservice.Run(context.Background(), cfg, database, nil,
		func(ctx context.Context) error {
			return database.LogGC(ctx, dbOpts.DiscardRatio, dbOpts.GcFrequency)
		})

	database.Close()
	os.Exit(exitCode)
}

func loadConfig() (*nilservice.Config, error) {
	cfg := nilservice.NewDefaultConfig()

	configFileName := flag.String("config", "", "")
	flag.Parse()

	name := *configFileName
	if name == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("can't read config %s: %w", name, err)
	}

	// Parse YAML into our Config struct
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("can't parse config %s: %w", name, err)
	}

	return cfg, nil
}

func parseArgs() (*nilservice.Config, *db.BadgerDBOptions) {
	cfg, err := loadConfig()
	check.PanicIfErr(err)

	dbOpts := db.NewDefaultBadgerDBOptions()

	rootCmd := &cobra.Command{
		Use:           "nild [global flags] [command]",
		Short:         "nild cluster app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	logLevel := rootCmd.PersistentFlags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")
	rootCmd.PersistentFlags().String("config", "", "config file (none by default)")

	rootCmd.PersistentFlags().StringVar(&dbOpts.Path, "db-path", dbOpts.Path, "path to database")
	rootCmd.PersistentFlags().Float64Var(&dbOpts.DiscardRatio, "db-discard-ratio", dbOpts.DiscardRatio, "discard ratio for badger GC")
	rootCmd.PersistentFlags().DurationVar(&dbOpts.GcFrequency, "db-gc-interval", dbOpts.GcFrequency, "frequency for badger GC")
	rootCmd.PersistentFlags().Float64Var(&cfg.GasPriceScale, "gas-price-scale", cfg.GasPriceScale, "gas price scale factor for each transaction")
	rootCmd.PersistentFlags().Uint64Var(&cfg.GasBasePrice, "gas-base-price", cfg.GasBasePrice, "base gas price for each transaction")
	rootCmd.PersistentFlags().IntVar(&cfg.RPCPort, "port", cfg.RPCPort, "http port for rpc server")
	rootCmd.PersistentFlags().StringVar(&cfg.AdminSocketPath, "admin-socket-path", cfg.AdminSocketPath, "unix socket path to start admin server on (disabled if empty)}")
	rootCmd.PersistentFlags().StringVar(&dbOpts.DbAddr, "read-through-db-addr", dbOpts.DbAddr, "address of the read-through database server. If provided, the local node will be run in read-through mode.")
	rootCmd.PersistentFlags().Int64Var(&dbOpts.StartBlock, "read-through-start-block", dbOpts.StartBlock, "mainshard start block number for read-through mode, latest block by default")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run nil application server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.RunMode = nilservice.NormalRunMode
		},
	}
	runCmd.Flags().Uint32Var(&cfg.NShards, "nshards", cfg.NShards, "number of shardchains")
	runCmd.Flags().Var(&cfg.RunOnlyShard, "run-only-shard", "run only specified shard")
	runCmd.Flags().StringToStringVar(&cfg.ShardEndpoints, "shard-endpoints", cfg.ShardEndpoints, "shard endpoints (e.g. 1=localhost:31337,2=localhost:31338)")
	runCmd.Flags().BoolVar(&dbOpts.AllowDrop, "allow-db-clear", dbOpts.AllowDrop, "allow to clear database in case of outdated version")

	// network
	runCmd.Flags().IntVar(&cfg.Network.TcpPort, "tcp-port", cfg.Network.TcpPort, "tcp port for network")
	runCmd.Flags().IntVar(&cfg.Network.QuicPort, "quic-port", cfg.Network.QuicPort, "udp port for network")
	runCmd.Flags().BoolVar(&cfg.Network.UseMdns, "use-mdns", cfg.Network.UseMdns, "use mDNS for discovery (works only in the local network)")
	runCmd.Flags().BoolVar(&cfg.Network.DHTEnabled, "with-discovery", cfg.Network.DHTEnabled, "enable discovery (with Kademlia DHT)")
	runCmd.Flags().StringSliceVar(&cfg.Network.DHTBootstrapPeers, "discovery-bootstrap-peers", cfg.Network.DHTBootstrapPeers, "bootstrap peers for discovery")

	runCmd.Flags().StringVar(&cfg.NetworkKeysPath, "keys-path", cfg.NetworkKeysPath, "path to write keys")

	check.PanicIfErr(runCmd.Flags().SetAnnotation("discovery-bootstrap-peers", cobra.BashCompOneRequiredFlag, []string{"with-discovery"}))

	// telemetry
	runCmd.Flags().BoolVar(&cfg.Telemetry.ExportMetrics, "metrics", cfg.Telemetry.ExportMetrics, "export metrics via grpc")

	replayCmd := &cobra.Command{
		Use:   "replay-block",
		Short: "Start server in single-shard mode to replay particular block",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.RunMode = nilservice.BlockReplayRunMode
		},
	}
	replayCmd.Flags().Var(&cfg.ReplayBlockId, "block-id", "block id to replay")
	replayCmd.Flags().Var(&cfg.ReplayShardId, "shard-id", "shard id to replay block from")

	rootCmd.AddCommand(runCmd, replayCmd)

	f := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		f(c, s)
		os.Exit(0)
	})

	check.PanicIfErr(rootCmd.Execute())

	logging.SetupGlobalLogger(*logLevel)

	return cfg, dbOpts
}

func openDb(dbPath string, allowDrop bool, logger zerolog.Logger) (db.DB, error) {
	dbExists := true
	if _, err := os.Open(dbPath); err != nil {
		if !os.IsNotExist(err) {
			logger.Error().Err(err).Msg("Error opening db path")
			return nil, err
		}
		dbExists = false
	}

	// each shard will interact with DB via this client
	badger, err := db.NewBadgerDb(dbPath)
	if err != nil {
		return nil, err
	}

	tx, err := badger.CreateRwTx(context.Background())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	logger.Info().Msg("Checking scheme format...")
	isVersionOutdated, err := db.IsVersionOutdated(tx)
	if err != nil {
		return nil, err
	}

	if isVersionOutdated {
		if !allowDrop {
			return nil, errors.New("database schema is outdated; remove database or use --allow-db-clear")
		}

		logger.Info().Msg("Clearing database from old data...")
		if err := badger.DropAll(); err != nil {
			return nil, err
		}
	}

	if !dbExists || isVersionOutdated {
		if err := db.WriteVersionInfo(tx, types.NewVersionInfo()); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	return badger, nil
}
