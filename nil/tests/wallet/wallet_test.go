package main

import (
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

type SuiteWalletRpc struct {
	tests.RpcSuite
}

func (s *SuiteWalletRpc) SetupSuite() {
	s.Start(&nilservice.Config{
		NShards:       4,
		HttpUrl:       rpc.GetSockPath(s.T()),
		ZeroStateYaml: execution.DefaultZeroStateConfig,
		RunMode:       nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteWalletRpc) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteWalletRpc) TestWallet() {
	var addrCallee types.Address

	s.Run("Deploy", func() {
		var receipt *jsonrpc.RPCReceipt
		addrCallee, receipt = s.DeployContractViaMainWallet(3,
			contracts.CounterDeployPayload(s.T()),
			types.NewValueFromUint64(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Call", func() {
		receipt := s.SendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), 11))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Check", func() {
		seqno, err := s.Client.GetTransactionCount(addrCallee, "pending")
		s.Require().NoError(err)

		resHash, err := s.Client.SendExternalMessage(
			contracts.NewCounterGetCallData(s.T()),
			addrCallee,
			nil,
			types.Value{},
		)
		s.Require().NoError(err)

		receipt := s.WaitForReceipt(resHash)
		s.Require().True(receipt.Success)

		newSeqno, err := s.Client.GetTransactionCount(addrCallee, "pending")
		s.Require().NoError(err)
		s.Equal(seqno+1, newSeqno)
	})
}

func (s *SuiteWalletRpc) TestDeployWithValueNonPayableConstructor() {
	wallet := types.MainWalletAddress

	hash, addr, err := s.Client.DeployContract(2, wallet,
		contracts.CounterDeployPayload(s.T()),
		types.NewValueFromUint64(500_000), execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	balance, err := s.Client.GetBalance(addr, "latest")
	s.Require().NoError(err)
	s.EqualValues(0, balance.Uint64())

	code, err := s.Client.GetCode(addr, "latest")
	s.Require().NoError(err)
	s.Empty(code)
}

func (s *SuiteWalletRpc) TestDeployWalletWithValue() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pub := crypto.CompressPubkey(&pk.PublicKey)
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
	deployCode := types.BuildDeployPayload(walletCode, common.EmptyHash)

	hash, address, err := s.Client.DeployContract(
		types.BaseShardId, types.MainWalletAddress, deployCode, types.NewValueFromUint64(500_000), execution.MainPrivateKey,
	)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	value, err := s.Client.GetBalance(address, "latest")
	s.Require().NoError(err)
	s.EqualValues(500_000, value.Uint64())
}

func TestSuiteWalletRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteWalletRpc))
}
