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
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func main() {
	logger := logging.NewLogger("nil")

	rootCmd := &cobra.Command{
		Use:   "nil",
		Short: "Run nil cluster",
	}

	nShards := rootCmd.Flags().Int("nshards", 5, "number of shardchains")
	port := rootCmd.Flags().Int("port", 8529, "http port for rpc server")
	allowDropDb := rootCmd.Flags().Bool("allow-db-clear", false, "allow to clear database in case of outdated version")
	dbPath := rootCmd.Flags().String("db-path", "test.db", "path to database")
	dbDiscardRatio := rootCmd.Flags().Float64("db-discard-ratio", 0.5, "discard ratio for badger GC")
	dbGcFrequency := rootCmd.Flags().Duration("db-gc-interval", time.Hour, "frequency for badger GC")
	gasPriceScale := rootCmd.Flags().Float64("gas-price-scale", 0, "gas price scale factor for each transaction")
	gasBasePrice := rootCmd.Flags().Uint64("gas-base-price", 10, "base gas price for each transaction")
	logLevel := rootCmd.Flags().String("log-level", "info", "log level: trace|debug|info|warn|error|fatal|panic")

	check.PanicIfErr(rootCmd.Execute())

	logging.SetupGlobalLogger(*logLevel)

	dbOpts := db.BadgerDBOptions{Path: *dbPath, DiscardRatio: *dbDiscardRatio, GcFrequency: *dbGcFrequency, AllowDrop: *allowDropDb}
	database, err := openDb(dbOpts.Path, dbOpts.AllowDrop, logger)
	check.PanicIfErr(err)

	cfg := &nilservice.Config{
		NShards:          *nShards,
		HttpPort:         *port,
		Topology:         collate.TrivialShardTopologyId,
		MainKeysOutPath:  "keys.yaml",
		GracefulShutdown: true,
		GasPriceScale:    *gasPriceScale,
		GasBasePrice:     *gasBasePrice,
	}
	os.Exit(nilservice.Run(context.Background(), cfg, database,
		func(ctx context.Context) error {
			return database.LogGC(ctx, dbOpts.DiscardRatio, dbOpts.GcFrequency)
		}))
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
