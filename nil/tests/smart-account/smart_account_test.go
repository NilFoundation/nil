package main

import (
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteSmartAccountRpc struct {
	tests.RpcSuite
}

func (s *SuiteSmartAccountRpc) SetupSuite() {
	s.Start(&nilservice.Config{
		NShards: 4,
		HttpUrl: rpc.GetSockPath(s.T()),
		RunMode: nilservice.CollatorsOnlyRunMode,

		DisableConsensus: true,
	})
}

func (s *SuiteSmartAccountRpc) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteSmartAccountRpc) TestSmartAccount() {
	var addrCallee types.Address

	s.Run("Deploy", func() {
		var receipt *jsonrpc.RPCReceipt
		addrCallee, receipt = s.DeployContractViaMainSmartAccount(3,
			contracts.CounterDeployPayload(s.T()),
			types.GasToValue(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Call", func() {
		receipt := s.SendTransactionViaSmartAccount(types.MainSmartAccountAddress, addrCallee, execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), 11))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Check", func() {
		seqno, err := s.Client.GetTransactionCount(s.Context, addrCallee, "pending")
		s.Require().NoError(err)

		resHash, err := s.Client.SendExternalTransaction(
			s.Context,
			contracts.NewCounterGetCallData(s.T()),
			addrCallee,
			nil,
			types.NewFeePackFromGas(500_000),
		)
		s.Require().NoError(err)

		receipt := s.WaitForReceipt(resHash)
		s.Require().True(receipt.Success)

		newSeqno, err := s.Client.GetTransactionCount(s.Context, addrCallee, "pending")
		s.Require().NoError(err)
		s.Equal(seqno+1, newSeqno)
	})
}

func (s *SuiteSmartAccountRpc) TestDeployWithValueNonPayableConstructor() {
	smartAccount := types.MainSmartAccountAddress

	hash, addr, err := s.Client.DeployContract(s.Context, 2, smartAccount,
		contracts.CounterDeployPayload(s.T()),
		types.NewValueFromUint64(500_000), types.NewFeePackFromGas(1_000_000), execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.AllSuccess())
	//s.Require().False(receipt.OutReceipts[0].Success)

	balance, err := s.Client.GetBalance(s.Context, addr, "latest")
	s.Require().NoError(err)
	s.Zero(balance.Uint64())

	code, err := s.Client.GetCode(s.Context, addr, "latest")
	s.Require().NoError(err)
	s.Empty(code)
}

func (s *SuiteSmartAccountRpc) TestDeploySmartAccountWithValue() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pub := crypto.CompressPubkey(&pk.PublicKey)
	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(pub)
	deployCode := types.BuildDeployPayload(smartAccountCode, common.EmptyHash)

	hash, address, err := s.Client.DeployContract(
		s.Context, types.BaseShardId, types.MainSmartAccountAddress, deployCode, types.NewValueFromUint64(500_000),
		types.NewFeePackFromGas(10_000_000), execution.MainPrivateKey,
	)
	s.Require().NoError(err)
	fmt.Println("CCCCCCCCCCCCCCCCCCC 1:", address.Hex())


	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	value, err := s.Client.GetBalance(s.Context, address, "latest")
	s.Require().NoError(err)
	s.EqualValues(500_000, value.Uint64())
}

func TestSuiteSmartAccountRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteSmartAccountRpc))
}
