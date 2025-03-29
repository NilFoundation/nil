package main

import (
	"math/big"
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

	smartAccountValue, err := types.NewValueFromDecimal("10000000000000000000")
	s.Require().NoError(err)
	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{
				Name:     "MainSmartAccount",
				Contract: "SmartAccount",
				Address:  types.MainSmartAccountAddress,
				Value:    smartAccountValue,
				CtorArgs: []any{execution.MainPublicKey},
			},
			{
				Name:     "Test",
				Contract: "tests/Test",
				Address:  s.testAddress,
				Value:    types.NewValueFromUint64(100_000_000_000_000),
			},
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

func (s *SuiteRegression) TearDownSuite() {
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
	receipt = s.SendTransactionViaSmartAccountNoCheck(
		types.MainSmartAccountAddress,
		addrQuery,
		execution.MainPrivateKey,
		data,
		types.NewFeePackFromGas(200_000),
		types.NewZeroValue(),
		nil)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(abiQuery, "querySyncIncrement", addrSource)
	receipt = s.SendTransactionViaSmartAccountNoCheck(
		types.MainSmartAccountAddress,
		addrQuery,
		execution.MainPrivateKey,
		data,
		types.NewFeePackFromGas(200_000),
		types.NewZeroValue(),
		nil)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(abiQuery, "checkValue", addrSource, types.NewUint256(43))
	receipt = s.SendTransactionViaSmartAccountNoCheck(
		types.MainSmartAccountAddress,
		addrQuery,
		execution.MainPrivateKey,
		data,
		types.NewFeePackFromGas(200_000),
		types.NewZeroValue(),
		nil)
	s.Require().True(receipt.AllSuccess())
}

func (s *SuiteRegression) TestEmptyError() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	data := s.AbiPack(abi, "returnEmptyError")
	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress)
	s.Require().False(receipt.Success)
}

func (s *SuiteRegression) TestProposerOutOfGas() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	calldata, err := abi.Pack("burnGas")
	s.Require().NoError(err)

	txHash, err := s.Client.SendTransactionViaSmartAccount(
		s.T().Context(),
		types.MainSmartAccountAddress,
		calldata,
		types.NewFeePackFromGas(100_000_000_000),
		types.Value0,
		[]types.TokenBalance{}, s.testAddress, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().Equal("OutOfGasDynamic", receipt.OutReceipts[0].Status)
}

func (s *SuiteRegression) TestInsufficientFundsIncExtSeqno() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	calldata, err := abi.Pack("burnGas")
	s.Require().NoError(err)

	seqno, err := s.Client.GetTransactionCount(s.T().Context(), s.testAddress, "pending")
	s.Require().NoError(err)

	fee := types.NewFeePackFromGas(100_000_000_000_000_000)

	txn := &types.ExternalTransaction{
		Kind:                 types.ExecutionTransactionKind,
		To:                   s.testAddress,
		Data:                 calldata,
		Seqno:                seqno,
		FeeCredit:            fee.FeeCredit,
		MaxFeePerGas:         fee.MaxFeePerGas,
		MaxPriorityFeePerGas: fee.MaxPriorityFeePerGas,
	}

	txHash, err := s.Client.SendTransaction(s.T().Context(), txn)
	s.Require().NoError(err)
	receipt := s.WaitIncludedInMain(txHash)
	s.Require().False(receipt.Success)
	s.Require().Equal("InsufficientFunds", receipt.Status)

	txn.Seqno++
	txHash, err = s.Client.SendTransaction(s.T().Context(), txn)
	s.Require().NoError(err)
	receipt = s.WaitIncludedInMain(txHash)
	s.Require().False(receipt.Success)
	s.Require().Equal("InsufficientFunds", receipt.Status)

	tests.WaitShardTick(s.T(), s.Context, s.Client, types.BaseShardId)

	txn.Seqno++
	txHash, err = s.Client.SendTransaction(s.T().Context(), txn)
	s.Require().NoError(err)
	receipt = s.WaitIncludedInMain(txHash)
	s.Require().False(receipt.Success)
	s.Require().Equal("InsufficientFunds", receipt.Status)
}

