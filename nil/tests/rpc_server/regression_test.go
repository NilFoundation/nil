package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/stretchr/testify/suite"
)

type SuiteRegression struct {
	RpcSuite
}

func (s *SuiteRegression) SetupSuite() {
	s.shardsNum = 4
}

func (s *SuiteRegression) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              s.shardsNum,
		HttpUrl:              GetSockPath(s.T()),
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteRegression) TearDownTest() {
	s.cancel()
}

func (s *SuiteRegression) TestStaticCall() {
	code, err := contracts.GetCode("tests/StaticCallSource")
	s.Require().NoError(err)
	payload := types.BuildDeployPayload(code, common.EmptyHash)

	addrSource, receipt := s.deployContractViaMainWallet(types.BaseShardId, payload, types.NewValueFromUint64(1_000_000_000))
	s.Require().True(receipt.AllSuccess())

	code, err = contracts.GetCode("tests/StaticCallQuery")
	s.Require().NoError(err)
	payload = types.BuildDeployPayload(code, common.EmptyHash)

	addrQuery, receipt := s.deployContractViaMainWallet(types.BaseShardId, payload, types.NewValueFromUint64(1_000_000_000))
	s.Require().True(receipt.AllSuccess())

	abiQuery, err := contracts.GetAbi("tests/StaticCallQuery")
	s.Require().NoError(err)

	data := s.AbiPack(abiQuery, "checkValue", addrSource, types.NewUint256(42))
	receipt = s.sendMessageViaWalletNoCheck(types.MainWalletAddress, addrQuery, execution.MainPrivateKey, data,
		s.gasToValue(500_000), types.NewZeroValue(), nil)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(abiQuery, "querySyncIncrement", addrSource)
	receipt = s.sendMessageViaWalletNoCheck(types.MainWalletAddress, addrQuery, execution.MainPrivateKey, data,
		s.gasToValue(500_000), types.NewZeroValue(), nil)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(abiQuery, "checkValue", addrSource, types.NewUint256(43))
	receipt = s.sendMessageViaWalletNoCheck(types.MainWalletAddress, addrQuery, execution.MainPrivateKey, data,
		s.gasToValue(500_000), types.NewZeroValue(), nil)
	s.Require().True(receipt.AllSuccess())
}

func TestRegression(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRegression))
}
