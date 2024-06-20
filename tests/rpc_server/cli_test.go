package rpctest

import (
	"encoding/json"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

func (s *SuiteRpc) toJSON(v interface{}) string {
	s.T().Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	s.Require().NoError(err)

	return string(data)
}

func (s *SuiteRpc) TestCliBlock() {
	block, err := s.client.GetBlock(types.BaseShardId, 0, false)
	s.Require().NoError(err)

	res, err := s.cli.FetchBlock(types.BaseShardId, block.Hash.Hex())
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(block), string(res))

	res, err = s.cli.FetchBlock(types.BaseShardId, "0")
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(block), string(res))
}

func (s *SuiteRpc) TestCliMessage() {
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	contractCode = s.prepareDefaultDeployBytecode(abi, contractCode, big.NewInt(0))

	_, receipt := s.deployContractViaMainWallet(types.BaseShardId, contractCode, types.NewUint256(5_000_000))
	s.Require().True(receipt.Success)

	msg, err := s.client.GetInMessageByHash(types.MainWalletAddress.ShardId(), receipt.MsgHash)
	s.Require().NoError(err)
	s.Require().NotNil(msg)
	s.Require().True(msg.Success)

	res, err := s.cli.FetchMessageByHash(types.MainWalletAddress.ShardId(), receipt.MsgHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(msg), string(res))

	res, err = s.cli.FetchReceiptByHash(types.MainWalletAddress.ShardId(), receipt.MsgHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(receipt), string(res))
}

func (s *SuiteRpc) TestReadContract() {
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	contractCode = s.prepareDefaultDeployBytecode(abi, contractCode, big.NewInt(1))

	addr, receipt := s.deployContractViaMainWallet(types.BaseShardId, contractCode, types.NewUint256(5_000_000))
	s.Require().True(receipt.Success)

	res, err := s.cli.GetCode(addr.String())
	s.Require().NoError(err)
	s.NotEmpty(res)
	s.NotEqual("0x", res)

	res, err = s.cli.GetCode("0x00000000000000000000")
	s.Require().NoError(err)
	s.Equal("0x", res)
}

func (s *SuiteRpc) TestContract() {
	wallet := types.MainWalletAddress

	// Deploy contract
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployCode := s.prepareDefaultDeployBytecode(abi, contractCode, big.NewInt(2))
	txHash, addrStr, err := s.cli.DeployContract(types.BaseShardId, wallet, deployCode)
	s.Require().NoError(err)
	addr := types.HexToAddress(addrStr)

	receipt := s.waitForReceiptOnShard(wallet.ShardId(), common.HexToHash(txHash))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res)

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, calldata, nil, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(wallet.ShardId(), common.HexToHash(txHash))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res)

	// Test value transfer
	balanceBefore, err := s.client.GetBalance(addr, "latest")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, nil, types.NewUint256(100), addr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(wallet.ShardId(), common.HexToHash(txHash))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balanceAfter, err := s.client.GetBalance(addr, "latest")
	s.Require().NoError(err)

	s.EqualValues(100, balanceBefore.Sub(&balanceAfter.Int, &balanceBefore.Int).Uint64())
}
