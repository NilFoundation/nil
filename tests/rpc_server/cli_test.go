package rpctest

import (
	"encoding/json"

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
	contractCode, _ := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")

	_, receipt := s.deployContractViaMainWallet(types.BaseShardId, contractCode, *types.NewUint256(5_000_000))
	s.Require().True(receipt.Success)

	msg, err := s.client.GetInMessageByHash(types.MasterShardId, receipt.MsgHash)
	s.Require().NoError(err)
	s.Require().NotNil(msg)
	s.Require().True(msg.Success)

	res, err := s.cli.FetchMessageByHash(types.MasterShardId, receipt.MsgHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(msg), string(res))

	res, err = s.cli.FetchReceiptByHash(types.MasterShardId, receipt.MsgHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(receipt), string(res))
}

func (s *SuiteRpc) TestReadContract() {
	contractCode, _ := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")

	addr, receipt := s.deployContractViaMainWallet(types.BaseShardId, contractCode, *types.NewUint256(5_000_000))
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
	txHash, addrStr, err := s.cli.DeployContract(types.BaseShardId, wallet, contractCode)
	s.Require().NoError(err)
	addr := types.HexToAddress(addrStr)

	receipt := s.waitForReceiptOnShard(types.MasterShardId, common.HexToHash(txHash))
	s.Require().True(receipt.Success)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", res)

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, calldata, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(types.MasterShardId, common.HexToHash(txHash))
	s.Require().True(receipt.Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000001", res)
}
