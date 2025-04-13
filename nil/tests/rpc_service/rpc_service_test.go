package tests

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SuiteRpcService struct {
	tests.RpcSuite
}

func (s *SuiteRpcService) SetupSuite() {
	s.Start(&nilservice.Config{
		NShards: 2,
		HttpUrl: rpc.GetSockPath(s.T()),

		// NOTE: caching won't work with parallel tests in this module, because global cache will be shared
		EnableConfigCache: true,
		DisableConsensus:  true,
	})
}

func (s *SuiteRpcService) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteRpcService) TestRpcBasicGetters() {
	var someRandomMissingBlock common.Hash
	s.Require().NoError(someRandomMissingBlock.UnmarshalText(
		[]byte("0x0001117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")))

	shardIdListRes, err := s.Client.GetShardIdList(s.Context)
	s.Require().NoError(err)
	shardIdListExp := make([]types.ShardId, s.ShardsNum-1)
	for i := range shardIdListExp {
		shardIdListExp[i] = types.ShardId(i + 1)
	}
	s.Require().Equal(shardIdListExp, shardIdListRes)

	numShards, err := s.Client.GetNumShards(s.Context)
	s.Require().NoError(err)
	s.Require().Equal(uint64(s.ShardsNum), numShards)

	gasPrice, err := s.Client.GasPrice(s.Context, types.BaseShardId)
	s.Require().NoError(err)
	s.Require().Equal(types.DefaultGasPrice, gasPrice)

	res0Num, err := s.Client.GetBlock(s.Context, types.BaseShardId, 0, false)
	s.Require().NoError(err)
	s.Require().NotNil(res0Num)

	res0Str, err := s.Client.GetBlock(s.Context, types.BaseShardId, "0", false)
	s.Require().NoError(err)
	s.Require().NotNil(res0Num)
	s.Equal(res0Num, res0Str)

	res, err := s.Client.GetBlock(s.Context, types.BaseShardId, transport.BlockNumber(0x1b4), false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	count, err := s.Client.GetBlockTransactionCount(s.Context, types.BaseShardId, transport.EarliestBlockNumber)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	count, err = s.Client.GetBlockTransactionCount(s.Context, types.BaseShardId, someRandomMissingBlock)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	res, err = s.Client.GetBlock(s.Context, types.BaseShardId, someRandomMissingBlock, false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	res, err = s.Client.GetBlock(s.Context, types.BaseShardId, transport.EarliestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	latest, err := s.Client.GetBlock(s.Context, types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	res, err = s.Client.GetBlock(s.Context, types.BaseShardId, latest.Hash, false)
	s.Require().NoError(err)
	s.Require().Equal(latest, res)

	txn, err := s.Client.GetInTransactionByHash(s.Context, someRandomMissingBlock)
	s.Require().NoError(err)
	s.Require().Nil(txn)
}

func (s *SuiteRpcService) TestRpcDebugModules() {
	res, err := s.Client.GetDebugBlock(s.Context, types.BaseShardId, "latest", false)
	s.Require().NoError(err)

	block, err := res.DecodeSSZ()
	s.Require().NoError(err)

	s.Require().NotEmpty(block.Id)
	s.Require().NotEqual(common.EmptyHash, block.Hash(types.BaseShardId))
	s.Require().NotEmpty(res.Content)

	fullRes, err := s.Client.GetDebugBlock(s.Context, types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.Require().NotEmpty(fullRes.Content)
	s.Require().Empty(block.InTransactions)
	s.Require().Empty(block.OutTransactions)
	s.Require().Empty(block.Receipts)
}

func (s *SuiteRpcService) TestRpcApiModules() {
	res, err := s.Client.RawCall(s.Context, "rpc_modules")
	s.Require().NoError(err)

	var data map[string]any
	s.Require().NoError(json.Unmarshal(res, &data))
	s.Equal("1.0", data["eth"])
	s.Equal("1.0", data["rpc"])
}

func (s *SuiteRpcService) TestUnsupportedCliVersion() {
	logger := logging.NewFromZerolog(zerolog.New(os.Stderr))
	s.Run("Unsupported version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint, logger, map[string]string{"User-Agent": "nil-cli/12"})
		_, err := client.ChainId(s.Context)
		s.Require().ErrorContains(err, "unexpected status code: 426: specified revision 12, minimum supported is")
	})

	s.Run("0 means unknown - skip check", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint, logger, map[string]string{"User-Agent": "nil-cli/0"})
		_, err := client.ChainId(s.Context)
		s.Require().NoError(err)
	})

	s.Run("Valid revision", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint,
			logger,
			map[string]string{"User-Agent": "nil-cli/10000000"})
		_, err := client.ChainId(s.Context)
		s.Require().NoError(err)
	})
}

