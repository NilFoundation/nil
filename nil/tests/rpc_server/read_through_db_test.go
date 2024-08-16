package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/readthroughdb"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/suite"
)

type SuiteReadThroughDb struct {
	suite.Suite

	server RpcSuite
	cache  RpcSuite
	num    int

	cfg *nilservice.Config
}

func (s *SuiteReadThroughDb) SetupTest() {
	s.server.SetT(s.T())
	s.cache.SetT(s.T())
	s.num = 0

	s.cfg = &nilservice.Config{
		NShards:              5,
		HttpUrl:              GetSockPathIdx(s.T(), s.num),
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}

	s.server.start(s.cfg)
}

func (s *SuiteReadThroughDb) TearDownTest() {
	s.cache.cancel()
	s.server.cancel()
}

func (s *SuiteReadThroughDb) initCache() {
	s.T().Helper()

	s.cache.dbInit = func() db.DB {
		inDb, err := db.NewBadgerDbInMemory()
		check.PanicIfErr(err)
		db, err := readthroughdb.NewReadThroughDbWithMasterChain(s.cache.context, s.server.client, inDb, transport.LatestBlockNumber)
		check.PanicIfErr(err)
		return db
	}

	s.num += 1
	s.cfg.HttpUrl = GetSockPathIdx(s.T(), s.num)
	s.cache.start(s.cfg)
}

func (s *SuiteReadThroughDb) waitBlockOnMasterShard(shardId types.ShardId, blockNumber types.BlockNumber) {
	s.T().Helper()

	s.Require().Eventually(func() bool {
		block, err := s.server.client.GetBlock(types.MainShardId, transport.LatestBlockNumber, true)
		s.Require().NoError(err)
		childBlock, err := s.server.client.GetBlock(shardId, block.ChildBlocks[shardId-1], false)
		s.Require().NoError(err)
		return childBlock.Number > blockNumber
	}, ReceiptWaitTimeout, ReceiptPollInterval)
}

func (s *SuiteReadThroughDb) TestBasic() {
	shardId := types.BaseShardId
	var addrCallee types.Address
	var receipt *jsonrpc.RPCReceipt

	s.Run("Deploy", func() {
		addrCallee, receipt = s.server.deployContractViaMainWallet(shardId,
			contracts.CounterDeployPayload(s.T()),
			types.NewValueFromUint64(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	value := int32(5)

	s.Run("Increment", func() {
		receipt = s.server.sendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
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
		receipt := s.cache.sendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
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
		addrCallee, receipt = s.server.deployContractViaMainWallet(shardId,
			contracts.CounterDeployPayload(s.T()),
			types.NewValueFromUint64(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.waitBlockOnMasterShard(shardId, receipt.BlockNumber)
	s.initCache()

	value := int32(5)
	s.Run("Increment", func() {
		receipt = s.server.sendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), value))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("ReceiptCache", func() {
		r, err := s.cache.client.GetInMessageReceipt(shardId, receipt.MsgHash)
		s.Require().NoError(err)
		s.Require().Nil(r, "The receipt should not be found in the cache")
	})
}

func TestSuiteReadThroughDb(t *testing.T) {
	t.Parallel()

	// These tests flap on Github. Run them when the issue is resolved.
	// suite.Run(t, new(SuiteReadThroughDb))
}
