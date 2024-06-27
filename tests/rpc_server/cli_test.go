package rpctest

import (
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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
	deployPayload := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	_, receipt := s.deployContractViaMainWallet(types.BaseShardId, deployPayload, types.NewUint256(5_000_000))
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
	deployPayload := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(1))

	addr, receipt := s.deployContractViaMainWallet(types.BaseShardId, deployPayload, types.NewUint256(5_000_000))
	s.Require().True(receipt.Success)

	res, err := s.cli.GetCode(addr)
	s.Require().NoError(err)
	s.NotEmpty(res)
	s.NotEqual("0x", res)

	res, err = s.cli.GetCode(types.EmptyAddress)
	s.Require().NoError(err)
	s.Equal("0x", res)
}

func (s *SuiteRpc) TestContract() {
	wallet := types.MainWalletAddress

	// Deploy contract
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployCode := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaWallet(wallet.ShardId()+1, wallet, deployCode, nil)
	s.Require().NoError(err)

	receipt := s.waitForReceiptOnShard(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, types.NewUint256(100000), getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res)

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, calldata, types.NewUint256(100_000), nil, nil, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, types.NewUint256(100000), getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res)

	// Test value transfer
	balanceBefore, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, nil, types.NewUint256(100_000), types.NewUint256(100), nil, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balanceAfter, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	b1, err := strconv.ParseInt(balanceBefore, 10, 64)
	s.Require().NoError(err)
	b2, err := strconv.ParseInt(balanceAfter, 10, 64)
	s.Require().NoError(err)

	s.EqualValues(100, b2-b1)
}

func (s *SuiteRpc) testNewWalletOnShard(shardId types.ShardId) {
	s.T().Helper()

	ownerPrivateKey, err := crypto.GenerateKey()
	s.Require().NoError(err)

	walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(&ownerPrivateKey.PublicKey))
	code := types.BuildDeployPayload(walletCode, common.EmptyHash)
	expectedAddress := types.CreateAddress(shardId, code)
	walletAddres, err := s.cli.CreateWallet(shardId, *types.NewUint256(0), types.NewUint256(10_000_000), &ownerPrivateKey.PublicKey)
	s.Require().NoError(err)
	s.Require().Equal(expectedAddress, walletAddres)
}

func (s *SuiteRpc) TestNewWalletOnFaucetShard() {
	s.testNewWalletOnShard(types.FaucetAddress.ShardId())
}

func (s *SuiteRpc) TestNewWalletOnRandomShard() {
	s.testNewWalletOnShard(types.FaucetAddress.ShardId() + 1)
}

func (s *SuiteRpc) TestSendExternalMessage() {
	wallet := types.MainWalletAddress

	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/external_increment.sol"), "ExternalIncrementer")
	deployCode := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaWallet(types.BaseShardId, wallet, deployCode, types.NewUint256(10_000_000))
	s.Require().NoError(err)

	receipt := s.waitForReceiptOnShard(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balance, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)
	s.Equal("10000000", balance)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, types.NewUint256(100000), getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res)

	// Call contract method
	calldata, err := abi.Pack("increment", big.NewInt(123))
	s.Require().NoError(err)

	txHash, err = s.cli.SendExternalMessage(calldata, addr, true)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(addr.ShardId(), txHash)
	s.Require().True(receipt.Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, types.NewUint256(100000), getCalldata)
	s.Require().NoError(err)
	s.Equal("0x000000000000000000000000000000000000000000000000000000000000007d", res)
}
