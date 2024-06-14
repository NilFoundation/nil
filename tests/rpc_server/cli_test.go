package rpctest

import (
	"encoding/hex"
	"encoding/json"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
)

func (s *SuiteRpc) toJSON(v interface{}) string {
	s.T().Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	s.Require().NoError(err)

	return string(data)
}

func (s *SuiteRpc) TestCliBlock() {
	s.cli.SetShardId(types.BaseShardId)

	block, err := s.client.GetBlock(types.BaseShardId, 0, false)
	s.Require().NoError(err)

	res, err := s.cli.FetchBlock(block.Hash.Hex())
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(block), string(res))

	res, err = s.cli.FetchBlock("0")
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(block), string(res))
}

func (s *SuiteRpc) TestCliMessage() {
	s.cli.SetShardId(types.MasterShardId)

	contractCode, _ := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")

	_, receipt := s.deployContractViaWallet(types.BaseShardId, contractCode)
	s.Require().True(receipt.Success)

	msg, err := s.client.GetInMessageByHash(types.MasterShardId, receipt.MsgHash)
	s.Require().NoError(err)
	s.Require().NotNil(msg)
	s.Require().True(msg.Success)

	res, err := s.cli.FetchMessageByHash(receipt.MsgHash.Hex())
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(msg), string(res))

	res, err = s.cli.FetchReceiptByHash(receipt.MsgHash.Hex())
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(receipt), string(res))
}

func (s *SuiteRpc) TestReadContract() {
	s.cli.SetShardId(types.MasterShardId)

	contractCode, _ := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")

	addr, receipt := s.deployContractViaWallet(types.BaseShardId, contractCode)
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
	s.cli.SetShardId(types.BaseShardId)
	wallet := types.MainWalletAddress.Hex()

	// Deploy contract
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	txHash, addrStr, err := s.cli.DeployContract(wallet, contractCode.Hex())
	s.Require().NoError(err)
	addr := types.HexToAddress(addrStr)

	receipt := s.waitForReceiptOnShard(types.MasterShardId, common.HexToHash(txHash))
	s.Require().True(receipt.Success)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	callArgsData := hexutil.Bytes(getCalldata)
	seqno := hexutil.Uint64(0)
	callArgs := jsonrpc.CallArgs{
		From:     addr,
		Data:     callArgsData,
		To:       addr,
		Value:    types.NewUint256(0),
		GasLimit: types.NewUint256(10000),
		Seqno:    &seqno,
	}

	// Get current value
	res, err := s.client.Call("eth_call", callArgs, "latest")
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", string(res[1:len(res)-1]))

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, hex.EncodeToString(calldata), addrStr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(types.MasterShardId, common.HexToHash(txHash))
	s.Require().True(receipt.Success)

	// Get updated value
	res, err = s.client.Call("eth_call", callArgs, "latest")
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000001", string(res[1:len(res)-1]))
}
