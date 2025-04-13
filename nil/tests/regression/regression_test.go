package main

import (
	"math/big"
	"testing"
	"time"

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
	tests.WaitShardTick(s.T(), s.Client, types.MainShardId)
	tests.WaitShardTick(s.T(), s.Client, types.BaseShardId)
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
	s.Require().Equal("TransactionExceedsBlockGasLimit", receipt.OutReceipts[0].Status)
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

	tests.WaitShardTick(s.T(), s.Client, types.BaseShardId)

	txn.Seqno++
	txHash, err = s.Client.SendTransaction(s.T().Context(), txn)
	s.Require().NoError(err)
	receipt = s.WaitIncludedInMain(txHash)
	s.Require().False(receipt.Success)
	s.Require().Equal("InsufficientFunds", receipt.Status)
}

func (s *SuiteRegression) TestInsufficientFundsDeploy() {
	salt := common.HexToHash("0x02")
	deployPayload := contracts.GetDeployPayloadWithSalt(s.T(), contracts.NameTest, salt)
	addr, err := contracts.CalculateAddress(contracts.NameTest, 1, salt.Bytes())
	s.Require().NoError(err)

	txHash, err := s.Client.SendTransactionViaSmartAccount(
		s.T().Context(),
		types.MainSmartAccountAddress,
		nil,
		types.NewFeePackFromGas(100_000),
		types.Value10,
		[]types.TokenBalance{}, addr, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)

	fee := types.NewFeePackFromGas(100_000_000_000_000_000)
	txn := &types.ExternalTransaction{
		Kind:                 types.DeployTransactionKind,
		To:                   addr,
		Data:                 deployPayload.Bytes(),
		Seqno:                0,
		FeeCredit:            fee.FeeCredit,
		MaxFeePerGas:         fee.MaxFeePerGas,
		MaxPriorityFeePerGas: fee.MaxPriorityFeePerGas,
	}

	txHash, err = s.Client.SendTransaction(s.T().Context(), txn)
	s.Require().NoError(err)
	receipt = s.WaitIncludedInMain(txHash)
	s.Require().False(receipt.Success)
	s.Require().True(receipt.Temporary)
	s.Require().Equal("InsufficientFunds", receipt.Status)

	contract := tests.GetContract(s.T(), s.Client, addr)
	s.Zero(contract.ExtSeqno)
}

func (s *SuiteRegression) TestUnsuccessfulDeployWithGasUsed() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/Unconstructable.sol"), "Unconstructable")
	deployPayload := s.PrepareDefaultDeployPayload(abi, common.EmptyHash, contractCode)
	addr := types.CreateAddress(types.BaseShardId, deployPayload)

	txHash, err := s.Client.SendTransactionViaSmartAccount(
		s.T().Context(),
		types.MainSmartAccountAddress,
		nil,
		types.NewFeePackFromGas(100_000_000_000),
		types.NewValueFromUint64(50_000_000_000),
		[]types.TokenBalance{}, addr, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)

	balance, err := s.Client.GetBalance(s.T().Context(), addr, "latest")
	s.Require().NoError(err)
	s.Require().EqualValues(50_000_000_000, balance.Uint64())

	fee := types.NewFeePackFromGas(1_000)
	txn := &types.ExternalTransaction{
		Kind:                 types.DeployTransactionKind,
		To:                   addr,
		Data:                 deployPayload.Bytes(),
		Seqno:                0,
		FeeCredit:            fee.FeeCredit,
		MaxFeePerGas:         fee.MaxFeePerGas,
		MaxPriorityFeePerGas: fee.MaxPriorityFeePerGas,
	}

	txHash, err = s.Client.SendTransaction(s.T().Context(), txn)
	s.Require().NoError(err)
	receipt = s.WaitIncludedInMain(txHash)
	s.Require().False(receipt.Success)
	s.Require().False(receipt.Temporary)
	s.Require().NotZero(receipt.GasUsed)
	s.Require().Equal("OutOfGasDynamic", receipt.Status)

	contract := tests.GetContract(s.T(), s.Client, addr)
	s.NotZero(contract.ExtSeqno)
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

	data := tests.GetRandomBytes(s.T(), 65)
	refHash := common.PoseidonHash(data)
	salt := tests.GetRandomBytes(s.T(), 32)
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

	payload := types.BuildDeployPayload(code, common.Hash{0x03})
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

