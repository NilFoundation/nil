package rpctest

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
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

	seqno, err := suite.client.GetTransactionCount(types.MainWalletAddress, "latest")
	suite.Require().NoError(err)

	addrWallet := types.CreateAddress(shardId, code)

	msgDeploy := types.BuildDeployPayload(code, common.EmptyHash)

	msgInternal := &types.Message{
		Seqno:    seqno,
		From:     types.MainWalletAddress,
		To:       addrWallet,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
		Data:     msgDeploy.Bytes(),
		Internal: true,
		Deploy:   true,
	}
	msgInternalData, err := msgInternal.MarshalSSZ()
	suite.Require().NoError(err)

	// Make external message to the Main Wallet
	walletAbi, err := contracts.GetAbi("Wallet")
	suite.Require().NoError(err)
	calldata, err := walletAbi.Pack("send", msgInternalData)
	suite.Require().NoError(err)

	msgExternal := &types.ExternalMessage{
		Seqno: seqno,
		To:    types.MainWalletAddress,
		Data:  calldata,
	}
	suite.Require().NoError(msgExternal.Sign(execution.MainPrivateKey))

	resHash, err := suite.client.SendMessage(msgExternal)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(types.MainWalletAddress.ShardId(), resHash)
	suite.Require().Len(receipt.OutReceipts, 1)
	suite.checkReceipt(receipt.OutReceipts[0], msgInternal.Hash())

	return addrWallet, receipt
}

func (suite *SuiteRpc) sendMessageViaWallet(addrFrom types.Address, messageToSend *types.Message) {
	suite.T().Helper()

	seqno, err := suite.client.GetTransactionCount(addrFrom, "latest")
	suite.Require().NoError(err)

	calldata, err := messageToSend.MarshalSSZ()
	suite.Require().NoError(err)

	// Make external message to the Main Wallet
	walletAbi, err := contracts.GetAbi("Wallet")
	suite.Require().NoError(err)
	calldataExt, err := walletAbi.Pack("send", calldata)
	suite.Require().NoError(err)

	msgExternal := &types.ExternalMessage{
		Seqno: seqno,
		To:    addrFrom,
		Data:  calldataExt,
	}
	suite.Require().NoError(msgExternal.Sign(execution.MainPrivateKey))

	resHash, err := suite.client.SendMessage(msgExternal)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(addrFrom.ShardId(), resHash)
	suite.Require().Len(receipt.OutReceipts, 1)
	suite.checkReceipt(receipt.OutReceipts[0], messageToSend.Hash())
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

	messageToSend := &types.Message{
		Data:     calldata,
		From:     types.MainWalletAddress,
		To:       addrCallee,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
		Internal: true,
	}

	suite.sendMessageViaWallet(types.MainWalletAddress, messageToSend)

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
