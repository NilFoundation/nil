package rpctest

import (
	"encoding/hex"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/tools/solc"
)

// TODO: This is too boilerplate code, need to simplify it

// Deploy contract to specific shard
func (suite *SuiteRpc) deployContractViaWallet(shardId types.ShardId, code []byte) types.Address {
	seqno := suite.getTransactionCount(types.MainWalletAddress, "latest")

	addrWallet := types.CreateAddress(shardId, code)

	msgDeploy := &types.DeployMessage{
		ShardId: shardId,
		Code:    code,
	}
	msgDeployData, err := msgDeploy.MarshalSSZ()
	suite.Require().NoError(err)

	msgInternal := &types.Message{
		Seqno:    seqno,
		From:     types.MainWalletAddress,
		To:       addrWallet,
		Value:    *types.NewUint256(0),
		GasLimit: *types.NewUint256(100000),
		Data:     msgDeployData,
		Internal: true,
		Deploy:   true,
	}
	suite.Require().NoError(msgInternal.Sign(execution.MainPrivateKey))
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
	msgExternalData, err := msgExternal.MarshalSSZ()
	suite.Require().NoError(err)

	request := &Request{
		Jsonrpc: "2.0",
		Method:  sendRawTransaction,
		Params:  []any{"0x" + hex.EncodeToString(msgExternalData)},
		Id:      1,
	}

	resp, err := makeRequest[common.Hash](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error)
	suite.Equal(msgExternal.Hash(), resp.Result)

	suite.waitForReceiptOnShard(types.MainWalletAddress.ShardId(), msgExternal)

	suite.waitForReceipt(addrWallet, msgInternal)

	return addrWallet
}

func (suite *SuiteRpc) sendMessageViaWallet(addrFrom types.Address, messageToSend *types.Message) {
	seqno := suite.getTransactionCount(addrFrom, "latest")

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
	msgExternalData, err := msgExternal.MarshalSSZ()
	suite.Require().NoError(err)

	request := &Request{
		Jsonrpc: "2.0",
		Method:  sendRawTransaction,
		Params:  []any{"0x" + hex.EncodeToString(msgExternalData)},
		Id:      1,
	}

	resp, err := makeRequest[common.Hash](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error)
	suite.Equal(msgExternal.Hash(), resp.Result)

	suite.waitForReceipt(addrFrom, msgExternal)

	suite.waitForReceipt(messageToSend.To, messageToSend)
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
	seqno := suite.getTransactionCount(addrCallee, "latest")
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
	msgExternalData, err := messageToSend.MarshalSSZ()
	suite.Require().NoError(err)

	request := &Request{
		Jsonrpc: "2.0",
		Method:  sendRawTransaction,
		Params:  []any{"0x" + hex.EncodeToString(msgExternalData)},
		Id:      1,
	}

	resp, err := makeRequest[common.Hash](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error)
	suite.Equal(messageToSend.Hash(), resp.Result)

	suite.waitForReceiptOnShard(addrCallee.ShardId(), messageToSend)
}
