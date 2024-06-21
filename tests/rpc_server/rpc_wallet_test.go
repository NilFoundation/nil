package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
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
		GracefulShutdown:     false,
	})
}

func (suite *SuiteWalletRpc) TestWallet() {
	// Deploy counter contract via main wallet
	code, err := contracts.GetCode("tests/Counter")
	suite.Require().NoError(err)
	abiCalee, err := contracts.GetAbi("tests/Counter")
	suite.Require().NoError(err)

	addrCallee, receipt := suite.deployContractViaMainWallet(3, code, types.NewUint256(50_000_000))
	suite.Require().True(receipt.OutReceipts[0].Success)

	var calldata []byte

	// Call `Counter::add` method via main wallet
	calldata, err = abiCalee.Pack("add", int32(11))
	suite.Require().NoError(err)

	receipt = suite.sendMessageViaWallet(types.MainWalletAddress, addrCallee, execution.MainPrivateKey, calldata)
	suite.Require().True(receipt.OutReceipts[0].Success)

	// Call get method
	seqno, err := suite.client.GetTransactionCount(addrCallee, "latest")
	suite.Require().NoError(err)
	calldata, err = abiCalee.Pack("get")
	suite.Require().NoError(err)
	messageToSend2 := &types.ExternalMessage{
		Data:  calldata,
		Seqno: seqno,
		To:    addrCallee,
	}

	resHash, err := suite.client.SendMessage(messageToSend2)
	suite.Require().NoError(err)

	receipt = suite.waitForReceipt(addrCallee.ShardId(), resHash)
	suite.Require().True(receipt.Success)

	newSeqno, err := suite.client.GetTransactionCount(addrCallee, "latest")
	suite.Require().NoError(err)
	suite.Equal(seqno+1, newSeqno)
}

func (s *SuiteWalletRpc) TestDeployWithValueNonpayableConstructor() {
	code, err := contracts.GetCode("tests/Counter")
	s.Require().NoError(err)
	abiCalee, err := contracts.GetAbi("tests/Counter")
	s.Require().NoError(err)

	wallet := types.MainWalletAddress
	code = s.prepareDefaultDeployBytecode(*abiCalee, code)

	var shardId types.ShardId = 2
	hash, addr, err := s.client.DeployContract(shardId, wallet, code, types.NewUint256(500_000), execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(wallet.ShardId(), hash)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	balance, err := s.client.GetBalance(addr, "latest")
	s.Require().NoError(err)
	s.EqualValues(0, balance.Uint64())

	code, err = s.client.GetCode(addr, "latest")
	s.Require().NoError(err)
	s.Empty(code)
}

func (s *SuiteRpc) TestDeployWalletWithValue() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pub := crypto.CompressPubkey(&pk.PublicKey)
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
	deployCode := types.BuildDeployPayload(walletCode, common.EmptyHash)

	hash, address, err := s.client.DeployContract(
		types.BaseShardId, types.MainWalletAddress, types.Code(deployCode), types.NewUint256(500_000), execution.MainPrivateKey,
	)
	s.Require().NoError(err)

	receipt := s.waitForReceiptOnShard(types.MainWalletAddress.ShardId(), hash)
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
