package rpctest

import (
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/tools/solc"
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

	_, receipt := s.deployContractViaMainWallet(types.BaseShardId, deployPayload, types.NewValueFromUint64(5_000_000))
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

	addr, receipt := s.deployContractViaMainWallet(types.BaseShardId, deployPayload, types.NewValueFromUint64(5_000_000))
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
	txHash, addr, err := s.cli.DeployContractViaWallet(wallet.ShardId()+1, wallet, deployCode, types.Value{})
	s.Require().NoError(err)

	receipt := s.waitForReceiptOnShard(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, 100000, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res)

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, calldata, 100_000, types.Value{}, nil, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceiptOnShard(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, 100000, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res)

	// Test value transfer
	balanceBefore, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, nil, 100_000, types.NewValueFromUint64(100), nil, addr)
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
	walletAddres, err := s.cli.CreateWallet(shardId, types.NewUint256(0), types.NewValueFromUint64(10_000_000), &ownerPrivateKey.PublicKey)
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
	txHash, addr, err := s.cli.DeployContractViaWallet(types.BaseShardId, wallet, deployCode, types.NewValueFromUint64(10_000_000))
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
	res, err := s.cli.CallContract(addr, 100000, getCalldata)
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
	res, err = s.cli.CallContract(addr, 100000, getCalldata)
	s.Require().NoError(err)
	s.Equal("0x000000000000000000000000000000000000000000000000000000000000007d", res)
}

func (s *SuiteRpc) TestCurrency() {
	wallet := types.MainWalletAddress
	value := types.NewValueFromUint64(12345)
	s.Require().NoError(s.cli.CurrencyCreate(wallet, value, "token1", true))
	cur, err := s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	currencyId := hexutil.ToHexNoLeadingZeroes(types.CurrencyIdForAddress(wallet)[:])
	val, ok := cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(value, val)

	s.Require().NoError(s.cli.CurrencyMint(wallet, value, false))
	s.Require().NoError(s.cli.CurrencyWithdraw(wallet, value, wallet))
	cur, err = s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	val, ok = cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(2*value.Uint64(), val.Uint64())
}

func (s *SuiteRpc) runCli(args ...string) string {
	s.T().Helper()

	mainPath, err := filepath.Abs("../../cmd/nil_cli/main.go")
	s.Require().NoError(err)

	args = append([]string{"run", mainPath}, args...)
	cmd := exec.Command("go", args...)

	data, err := cmd.CombinedOutput()
	s.Require().NoErrorf(err, string(data))
	return string(data)
}

func (s *SuiteRpc) TestCallCliHelp() {
	res := s.runCli("help")

	for _, cmd := range []string{"block", "message", "contract", "wallet", "completion"} {
		s.Contains(res, cmd)
	}
}

func (s *SuiteRpc) TestCallCliBasic() {
	cfgPath := s.T().TempDir() + "/config.ini"

	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	block, err := s.client.GetBlock(types.BaseShardId, "latest", false)
	s.Require().NoError(err)

	res := s.runCli("-c", cfgPath, "block", block.Number.String())
	s.Contains(res, block.Number.String())
	s.Contains(res, block.Hash.String())
}

func (s *SuiteRpc) TestCliCreateWallet() {
	dir := s.T().TempDir()

	cfgPath := dir + "/config.ini"
	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	s.Run("Generate a key", func() {
		res := s.runCli("-c", cfgPath, "keygen", "new")
		s.Contains(res, "Pivate key:")
	})

	s.Run("Deploy new wallet", func() {
		res := s.runCli("-c", cfgPath, "wallet", "new")
		s.Contains(res, "New wallet address:")
	})

	binFileName := dir + "/Incrementer.bin"
	abiFileName := dir + "/Incrementer.abi"
	s.Run("Compile contract", func() {
		contractData, err := solc.CompileSource(common.GetAbsolutePath("./contracts/increment.sol"))
		s.Require().NoError(err)

		err = os.WriteFile(binFileName, []byte(contractData["Incrementer"].Code), 0o600)
		s.Require().NoError(err)

		abiData, err := json.Marshal(contractData["Incrementer"].Info.AbiDefinition)
		s.Require().NoError(err)
		err = os.WriteFile(abiFileName, abiData, 0o600)
		s.Require().NoError(err)
	})

	var addr types.Address
	s.Run("Get contract address", func() {
		res := s.runCli("-c", cfgPath, "contract", "address", dir+"/Incrementer.bin", "123321", "--abi", dir+"/Incrementer.abi")
		s.Contains(res, "Contract address:")
		s.Require().NoError(addr.Set(res[len(res)-43 : len(res)-1]))
	})

	s.Run("Deploy contract", func() {
		res := s.runCli("-c", cfgPath, "wallet", "deploy", dir+"/Incrementer.bin", "123321", "--abi", abiFileName)
		s.Contains(res, "Transaction hash:")
		s.Contains(res, strings.ToLower(addr.String()))
	})

	s.Run("Check contract code", func() {
		res := s.runCli("-c", cfgPath, "contract", "code", addr.String())
		s.Contains(res, "Contract code: 6080")
	})

	s.Run("Call read-only 'get' function of contract", func() {
		res := s.runCli("-c", cfgPath, "contract", "call-readonly", addr.String(), "get", "--abi", abiFileName)
		s.Contains(res, "Call result: 0x000000000000000000000000000000000000000000000000000000000001e1b9")
	})

	s.Run("Call 'increment' function of contract", func() {
		res := s.runCli("-c", cfgPath, "wallet", "send-message", addr.String(), "increment", "--abi", abiFileName)
		s.Contains(res, "Transaction hash:")
	})

	s.Run("Call read-only 'get' function of contract once again", func() {
		res := s.runCli("-c", cfgPath, "contract", "call-readonly", addr.String(), "get", "--abi", abiFileName)
		s.Contains(res, "Call result: 0x000000000000000000000000000000000000000000000000000000000001e1ba")
	})
}
