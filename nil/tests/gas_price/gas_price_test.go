package tests

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuitGasPrice struct {
	tests.RpcSuite
}

func (s *SuitGasPrice) SetupSuite() {
	s.Start(&nilservice.Config{
		NShards:       4,
		HttpUrl:       rpc.GetSockPath(s.T()),
		ZeroStateYaml: execution.DefaultZeroStateConfig,
		GasPriceScale: 15,
		GasBasePrice:  types.DefaultGasPrice.Uint64(),
		RunMode:       nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuitGasPrice) TearDownSuite() {
	s.Cancel()
}

func (s *SuitGasPrice) TestGasBehaviour() {
	shardId := types.ShardId(3)
	initialGasPrice, err := s.Client.GasPrice(s.Context, shardId)
	s.Require().NoError(err)
	var addrCallee types.Address

	s.Run("Deploy", func() {
		var receipt *jsonrpc.RPCReceipt
		addrCallee, receipt = s.DeployContractViaMainWallet(shardId,
			contracts.CounterDeployPayload(s.T()),
			types.NewValueFromUint64(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("IncreaseGasCost", func() {
		for i := range int32(10) {
			receipt := s.SendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
				contracts.NewCounterAddCallData(s.T(), i))
			s.Require().True(receipt.OutReceipts[0].Success)
		}
		increasedGasPrice, err := s.Client.GasPrice(s.Context, shardId)
		s.Require().NoError(err)
		s.Require().Positive(increasedGasPrice.Cmp(initialGasPrice))
	})

	s.Run("DecreaseGasCost", func() {
		s.Require().Eventually(func() bool {
			gasPrice, err := s.Client.GasPrice(s.Context, shardId)
			s.Require().NoError(err)
			return gasPrice.Cmp(initialGasPrice) == 0
		}, 20*time.Second, time.Second)
	})
}

func TestSuiteGasPrice(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuitGasPrice))
}
