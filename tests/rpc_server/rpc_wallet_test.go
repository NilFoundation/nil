package rpctest

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/tools/solc"
)

// Deploy contract to specific shard
func (suite *SuiteRpc) deployContractViaWallet(shardId types.ShardId, code []byte) types.Address {
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

	msgExternal := &types.Message{
		Seqno:    seqno,
		From:     types.MainWalletAddress,
		To:       types.MainWalletAddress,
		Value:    *types.NewUint256(123456),
		GasLimit: *types.NewUint256(20000000),
		Data:     calldata,
	}
	suite.Require().NoError(msgExternal.Sign(execution.MainPrivateKey))

	resHash, err := suite.client.SendMessage(msgExternal)
	suite.Require().NoError(err)
	suite.Equal(msgExternal.Hash(), resHash)

	receipt := suite.waitForReceipt(msgExternal)
	suite.Require().Len(receipt.OutReceipts, 1)
	suite.checkReceipt(receipt.OutReceipts[0], msgInternal.Hash())

	return addrWallet
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

	msgExternal := &types.Message{
		Seqno:    seqno,
		From:     addrFrom,
		To:       addrFrom,
		Value:    *types.NewUint256(123456),
		GasLimit: *types.NewUint256(20000000),
		Data:     calldataExt,
	}
	suite.Require().NoError(msgExternal.Sign(execution.MainPrivateKey))

	resHash, err := suite.client.SendMessage(msgExternal)
	suite.Require().NoError(err)
	suite.Equal(msgExternal.Hash(), resHash)

	receipt := suite.waitForReceipt(msgExternal)
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
	addrCallee := suite.deployContractViaWallet(1, hexutil.FromHex(smcCallee.Code))

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
	messageToSend = &types.Message{
		Data:     calldata,
		Seqno:    seqno,
		From:     addrCallee,
		To:       addrCallee,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
		Internal: false,
	}
	suite.Require().NoError(messageToSend.Sign(execution.MainPrivateKey))

	resHash, err := suite.client.SendMessage(messageToSend)
	suite.Require().NoError(err)
	suite.Equal(messageToSend.Hash(), resHash)

	suite.waitForReceiptOnShard(addrCallee.ShardId(), messageToSend)
}
