package readthroughdb_tests

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/readthroughdb"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteReadThroughDb struct {
	suite.Suite

	server tests.RpcSuite
	cache  tests.RpcSuite
	num    int

	cfg *nilservice.Config
}

func (s *SuiteReadThroughDb) SetupTest() {
	s.server.SetT(s.T())
	s.cache.SetT(s.T())

	s.num = 0

	s.cfg = &nilservice.Config{
		NShards: 5,
		HttpUrl: rpc.GetSockPathIdx(s.T(), s.num),
	}

	s.server.Start(s.cfg)
}

func (s *SuiteReadThroughDb) TearDownTest() {
	s.cache.Cancel()
	s.server.Cancel()
}

func (s *SuiteReadThroughDb) initCache() {
	s.T().Helper()

	s.cache.DbInit = func() db.DB {
		inDb, err := db.NewBadgerDbInMemory()
		check.PanicIfErr(err)
		db, err := readthroughdb.NewReadThroughDbWithMainShard(
			s.cache.Context, s.server.Client, inDb, transport.LatestBlockNumber)
		check.PanicIfErr(err)
		return db
	}

	s.num += 1
	s.cfg.HttpUrl = rpc.GetSockPathIdx(s.T(), s.num)
	s.cache.Start(s.cfg)
}

func (s *SuiteReadThroughDb) waitBlockOnMasterShard(shardId types.ShardId, blockNumber types.BlockNumber) {
	s.T().Helper()

	s.Require().Eventually(func() bool {
		block, err := s.server.Client.GetBlock(s.T().Context(), types.MainShardId, transport.LatestBlockNumber, true)
		s.Require().NoError(err)
		childBlock, err := s.server.Client.GetBlock(s.T().Context(), shardId, block.ChildBlocks[shardId-1], false)
		s.Require().NoError(err)
		return childBlock.Number > blockNumber
	}, tests.ReceiptWaitTimeout, tests.ReceiptPollInterval)
}

func (s *SuiteReadThroughDb) TestBasic() {
	shardId := types.BaseShardId
	var addrCallee types.Address
	var receipt *jsonrpc.RPCReceipt

	s.Run("Deploy", func() {
		addrCallee, receipt = s.server.DeployContractViaMainSmartAccount(shardId,
			contracts.CounterDeployPayload(s.T()),
			types.GasToValue(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	value := int32(5)

	s.Run("Increment", func() {
		receipt = s.server.SendTransactionViaSmartAccount(
			types.MainSmartAccountAddress,
			addrCallee,
			execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), value))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.waitBlockOnMasterShard(shardId, receipt.BlockNumber)
	s.initCache()

	s.Run("GetFromCache", func() {
		data := s.cache.CallGetter(addrCallee, contracts.NewCounterGetCallData(s.T()), "latest", nil)
		s.Require().Equal(value, int32(data[31]))
	})

	s.Run("IncrementCache", func() {
		receipt := s.cache.SendTransactionViaSmartAccount(
			types.MainSmartAccountAddress,
			addrCallee,
			execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), value))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("GetFromServer", func() {
		data := s.server.CallGetter(addrCallee, contracts.NewCounterGetCallData(s.T()), "latest", nil)
		s.Require().Equal(value, int32(data[31]))
	})

	s.Run("GetFromCache2", func() {
		data := s.cache.CallGetter(addrCallee, contracts.NewCounterGetCallData(s.T()), "latest", nil)
		s.Require().Equal(2*value, int32(data[31]))
	})
}

func (s *SuiteReadThroughDb) TestIsolation() {
	shardId := types.BaseShardId
	var addrCallee types.Address
	var receipt *jsonrpc.RPCReceipt

	s.Run("Deploy", func() {
		addrCallee, receipt = s.server.DeployContractViaMainSmartAccount(shardId,
			contracts.CounterDeployPayload(s.T()),
			types.GasToValue(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.waitBlockOnMasterShard(shardId, receipt.BlockNumber)
	s.initCache()

	value := int32(5)
	s.Run("Increment", func() {
		receipt = s.server.SendTransactionViaSmartAccount(
			types.MainSmartAccountAddress,
			addrCallee,
			execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), value))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("ReceiptCache", func() {
		r, err := s.cache.Client.GetInTransactionReceipt(s.T().Context(), receipt.TxnHash)
		s.Require().NoError(err)
		s.Require().Nil(r, "The receipt should not be found in the cache")
	})
}

func TestSuiteReadThroughDb(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteReadThroughDb))
}