func (s *SuiteRpcService) TestUnsupportedNiljsVersion() {
	logger := logging.NewFromZerolog(zerolog.New(os.Stderr))

	s.Run("Invalid version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint, logger, map[string]string{"Client-Version": "abc"})
		_, err := client.ChainId(s.Context)
		s.Require().ErrorContains(
			err,
			"unexpected status code: 400: invalid Client-Version header: \"abc\". Expected format is niljs/<version>")
	})

	s.Run("Empty version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint, logger, map[string]string{"Client-Version": "niljs"})
		_, err := client.ChainId(s.Context)
		s.Require().ErrorContains(
			err,
			"unexpected status code: 400: invalid Client-Version header: \"niljs\". Expected format is niljs/<version>")
	})

	s.Run("Unsupported version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint,
			logger,
			map[string]string{"Client-Version": "niljs/0.0.1"})
		_, err := client.ChainId(s.Context)
		s.Require().ErrorContains(
			err, "unexpected status code: 426: specified niljs version 0.0.1, minimum supported is")
	})

	s.Run("Valid version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(
			s.Endpoint,
			logger,
			map[string]string{"Client-Version": "niljs/100.0.0"})
		_, err := client.ChainId(s.Context)
		s.Require().NoError(err)
	})
}

func (s *SuiteRpcService) TestRpcError() {
	check := func(code int, txn, method string, params ...any) {
		resp, err := s.Client.RawCall(s.Context, method, params...)
		s.Require().ErrorContains(err, strconv.Itoa(code))
		s.Require().ErrorContains(err, txn)
		s.Require().Nil(resp)
	}

	check(-32601, "the method eth_doesntExist does not exist/is not available",
		"eth_doesntExist")

	check(-32602, "missing value for required argument 0",
		rpc_client.Eth_getBlockByNumber)

	check(-32602, "invalid argument 0: json: cannot unmarshal number 1099511627776 into Go value of type uint32",
		rpc_client.Eth_getBlockByNumber, 1<<40)

	check(-32602, "missing value for required argument 1",
		rpc_client.Eth_getBlockByNumber, types.BaseShardId)

	check(-32602, "invalid argument 0: hex string of odd length",
		rpc_client.Eth_getBlockByHash, "0x1b4", false)

	check(-32602, "invalid argument 0: hex string without 0x prefix",
		rpc_client.Eth_getBlockByHash, "latest")
}

func (s *SuiteRpcService) TestBatch() {
	apis := `{"db":"1.0","debug":"1.0","eth":"1.0","faucet":"1.0","rpc":"1.0","txpool":"1.0","web3":"1.0"}`
	testcases := map[string]string{
		"[]": `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"empty batch"}}`,

		`[{"jsonrpc":"2.0","id": 1, "method":"rpc_modules","params":[]}]`://
		`[{"jsonrpc":"2.0","id":1,"result":` + apis + `}]`,

		`[
			{"jsonrpc":"2.0","id": 1, "method":"rpc_modules","params":[]},
			{"jsonrpc":"2.0","id": 2, "method":"rpc_modules","params":[]}
		]`: //
		`[{"jsonrpc":"2.0","id":1,"result":` + apis + `}, {"jsonrpc":"2.0","id":2,"result":` + apis + `}]`,

		`[{"jsonrpc":"2.0", "method":"rpc_modules","params":[]}]`://
		`[{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request"}}]`,

		`[{"jsonrpc":"2.0", "method":"eth_getBlockByNumber", "params": [0, "100500", false], "id": 1}]`://
		`[{"jsonrpc":"2.0","id":1,"result":null}]`,
	}

	for req, expectedResp := range testcases {
		body, err := s.Client.PlainTextCall(s.Context, []byte(req))
		s.Require().NoError(err)
		s.JSONEq(expectedResp, string(body))
	}

	var err error
	batch := s.Client.CreateBatchRequest()

	_, err = batch.GetBlock(types.MainShardId, "latest", false)
	s.Require().NoError(err)
	_, err = batch.GetDebugBlock(types.BaseShardId, "latest", false)
	s.Require().NoError(err)
	const tooBigNonexistentBlockNumber = "100500"
	_, err = batch.GetBlock(types.MainShardId, tooBigNonexistentBlockNumber, false)
	s.Require().NoError(err)
	_, err = batch.GetDebugBlock(types.BaseShardId, tooBigNonexistentBlockNumber, false)
	s.Require().NoError(err)

	result, err := s.Client.BatchCall(s.Context, batch)
	s.Require().NoError(err)
	s.Require().Len(result, 4)

	b1, ok := result[0].(*jsonrpc.RPCBlock)
	s.Require().True(ok)
	s.Equal(types.MainShardId, b1.ShardId)

	b2, ok := result[1].(*jsonrpc.DebugRPCBlock)
	s.Require().True(ok)
	s.NotEmpty(b2.Content)

	s.Require().Nil(result[2])
	s.Require().Nil(result[3])

	for range 1000 {
		_, err = batch.GetDebugBlock(types.BaseShardId, "latest", false)
		s.Require().NoError(err)
	}
	_, err = s.Client.BatchCall(s.Context, batch)
	s.Require().ErrorContains(err, "batch limit 100 exceeded")
}

func (s *SuiteRpcService) TestClientVersion() {
	res, err := s.Client.ClientVersion(s.Context)
	s.Require().NoError(err)
	s.Require().Contains(res, "=;Nil")
}

func (s *SuiteRpcService) TestTxPoolApi() {
	_, err := s.Client.GetTxpoolStatus(s.Context, types.BaseShardId)
	s.Require().NoError(err)

	_, err = s.Client.GetTxpoolContent(s.Context, types.BaseShardId)
	s.Require().NoError(err)
}

func TestSuiteRpcService(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpcService))
}
