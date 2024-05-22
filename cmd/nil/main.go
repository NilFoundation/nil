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

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// each shard will interact with DB via this client
	badger, err := db.NewBadgerDb("test.db")
	if err != nil {
		log.Error().Err(err).Msg("Error opening badger db")
		return -1
	}
	tx, err := badger.CreateRwTx(ctx)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	defer tx.Rollback()
	goodVersion, err := db.CompareVersion(tx)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	if !goodVersion {
		log.Info().Msg("Clear database with old schema")
		badger.DropAll()
		err := db.WriteDbVersion(db.ReadCurrentVersion(), tx)
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
		err = tx.Commit()
		if err != nil {
			log.Fatal().Msg(err.Error())
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
