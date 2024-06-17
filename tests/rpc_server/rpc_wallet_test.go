package rpctest

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/tools/solc"
)

// Deploy contract to specific shard
func (suite *SuiteRpc) deployContractViaWallet(
	shardId types.ShardId, code []byte,
) (types.Address, *jsonrpc.RPCReceipt) {
	suite.T().Helper()

	txHash, addr, err := suite.client.DeployContract(shardId, types.MainWalletAddress, types.Code(code), execution.MainPrivateKey)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(types.MainWalletAddress.ShardId(), txHash)
	suite.Require().Len(receipt.OutReceipts, 1)
	return addr, receipt
}

func (suite *SuiteRpc) sendMessageViaWallet(addrTo types.Address, calldata []byte) {
	suite.T().Helper()

	txHash, err := suite.client.SendMessageViaWallet(types.MainWalletAddress, types.Code(calldata), addrTo, execution.MainPrivateKey)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(types.MainWalletAddress.ShardId(), txHash)
	suite.Require().Len(receipt.OutReceipts, 1)
}

func (suite *SuiteRpc) TestWallet() {
	// Deploy counter contract via main wallet
	code := common.GetAbsolutePath("../../tests/rpc_server/contracts/counter.sol")
	contracts, err := solc.CompileSource(code)
	suite.Require().NoError(err)
	smcCallee := contracts["Counter"]
	suite.Require().NotNil(smcCallee)
	addrCallee, _ := suite.deployContractViaWallet(types.BaseShardId, hexutil.FromHex(smcCallee.Code))

	var calldata []byte

	// Call `Counter::add` method via main wallet
	abiCalee := solc.ExtractABI(smcCallee)
	calldata, err = abiCalee.Pack("add", int32(11))
	suite.Require().NoError(err)

	suite.sendMessageViaWallet(addrCallee, calldata)

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

	suite.waitForReceiptOnShard(addrCallee.ShardId(), resHash)

	newSeqno, err := suite.client.GetTransactionCount(addrCallee, "latest")
	suite.Require().NoError(err)
	suite.Equal(seqno+1, newSeqno)
}
