package rpctest

import (
	"context"
	"fmt"
	"testing"

	rpc_client "github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type SuiteWalletRpc struct {
	RpcSuite
	WalletAddress types.Address
}

func (s *SuiteWalletRpc) SetupSuite() {
	s.shardsNum = 4
	s.context, s.cancel = context.WithCancel(context.Background())

	badger, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.port = 8533
	s.client = rpc_client.NewClient(fmt.Sprintf("http://127.0.0.1:%d/", s.port))

	s.WalletAddress = types.HexToAddress("0001111111111111111111111111111111111111") // Put wallet to Base shard instead of Main
	zerostate := fmt.Sprintf(`contracts:
- name: Wallet
  address: %s
  value: 1000000000000
  contract: Wallet
  ctorArgs: [%s]
`, s.WalletAddress.Hex(), hexutil.Encode(execution.MainPublicKey))
	cfg := &nilservice.Config{
		NShards:   s.shardsNum,
		HttpPort:  s.port,
		Topology:  collate.TrivialShardTopologyId,
		ZeroState: zerostate,
	}
	go nilservice.Run(s.context, cfg, badger)
	s.waitZerostate()
}

func (suite *SuiteWalletRpc) TestWallet() {
	// Deploy counter contract via main wallet
	code, err := contracts.GetCode("tests/Counter")
	suite.Require().NoError(err)
	abiCalee, err := contracts.GetAbi("tests/Counter")
	suite.Require().NoError(err)

	addrCallee, receipt := suite.deployContractViaWallet(suite.WalletAddress, execution.MainPrivateKey, types.BaseShardId, code, *types.NewUint256(5_000_000))
	suite.Require().True(receipt.OutReceipts[0].Success)

	var calldata []byte

	// Call `Counter::add` method via main wallet
	calldata, err = abiCalee.Pack("add", int32(11))
	suite.Require().NoError(err)

	receipt = suite.sendMessageViaWallet(suite.WalletAddress, addrCallee, execution.MainPrivateKey, calldata)
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

func TestSuiteWalletRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteWalletRpc))
}