func (s *SuiteRegression) TestBigTransactions() {
	abi, err := contracts.GetAbi(contracts.NameStresser)
	s.Require().NoError(err)

	stresserCode, err := contracts.GetCode(contracts.NameStresser)
	s.Require().NoError(err)

	addr, receipt := s.DeployContractViaMainSmartAccount(3,
		types.BuildDeployPayload(stresserCode, common.EmptyHash), types.GasToValue(1_000_000_000))

	n := (types.DefaultMaxGasInBlock + 10000) / 529
	calldata := s.AbiPack(abi, "gasConsumer", big.NewInt(int64(n)))

	s.Run("Internal big transaction", func() {
		txHash, err := s.Client.SendTransactionViaSmartAccount(
			s.Context,
			types.MainSmartAccountAddress,
			calldata,
			types.NewFeePackFromGas(50_000_000),
			types.Value0,
			nil,
			addr,
			execution.MainPrivateKey,
		)
		s.Require().NoError(err)

		// Use longer timeout because it can fail in CI tests
		s.Require().Eventually(func() bool {
			receipt, err = s.Client.GetInTransactionReceipt(s.Context, txHash)
			s.Require().NoError(err)
			return receipt.IsComplete()
		}, 30*time.Second, 1000*time.Millisecond)

		s.Require().NoError(err)

		s.Require().True(receipt.Success)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().False(receipt.OutReceipts[0].Success)
		s.Require().Equal("TransactionExceedsBlockGasLimit", receipt.OutReceipts[0].Status)
	})

	s.Run("External big transaction", func() {
		txHash, err := s.Client.SendExternalTransaction(s.Context, calldata, addr, nil,
			types.NewFeePackFromGas(50_000_000))
		s.Require().NoError(err)

		// Use longer timeout because it can fail in CI tests
		s.Require().Eventually(func() bool {
			receipt, err = s.Client.GetInTransactionReceipt(s.Context, txHash)
			s.Require().NoError(err)
			return receipt.IsComplete()
		}, 30*time.Second, 1000*time.Millisecond)

		s.Require().False(receipt.Success)
		s.Require().Equal("TransactionExceedsBlockGasLimit", receipt.Status)
		txpool, err := s.Client.GetTxpoolStatus(s.Context, addr.ShardId())
		s.Require().NoError(err)
		s.Require().Zero(txpool.Pending)
		s.Require().Zero(txpool.Queued)
	})

	s.Run("Big deploy transaction", func() {
		payload, err := contracts.CreateDeployPayload("tests/HeavyConstructor", nil,
			big.NewInt(int64((types.DefaultMaxGasInBlock+10000)/529)))
		s.Require().NoError(err)

		txHash, _, err := s.Client.DeployContract(
			s.Context,
			types.BaseShardId,
			types.MainSmartAccountAddress,
			payload,
			types.Value0,
			types.NewFeePackFromGas(50_000_000),
			execution.MainPrivateKey)
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(txHash)
		s.Require().True(receipt.Success)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().False(receipt.OutReceipts[0].Success)
		s.Require().Equal("TransactionExceedsBlockGasLimit", receipt.OutReceipts[0].Status)
	})
}

func (s *SuiteRegression) TestDeployFromContract() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	calldata, err := abi.Pack("deployContract")
	s.Require().NoError(err)

	receipt := s.SendExternalTransactionNoCheck(calldata, s.testAddress)
	s.Require().True(receipt.Success)

	contractAddr1, err := abi.Unpack("newContract", receipt.Logs[0].Data)
	s.Require().NoError(err)

	receipt = s.SendExternalTransactionNoCheck(calldata, s.testAddress)
	s.Require().True(receipt.Success)

	contractAddr2, err := abi.Unpack("newContract", receipt.Logs[0].Data)
	s.Require().NoError(err)
	s.NotEqual(contractAddr1, contractAddr2)
}

func TestRegression(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRegression))
}
