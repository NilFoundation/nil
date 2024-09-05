package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/cmd/nild/nildconfig"
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

	cfg := parseArgs()

	profiling.Start(profiling.DefaultPort)

	database, err := openDb(cfg.DB.Path, cfg.DB.AllowDrop, logger)
	check.PanicIfErr(err)

	if len(cfg.ReadThrough.SourceAddr) != 0 {
		database, err = readthroughdb.NewReadThroughWithEndpoint(context.Background(), cfg.ReadThrough.SourceAddr, database, cfg.ReadThrough.ForkMainAtBlock)
		check.PanicIfErr(err)
	}

	exitCode := nilservice.Run(context.Background(), cfg.Config, database, nil,
		func(ctx context.Context) error {
			return database.LogGC(ctx, cfg.DB.DiscardRatio, cfg.DB.GcFrequency)
		})

	database.Close()
	os.Exit(exitCode)
}

func loadConfig() (*nildconfig.Config, error) {
	cfg := &nildconfig.Config{
		Config: nilservice.NewDefaultConfig(),
		DB:     db.NewDefaultBadgerDBOptions(),
		ReadThrough: &nildconfig.ReadThroughOptions{
			ForkMainAtBlock: transport.LatestBlockNumber,
		},
	}
	name := ""

	// We need to load config before parsing arguments (it changes global state).
	// Let's search arguments explicitly.
	for i, f := range os.Args[:len(os.Args)-1] {
		if f == "--config" || f == "-c" {
			name = os.Args[i+1]
			break
		}
	}

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

	if cfg.DB == nil {
		cfg.DB = db.NewDefaultBadgerDBOptions()
	}

	return cfg, nil
}

func parseArgs() *nildconfig.Config {
	cfg, err := loadConfig()
	check.PanicIfErr(err)

	rootCmd := &cobra.Command{
		Use:           "nild [global flags] [command]",
		Short:         "nild cluster app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	logLevel := rootCmd.PersistentFlags().StringP("log-level", "l", "info", "log level: trace|debug|info|warn|error|fatal|panic")
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (none by default)")

	rootCmd.PersistentFlags().StringVar(&cfg.DB.Path, "db-path", cfg.DB.Path, "path to database")
	rootCmd.PersistentFlags().Float64Var(&cfg.DB.DiscardRatio, "db-discard-ratio", cfg.DB.DiscardRatio, "discard ratio for badger GC")
	rootCmd.PersistentFlags().DurationVar(&cfg.DB.GcFrequency, "db-gc-interval", cfg.DB.GcFrequency, "frequency for badger GC")
	rootCmd.PersistentFlags().Float64Var(&cfg.GasPriceScale, "gas-price-scale", cfg.GasPriceScale, "gas price scale factor for each transaction")
	rootCmd.PersistentFlags().Uint64Var(&cfg.GasBasePrice, "gas-base-price", cfg.GasBasePrice, "base gas price for each transaction")
	rootCmd.PersistentFlags().IntVar(&cfg.RPCPort, "port", cfg.RPCPort, "http port for rpc server")
	rootCmd.PersistentFlags().StringVar(&cfg.AdminSocketPath, "admin-socket-path", cfg.AdminSocketPath, "unix socket path to start admin server on (disabled if empty)}")
	rootCmd.PersistentFlags().StringVar(&cfg.ReadThrough.SourceAddr, "read-through-db-addr", cfg.ReadThrough.SourceAddr, "address of the read-through database server. If provided, the local node will be run in read-through mode.")
	rootCmd.PersistentFlags().Var(&cfg.ReadThrough.ForkMainAtBlock, "read-through-fork-main-at-block", "all blocks generated later than this MainChain block won't be fetched; latest block by default")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run nil application server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg.RunMode = nilservice.NormalRunMode
		},
	}
	runCmd.Flags().Uint32Var(&cfg.NShards, "nshards", cfg.NShards, "number of shardchains")
	runCmd.Flags().Var(&cfg.MyShard, "my-shard", "run only specified shard")
	runCmd.Flags().BoolVar(&cfg.SplitShards, "split-shards", false, "run each shard in separate process")
	runCmd.Flags().StringToStringVar(&cfg.ShardEndpoints, "shard-endpoints", cfg.ShardEndpoints, "shard endpoints (e.g. 1=localhost:31337,2=localhost:31338)")
	runCmd.Flags().BoolVar(&cfg.DB.AllowDrop, "allow-db-clear", cfg.DB.AllowDrop, "allow to clear database in case of outdated version")

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
	replayCmd.Flags().Var(&cfg.Replay.BlockIdFirst, "first-block", "first block id to replay")
	replayCmd.Flags().Var(&cfg.Replay.BlockIdLast, "last-block", "last block id to replay")
	replayCmd.Flags().Var(&cfg.Replay.ShardId, "shard-id", "shard id to replay block from")

	rootCmd.AddCommand(runCmd, replayCmd)

	f := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		f(c, s)
		os.Exit(0)
	})

	check.PanicIfErr(rootCmd.Execute())

	logging.SetupGlobalLogger(*logLevel)

	if cfg.Replay.BlockIdLast == 0 {
		cfg.Replay.BlockIdLast = cfg.Replay.BlockIdFirst
	}

	return cfg
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
