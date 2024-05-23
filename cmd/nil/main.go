package main

import (
	"context"
	"flag"
	"os"
	"syscall"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/rs/zerolog/log"
)

func startRpcServer(ctx context.Context, db db.DB, pool msgpool.Pool) error {
	logger := common.NewLogger("RPC", false)

	httpConfig := &httpcfg.HttpCfg{
		Enabled:           true,
		HttpURL:           "tcp://127.0.0.1:8529",
		HttpListenAddress: "127.0.0.1",
		HttpPort:          8529,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}

	base := jsonrpc.NewBaseApi(rpccfg.DefaultEvmCallTimeout)

	ethImpl := jsonrpc.NewEthAPI(ctx, base, db, pool, logger)
	debugImpl := jsonrpc.NewDebugAPI(base, db, logger)

	apiList := []transport.API{
		{
			Namespace: "eth",
			Public:    true,
			Service:   jsonrpc.EthAPI(ethImpl),
			Version:   "1.0",
		},
		{
			Namespace: "debug",
			Public:    true,
			Service:   jsonrpc.DebugAPI(debugImpl),
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}

func run() int {
	common.SetupGlobalLogger()

	// parse args
	nShards := flag.Int("nshards", 5, "number of shardchains")
	allowDropDb := flag.Bool("allow-db-clear", false, "allow to clear database in case of outdated version")
	dbPath := flag.String("db-path", "test.db", "path to database")
	dbExist := false
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := os.Open(*dbPath); !os.IsNotExist(err) {
		dbExist = true
	}
	// each shard will interact with DB via this client
	badger, err := db.NewBadgerDb(*dbPath)
	if err != nil {
		log.Error().Err(err).Msg("Error opening badger db")
		return -1
	}
	tx, err := badger.CreateRwTx(ctx)
	if err != nil {
		log.Error().Msg(err.Error())
		return -1
	}
	defer tx.Rollback()
	log.Info().Msg("Checking scheme format...")
	isVersionOutdated, err := db.IsVersionOutdated(tx)
	if err != nil {
		log.Error().Msg(err.Error())
		return -1
	}
	if isVersionOutdated {
		if !*allowDropDb {
			log.Error().Msg("Database schema is outdated. Use -allow-db-clear to clear database or clear it manually")
			return -1
		}
		log.Info().Msg("Clear database from old data...")
		if err := badger.DropAll(); err != nil {
			log.Error().Msg(err.Error())
			return -1
		}
	}
	if !dbExist || isVersionOutdated {
		if err := db.WriteVersionInfo(tx, types.NewVersionInfo()); err != nil {
			log.Error().Msg(err.Error())
			return -1
		}
		if err := tx.Commit(); err != nil {
			log.Error().Msg(err.Error())
			return -1
		}
	}

	msgPool := msgpool.New(msgpool.DefaultConfig)
	if msgPool == nil {
		log.Error().Msg("Failed to create message pool")
		return -1
	}

	log.Info().Msg("Starting services...")

	if err := concurrent.Run(ctx,
		func(ctx context.Context) error {
			concurrent.OnSignal(ctx, cancel, syscall.SIGTERM, syscall.SIGINT)
			return nil
		},
		func(ctx context.Context) error {
			shards := make([]*shardchain.ShardChain, *nShards)
			for i := range *nShards {
				shards[i] = shardchain.NewShardChain(types.ShardId(i), badger, *nShards)
			}

			collator := collate.NewCollator(shards)
			return collator.Run(ctx)
		},
		func(ctx context.Context) error {
			return startRpcServer(ctx, badger, msgPool)
		},
	); err != nil {
		log.Error().Err(err).Msg("App encountered an error and will be terminated.")
		return 1
	}

	log.Warn().Msg("App is terminated.")
	return 0
}

func main() {
	os.Exit(run())
}
