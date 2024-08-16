package rpctest

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/stretchr/testify/suite"
)

type SuitGasPrice struct {
	RpcSuite
}

func (s *SuitGasPrice) SetupSuite() {
	s.start(&nilservice.Config{
		NShards:              4,
		HttpUrl:              "tcp://127.0.0.1:8535",
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            execution.DefaultZeroStateConfig,
		CollatorTickPeriodMs: 100,
		GasPriceScale:        15,
		GasBasePrice:         10,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuitGasPrice) TearDownSuite() {
	s.cancel()
}

func (s *SuitGasPrice) TestGasBehaviour() {
	shardId := types.ShardId(3)
	initialGasPrice, err := s.client.GasPrice(shardId)
	s.Require().NoError(err)
	var addrCallee types.Address

	s.Run("Deploy", func() {
		var receipt *jsonrpc.RPCReceipt
		addrCallee, receipt = s.deployContractViaMainWallet(shardId,
			contracts.CounterDeployPayload(s.T()),
			types.NewValueFromUint64(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("IncreaseGasCost", func() {
		for i := range 10 {
			receipt := s.sendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
				contracts.NewCounterAddCallData(s.T(), int32(i)))
			s.Require().True(receipt.OutReceipts[0].Success)
		}
		increasedGasPrice, err := s.client.GasPrice(shardId)
		s.Require().NoError(err)
		s.Require().Positive(increasedGasPrice.Cmp(initialGasPrice))
	})

	s.Run("DecreaseGasCost", func() {
		s.Require().Eventually(func() bool {
			gasPrice, err := s.client.GasPrice(shardId)
			s.Require().NoError(err)
			return gasPrice.Cmp(initialGasPrice) == 0
		}, 20*time.Second, time.Second)
	})
}

func TestSuiteGasPrice(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuitGasPrice))
}