func (s *SuiteRegression) TestNonStringError() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	data := []byte{0xC3, 0x28}
	calldata := s.AbiPack(abi, "garbageInRequire", false, string(data))
	receipt := s.SendExternalTransactionNoCheck(calldata, s.testAddress)
	s.Require().False(receipt.Success)
	s.Require().Contains(receipt.ErrorMessage, "ExecutionReverted: Not a UTF-8 string: 0xc328")
}

func (s *SuiteRegression) TestAddressCalculation() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	data := s.GetRandomBytes(65)
	refHash := common.PoseidonHash(data)
	salt := s.GetRandomBytes(32)
	shardId := types.ShardId(2)
	address := types.CreateAddress(shardId, types.BuildDeployPayload(code, common.BytesToHash(salt)))
	address2 := types.CreateAddressForCreate2(address, code, common.BytesToHash(salt))
	codeHash := common.PoseidonHash(code).Bytes()

	// Test `Nil.getPoseidonHash()` library method
	calldata, err := abi.Pack("getPoseidonHash", data)
	s.Require().NoError(err)
	resHash := s.CallGetter(s.testAddress, calldata, "latest", nil)
	s.Require().Equal(refHash[:], resHash)

	// Test `Nil.createAddress()` library method
	calldata, err = abi.Pack("createAddress", big.NewInt(int64(shardId)), []byte(code), big.NewInt(0).SetBytes(salt))
	s.Require().NoError(err)
	resAddress := s.CallGetter(s.testAddress, calldata, "latest", nil)
	s.Require().Equal(address, types.BytesToAddress(resAddress))

	// Test `Nil.createAddress2()` library method
	calldata, err = abi.Pack("createAddress2", big.NewInt(int64(shardId)), address, big.NewInt(0).SetBytes(salt),
		big.NewInt(0).SetBytes(codeHash))
	s.Require().NoError(err)
	resAddress = s.CallGetter(s.testAddress, calldata, "latest", nil)
	s.Require().Equal(address2, types.BytesToAddress(resAddress))
}

// Issue https://github.com/NilFoundation/nil/issues/543
func (s *SuiteRegression) TestNonRevertedErrDecoding() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)

	payload := types.BuildDeployPayload(code, common.EmptyHash)
	contractAddr := types.CreateAddress(1, payload)

	txHash, err := s.Client.SendTransactionViaSmartAccount(
		s.Context,
		types.MainSmartAccountAddress,
		nil,
		types.NewFeePackFromGas(10_000_000),
		types.NewValueFromUint64(1_000_000_000_000_000),
		nil,
		contractAddr,
		execution.MainPrivateKey)
	s.Require().NoError(err)
	receipt := s.WaitForReceipt(txHash)
	s.Require().True(receipt.AllSuccess())

	// Deploy contract with insufficient gas
	txHash, addr, err := s.Client.DeployExternal(s.Context, 1, payload, types.NewFeePackFromGas(50_000))
	s.Require().NoError(err)
	s.Require().Equal(addr, contractAddr)
	receipt = s.WaitForReceipt(txHash)
	s.Require().Equal("OutOfGasStorage", receipt.Status)
	s.Require().False(receipt.Success)

	// Deploy contract with sufficient gas
	txHash, addr, err = s.Client.DeployExternal(s.Context, 1, payload, types.NewFeePackFromGas(5_000_000))
	s.Require().NoError(err)
	s.Require().Equal(addr, contractAddr)
	receipt = s.WaitForReceipt(txHash)
	s.Require().True(receipt.AllSuccess())

	// Check that reverting message is properly propagated
	calldata := s.AbiPack(abi, "mayRevert", true)
	receipt = s.SendExternalTransactionNoCheck(calldata, contractAddr)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted: Revert is true", receipt.ErrorMessage)
}

func TestRegression(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRegression))
}
