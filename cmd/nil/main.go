package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/readthroughdb"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func main() {
	logger := logging.NewLogger("nil")

	cfg, dbOpts := parseArgs()

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

func parseArgs() (*nilservice.Config, *db.BadgerDBOptions) {
	rootCmd := &cobra.Command{
		Use:           "nil [global flags] [command]",
		Short:         "nil cluster app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	runMode := nilservice.NormalRunMode

	dbPath := rootCmd.PersistentFlags().String("db-path", "test.db", "path to database")
	dbDiscardRatio := rootCmd.PersistentFlags().Float64("db-discard-ratio", 0.5, "discard ratio for badger GC")
	dbGcFrequency := rootCmd.PersistentFlags().Duration("db-gc-interval", time.Hour, "frequency for badger GC")
	gasPriceScale := rootCmd.PersistentFlags().Float64("gas-price-scale", 0, "gas price scale factor for each transaction")
	gasBasePrice := rootCmd.PersistentFlags().Uint64("gas-base-price", 10, "base gas price for each transaction")
	logLevel := rootCmd.PersistentFlags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")
	port := rootCmd.PersistentFlags().Int("port", 8529, "http port for rpc server")
	adminSocket := rootCmd.PersistentFlags().String("admin-socket-path", "", "unix socket path to start admin server on (disabled if empty)}")
	dbAddr := rootCmd.PersistentFlags().String("read-through-db-addr", "", "address of the read-through database server. If provided, the local node will be run in read-through mode.")
	startBlock := rootCmd.PersistentFlags().Int64("read-through-start-block", int64(transport.LatestBlockNumber), "mainshard start block number for read-through mode, latest block by default")

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run nil application server",
		Run: func(cmd *cobra.Command, args []string) {
			runMode = nilservice.NormalRunMode
		},
	}
	nShards := runCmd.Flags().Int("nshards", 5, "number of shardchains")
	allowDropDb := runCmd.Flags().Bool("allow-db-clear", false, "allow to clear database in case of outdated version")

	// network
	tcpPort := runCmd.Flags().Int("tcp-port", 0, "tcp port for network")
	quicPort := runCmd.Flags().Int("quic-port", 0, "quic udp port for network")
	useMdns := runCmd.Flags().Bool("use-mdns", false, "use mDNS for discovery (works only in the local network)")

	replayCmd := &cobra.Command{
		Use:   "replay-block",
		Short: "Start server in single-shard mode to replay particular block",
		Run: func(cmd *cobra.Command, args []string) {
			runMode = nilservice.BlockReplayRunMode
		},
	}
	replayBlockId := replayCmd.Flags().Uint64("block-id", 1, "block id to replay")
	replayShardId := replayCmd.Flags().Uint64("shard-id", 1, "shard id to replay block from")

	rootCmd.AddCommand(runCmd, replayCmd)

	f := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		f(c, s)
		os.Exit(0)
	})

	check.PanicIfErr(rootCmd.Execute())

	logging.SetupGlobalLogger(*logLevel)

	dbOpts := &db.BadgerDBOptions{
		Path:         *dbPath,
		DiscardRatio: *dbDiscardRatio,
		GcFrequency:  *dbGcFrequency,
		AllowDrop:    *allowDropDb,
		DbAddr:       *dbAddr,
		StartBlock:   *startBlock,
	}

	cfg := &nilservice.Config{
		NShards:          *nShards,
		HttpPort:         *port,
		Libp2pTcpPort:    *tcpPort,
		Libp2pQuicPort:   *quicPort,
		UseMdns:          *useMdns,
		AdminSocketPath:  *adminSocket,
		Topology:         collate.TrivialShardTopologyId,
		MainKeysOutPath:  "keys.yaml",
		GracefulShutdown: true,
		GasPriceScale:    *gasPriceScale,
		GasBasePrice:     *gasBasePrice,
		RunMode:          runMode,
		ReplayBlockId:    types.BlockNumber(*replayBlockId),
		ReplayShardId:    types.ShardId(*replayShardId),
	}

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
