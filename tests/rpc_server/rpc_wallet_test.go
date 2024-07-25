package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteWalletRpc struct {
	RpcSuite
}

func (s *SuiteWalletRpc) SetupSuite() {
	s.start(&nilservice.Config{
		NShards:              4,
		HttpPort:             8533,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            execution.DefaultZeroStateConfig,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteWalletRpc) TearDownSuite() {
	s.cancel()
}

func (s *SuiteWalletRpc) TestWallet() {
	var addrCallee types.Address

	s.Run("Deploy", func() {
		var receipt *jsonrpc.RPCReceipt
		addrCallee, receipt = s.deployContractViaMainWallet(3,
			contracts.CounterDeployPayload(s.T()),
			types.NewValueFromUint64(50_000_000))
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Call", func() {
		receipt := s.sendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey,
			contracts.NewCounterAddCallData(s.T(), 11), types.Value{})
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Check", func() {
		seqno, err := s.client.GetTransactionCount(addrCallee, "latest")
		s.Require().NoError(err)

		resHash, err := s.client.SendMessage(&types.ExternalMessage{
			Data:  contracts.NewCounterGetCallData(s.T()),
			Seqno: seqno,
			To:    addrCallee,
		})
		s.Require().NoError(err)

		receipt := s.waitForReceipt(addrCallee.ShardId(), resHash)
		s.Require().True(receipt.Success)

		newSeqno, err := s.client.GetTransactionCount(addrCallee, "latest")
		s.Require().NoError(err)
		s.Equal(seqno+1, newSeqno)
	})
}

func (s *SuiteWalletRpc) TestDeployWithValueNonPayableConstructor() {
	wallet := types.MainWalletAddress

	hash, addr, err := s.client.DeployContract(2, wallet,
		contracts.CounterDeployPayload(s.T()),
		types.NewValueFromUint64(500_000), execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(wallet.ShardId(), hash)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	balance, err := s.client.GetBalance(addr, "latest")
	s.Require().NoError(err)
	s.EqualValues(0, balance.Uint64())

	code, err := s.client.GetCode(addr, "latest")
	s.Require().NoError(err)
	s.Empty(code)
}

func (s *SuiteWalletRpc) TestDeployWalletWithValue() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pub := crypto.CompressPubkey(&pk.PublicKey)
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
	deployCode := types.BuildDeployPayload(walletCode, common.EmptyHash)

	hash, address, err := s.client.DeployContract(
		types.BaseShardId, types.MainWalletAddress, deployCode, types.NewValueFromUint64(500_000), execution.MainPrivateKey,
	)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(types.MainWalletAddress.ShardId(), hash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	value, err := s.client.GetBalance(address, "latest")
	s.Require().NoError(err)
	s.EqualValues(500_000, value.Uint64())
}

func TestSuiteWalletRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteWalletRpc))
}
