package main

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteRegression struct {
	tests.RpcSuite

	testAddress types.Address
}

func (s *SuiteRegression) SetupSuite() {
	s.ShardsNum = 4

	var err error
	s.testAddress, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{1})
	s.Require().NoError(err)
}

func (s *SuiteRegression) SetupTest() {
	smartAccountValue, err := types.NewValueFromDecimal("10000000000000000000")
	s.Require().NoError(err)
	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{Name: "MainSmartAccount", Contract: "SmartAccount", Address: types.MainSmartAccountAddress, Value: smartAccountValue, CtorArgs: []any{execution.MainPublicKey}},
			{Name: "Test", Contract: "tests/Test", Address: s.testAddress, Value: types.NewValueFromUint64(100000000000000)},
		},
	}

	s.Start(&nilservice.Config{
		NShards:   s.ShardsNum,
		HttpUrl:   rpc.GetSockPath(s.T()),
		RunMode:   nilservice.CollatorsOnlyRunMode,
		ZeroState: zeroState,
	})
	tests.WaitShardTick(s.T(), s.Context, s.Client, types.MainShardId)
	tests.WaitShardTick(s.T(), s.Context, s.Client, types.BaseShardId)
}

func (s *SuiteRegression) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRegression) TestStaticCall() {
	code, err := contracts.GetCode("tests/StaticCallSource")
	s.Require().NoError(err)
	payload := types.BuildDeployPayload(code, common.EmptyHash)

	addrSource, receipt := s.DeployContractViaMainSmartAccount(types.BaseShardId, payload, types.GasToValue(10_000_000))
	s.Require().True(receipt.AllSuccess())

	code, err = contracts.GetCode("tests/StaticCallQuery")
	s.Require().NoError(err)
	payload = types.BuildDeployPayload(code, common.EmptyHash)

	addrQuery, receipt := s.DeployContractViaMainSmartAccount(types.BaseShardId, payload, types.GasToValue(10_000_000))
	s.Require().True(receipt.AllSuccess())

	abiQuery, err := contracts.GetAbi("tests/StaticCallQuery")
	s.Require().NoError(err)

	data := s.AbiPack(abiQuery, "checkValue", addrSource, types.NewUint256(42))
	receipt = s.SendTransactionViaSmartAccountNoCheck(types.MainSmartAccountAddress, addrQuery, execution.MainPrivateKey, data,
		types.NewFeePackFromGas(200_000), types.NewZeroValue(), nil)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(abiQuery, "querySyncIncrement", addrSource)
	receipt = s.SendTransactionViaSmartAccountNoCheck(types.MainSmartAccountAddress, addrQuery, execution.MainPrivateKey, data,
		types.NewFeePackFromGas(200_000), types.NewZeroValue(), nil)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(abiQuery, "checkValue", addrSource, types.NewUint256(43))
	receipt = s.SendTransactionViaSmartAccountNoCheck(types.MainSmartAccountAddress, addrQuery, execution.MainPrivateKey, data,
		types.NewFeePackFromGas(200_000), types.NewZeroValue(), nil)
	s.Require().True(receipt.AllSuccess())
}

func (s *SuiteRegression) TestEmptyError() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	data := s.AbiPack(abi, "returnEmptyError")
	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress)
	s.Require().False(receipt.Success)
}

func (s *SuiteRegression) TestNonStringError() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	data := s.AbiPack(abi, "returnNonStringError")
	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress)
	s.Require().False(receipt.Success)
	s.Require().Contains(receipt.ErrorMessage, "ExecutionReverted: Not a UTF-8 string: 0x6080")
}

func (s *SuiteRegression) TestErrorOutOfGasStorage() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/GasBurner.sol"), "GasBurner")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode)

	txHash, _, err := s.Client.DeployContract(s.Context, 3, types.MainSmartAccountAddress, deployPayload, types.Value{},
		types.NewFeePackFromGas(10_000), execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().Equal("OutOfGasStorage", receipt.OutReceipts[0].Status)

	outReceipt := receipt.OutReceipts[0]
	block, err := s.Client.GetDebugBlock(s.Context, outReceipt.ShardId, uint64(outReceipt.BlockNumber), true)
	s.Require().NoError(err)
	s.Equal("OutOfGasStorage", block.Errors[outReceipt.TxnHash])
}

func (s *SuiteRegression) TestProposerOutOfGas() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/GasBurner.sol"), "GasBurner")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode)

	addr, receipt := s.DeployContractViaMainSmartAccount(3, deployPayload, types.Value0)
	s.Require().True(receipt.OutReceipts[0].Success)

	calldata, err := abi.Pack("burnGas")
	s.Require().NoError(err)

	txHash, err := s.Client.SendTransactionViaSmartAccount(s.T().Context(),
		types.MainSmartAccountAddress, calldata, types.NewFeePackFromGas(100_000_000_000), types.Value0,
		[]types.TokenBalance{}, addr, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().Equal("OutOfGasDynamic", receipt.OutReceipts[0].Status)
}

func TestRegression(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRegression))
}
