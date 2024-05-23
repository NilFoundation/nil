package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/rs/zerolog/log"
	"time"
)

func main() {
	// parse args
	nShards := flag.Int("nshards", 5, "number of shardchains")
	allowDropDb := flag.Bool("allow-db-clear", false, "allow to clear database in case of outdated version")
	dbPath := flag.String("db-path", "test.db", "path to database")
	dbDiscardRation := flag.Float64("db-discard-ratio", 0.5, "discard ratio for badger GC")
	dbGcFrequency := flag.Duration("db-gc-interval", time.Hour, "frequency for badger GC")

	flag.Parse()

	database, err := openDb(*dbPath, *allowDropDb)
	if err != nil {
		log.Error().Err(err).Msg("Error opening db")
		os.Exit(-1)
	}

	os.Exit(nilservice.Run(context.Background(), *nShards, database, *dbDiscardRation, *dbGcFrequency))
}

func openDb(dbPath string, allowDrop bool) (db.DB, error) {
	dbExists := true
	if _, err := os.Open(dbPath); err != nil {
		if !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Error opening db path")
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

	log.Info().Msg("Checking scheme format...")
	isVersionOutdated, err := db.IsVersionOutdated(tx)
	if err != nil {
		return nil, err
	}

	if isVersionOutdated {
		if !allowDrop {
			return nil, errors.New("database schema is outdated; use -allow-db-clear to clear database or clear it manually")
		}

		log.Info().Msg("Clearing database from old data...")
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
