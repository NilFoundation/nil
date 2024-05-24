package exporter

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc"
	"github.com/NilFoundation/nil/rpc/httpcfg"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/suite"
)

type SuiteFetchBlock struct {
	suite.Suite
	nShards int32
	context context.Context
	cancel  context.CancelFunc
}

func startRpcServer(tempDir string, ctx context.Context, nShards int32) {
	logger := common.NewLogger("RPC", false)

	dbOpts := db.BadgerDBOptions{Path: tempDir, DiscardRatio: 0.5, GcFrequency: time.Hour, AllowDrop: false}
	database, err := db.NewBadgerDb(dbOpts.Path)
	if err != nil {
		log.Fatal().Msgf("Failed to open db: %s", err.Error())
	}

	httpCfg := httpcfg.HttpCfg{
		Enabled:           true,
		HttpListenAddress: "127.0.0.1",
		HttpPort:          8345,
		HttpCompression:   true,
		TraceRequests:     true,
		HTTPTimeouts:      rpccfg.DefaultHTTPTimeouts,
	}

	baseAPi := jsonrpc.NewBaseApi(0)
	pool := msgpool.New(msgpool.DefaultConfig)
	ethAPI := jsonrpc.NewEthAPI(ctx, baseAPi, database, pool, logger)
	debugAPI := jsonrpc.NewDebugAPI(baseAPi, database, logger)
	apiList := []transport.API{
		{
			Namespace: "eth",
			Public:    true,
			Service:   ethAPI,
			Version:   "1.0",
		},
		{
			Namespace: "debug",
			Public:    true,
			Service:   debugAPI,
			Version:   "1.0",
		},
	}
	go func() {
		if err := concurrent.Run(ctx,
			func(ctx context.Context) error {
				nilservice.Run(ctx, int(nShards), database, dbOpts)
				return nil
			},
			func(ctx context.Context) error {
				return rpc.StartRpcServer(ctx, &httpCfg, apiList, logger)
			},
		); err != nil {
			log.Fatal().Err(err).Msg("RPC server stopped.")
		}
	}()
}

func (suite *SuiteFetchBlock) TestFetchBlock() {
	cfg := Cfg{
		APIEndpoints: []string{"http://127.0.0.1:8345"},
	}

	fetchedBlock, err := cfg.FetchLastBlock(suite.context, types.MasterShardId)
	suite.Require().NoError(err, "Failed to fetch last block")

	suite.Require().NotNil(fetchedBlock, "Fetched block is nil")

	hashBlock, err := cfg.FetchBlockByHash(suite.context, types.MasterShardId, fetchedBlock.Hash())
	suite.Require().NoError(err, "Failed to fetch block by hash")
	suite.Require().NotNil(hashBlock, "Fetched block by hash is nil")

	suite.Require().Equal(fetchedBlock.Id, hashBlock.Id)
	suite.Require().Equal(fetchedBlock.PrevBlock, hashBlock.PrevBlock)
	suite.Require().Equal(fetchedBlock.SmartContractsRoot, hashBlock.SmartContractsRoot)
	suite.Require().Equal(fetchedBlock.MessagesRoot, hashBlock.MessagesRoot)
}

func (suite *SuiteFetchBlock) TestFetchShardIdList() {
	cfg := Cfg{
		APIEndpoints: []string{"http://127.0.0.1:8345"},
	}

	shardIds, err := cfg.FetchShards(suite.context)
	suite.Require().NoError(err, "Failed to fetch shard ids")

	// log the shard ids
	for _, shardId := range shardIds {
		log.Info().Msgf("Shard id: %d", shardId)
	}
	suite.Require().Len(shardIds, int(suite.nShards-1), "Shard ids length is not equal to expected")
}

func TestSuiteFetchBlock(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteFetchBlock))
}

func (suite *SuiteFetchBlock) SetupSuite() {
	suite.context, suite.cancel = context.WithCancel(context.Background())
	suite.nShards = 4
	go startRpcServer(suite.T().TempDir(), suite.context, suite.nShards)
	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteFetchBlock) TearDownSuite() {
	suite.cancel()
}
