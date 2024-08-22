package rpctest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteCli struct {
	RpcSuite
	cli *cliservice.Service
}

func (s *SuiteCli) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              5,
		HttpUrl:              GetSockPath(s.T()),
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	})

	s.cli = cliservice.NewService(s.client, execution.MainPrivateKey)
	s.Require().NotNil(s.cli)
}

func (s *SuiteCli) TearDownTest() {
	s.cancel()
}

func (s *SuiteCli) toJSON(v interface{}) string {
	s.T().Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	s.Require().NoError(err)

	return string(data)
}

func (s *SuiteCli) TestCliBlock() {
	block, err := s.client.GetBlock(types.BaseShardId, 0, false)
	s.Require().NoError(err)

	res, err := s.cli.FetchBlock(types.BaseShardId, block.Hash.Hex())
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(block), string(res))

	res, err = s.cli.FetchBlock(types.BaseShardId, "0")
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(block), string(res))
}

func (s *SuiteCli) TestCliMessage() {
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

func (s *SuiteCli) TestReadContract() {
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

func (s *SuiteCli) compileIncrementerAndSaveToFile(binFileName string, abiFileName string) {
	s.T().Helper()

	contractData, err := solc.CompileSource(common.GetAbsolutePath("./contracts/increment.sol"))
	s.Require().NoError(err)

	if len(binFileName) > 0 {
		err = os.WriteFile(binFileName, []byte(contractData["Incrementer"].Code), 0o600)
		s.Require().NoError(err)
	}

	if len(abiFileName) > 0 {
		abiData, err := json.Marshal(contractData["Incrementer"].Info.AbiDefinition)
		s.Require().NoError(err)
		err = os.WriteFile(abiFileName, abiData, 0o600)
		s.Require().NoError(err)
	}
}

func (s *SuiteCli) TestContract() {
	wallet := types.MainWalletAddress

	// Deploy contract
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployCode := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaWallet(wallet.ShardId()+1, wallet, deployCode, types.Value{})
	s.Require().NoError(err)

	receipt := s.waitForReceipt(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, s.gasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res.Data.String())

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, calldata, s.gasToValue(100_000), s.gasToValue(100_000), types.Value{}, nil, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceipt(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, s.gasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res.Data.String())

	// Inc value via read-only call
	res, err = s.cli.CallContract(addr, s.gasToValue(100000), calldata, nil)
	s.Require().NoError(err)

	// Get updated value with overrides
	res, err = s.cli.CallContract(addr, s.gasToValue(100000), getCalldata, &res.StateOverrides)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000004", res.Data.String())

	// Get value without overrides
	res, err = s.cli.CallContract(addr, s.gasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res.Data.String())

	// Test value transfer
	balanceBefore, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, nil, s.gasToValue(100_000), s.gasToValue(100_000), types.NewValueFromUint64(100), nil, addr)
	s.Require().NoError(err)

	receipt = s.waitForReceipt(wallet.ShardId(), txHash)
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

func (s *SuiteCli) testNewWalletOnShard(shardId types.ShardId) {
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

func (s *SuiteCli) TestNewWalletOnFaucetShard() {
	s.testNewWalletOnShard(types.FaucetAddress.ShardId())
}

func (s *SuiteCli) TestNewWalletOnRandomShard() {
	s.testNewWalletOnShard(types.FaucetAddress.ShardId() + 1)
}

func (s *SuiteCli) TestSendExternalMessage() {
	wallet := types.MainWalletAddress

	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/external_increment.sol"), "ExternalIncrementer")
	deployCode := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaWallet(types.BaseShardId, wallet, deployCode, types.NewValueFromUint64(10_000_000))
	s.Require().NoError(err)

	receipt := s.waitForReceipt(wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balance, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)
	s.Equal("10000000", balance)

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, s.gasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res.Data.String())

	// Call contract method
	calldata, err := abi.Pack("increment", big.NewInt(123))
	s.Require().NoError(err)

	txHash, err = s.cli.SendExternalMessage(calldata, addr, true)
	s.Require().NoError(err)

	receipt = s.waitForReceipt(addr.ShardId(), txHash)
	s.Require().True(receipt.Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, s.gasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x000000000000000000000000000000000000000000000000000000000000007d", res.Data.String())
}

func (s *SuiteCli) TestCurrency() {
	wallet := types.MainWalletAddress
	value := types.NewValueFromUint64(12345)

	_, err := s.cli.CurrencyCreate(wallet, value, "token1")
	s.Require().NoError(err)
	cur, err := s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	currencyId := hexutil.ToHexNoLeadingZeroes(types.CurrencyIdForAddress(wallet)[:])
	val, ok := cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(value, val)

	_, err = s.cli.CurrencyMint(wallet, value)
	s.Require().NoError(err)
	cur, err = s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	val, ok = cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(2*value.Uint64(), val.Uint64())
}

func (s *SuiteCli) TestCallCliHelp() {
	res := s.runCli("help")

	for _, cmd := range []string{"block", "message", "contract", "wallet", "completion"} {
		s.Contains(res, cmd)
	}
}

func (s *SuiteCli) TestCallCliBasic() {
	cfgPath := s.T().TempDir() + "/config.ini"

	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	block, err := s.client.GetBlock(types.BaseShardId, "latest", false)
	s.Require().NoError(err)

	res := s.runCli("-c", cfgPath, "block", "--json", block.Number.String())
	s.Contains(res, block.Number.String())
	s.Contains(res, block.Hash.String())
}

func (s *SuiteCli) TestCliWallet() {
	dir := s.T().TempDir()

	cfgPath := dir + "/config.ini"
	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	s.Run("Deploy new wallet", func() {
		res, err := s.runCliNoCheck("-c", cfgPath, "wallet", "new")
		s.Require().Error(err)
		s.Contains(res, "Error: Private key is not specified in config")
	})

	res := s.runCli("-c", cfgPath, "keygen", "new")
	s.Run("Generate a key", func() {
		s.Contains(res, "Private key:")
	})

	s.Run("Address not specified", func() {
		res, err := s.runCliNoCheck("-c", cfgPath, "wallet", "info")
		s.Require().Error(err)
		s.Contains(res, "Error: Valid wallet address is not specified in config")
	})

	s.Run("Deploy new wallet", func() {
		res := s.runCli("-c", cfgPath, "wallet", "new")
		s.Contains(res, "New wallet address:")
	})

	binFileName := dir + "/Incrementer.bin"
	abiFileName := dir + "/Incrementer.abi"
	s.compileIncrementerAndSaveToFile(binFileName, abiFileName)

	var addr types.Address
	s.Run("Get contract address", func() {
		res := s.runCli("-c", cfgPath, "contract", "address", dir+"/Incrementer.bin", "123321", "--abi", abiFileName, "-q")
		s.Require().NoError(addr.Set(res))
	})

	res = s.runCli("-c", cfgPath, "wallet", "deploy", dir+"/Incrementer.bin", "123321", "--abi", abiFileName)
	s.Run("Deploy contract", func() {
		s.Contains(res, "Contract address")
		s.Contains(res, strings.ToLower(addr.String()))
	})

	s.Run("Check deploy message result and receipt", func() {
		hash := strings.TrimPrefix(res, "Message hash: ")[:66]

		res = s.runCli("-c", cfgPath, "message", hash)
		s.Contains(res, "Message data:")
		s.Contains(res, "\"success\": true")

		res = s.runCli("-c", cfgPath, "receipt", hash)
		s.Contains(res, "Receipt data:")
		s.Contains(res, "\"success\": true")
	})

	s.Run("Check contract code", func() {
		res := s.runCli("-c", cfgPath, "contract", "code", addr.String())
		s.Contains(res, "Contract code: 0x6080")
	})

	s.Run("Call read-only 'get' function of contract", func() {
		res := s.runCli("-c", cfgPath, "contract", "call-readonly", addr.String(), "get", "--abi", abiFileName)
		s.Contains(res, "uint256: 123321")
	})

	s.Run("Call 'increment' function of contract", func() {
		res := s.runCli("-c", cfgPath, "wallet", "send-message", addr.String(), "increment", "--abi", abiFileName)
		s.Contains(res, "Message hash")
	})

	s.Run("Call read-only 'get' function of contract once again", func() {
		res := s.runCli("-c", cfgPath, "contract", "call-readonly", addr.String(), "get", "--abi", abiFileName)
		s.Contains(res, "uint256: 123322")
	})

	overridesFile := dir + "/overrides.json"
	s.Run("Call read-only via the wallet", func() {
		res := s.runCli("-c", cfgPath, "wallet", "call-readonly", addr.String(), "increment", "--abi", abiFileName, "--out-overrides", overridesFile)
		s.Contains(res, "Success, no result")
	})

	s.Run("Call read-only via the wallet", func() {
		res := s.runCli("-c", cfgPath, "wallet", "call-readonly", addr.String(), "get", "--abi", abiFileName, "--in-overrides", overridesFile)
		s.Contains(res, "uint256: 123323")
	})
}

func (s *SuiteCli) TestCliAbi() {
	dir := s.T().TempDir()

	abiFileName := dir + "/Incrementer.abi"
	s.compileIncrementerAndSaveToFile("", abiFileName)

	s.Run("Encode", func() {
		res := s.runCli("abi", "encode", "get", "--path", abiFileName)
		s.Equal("0x6d4ce63c", res)
	})

	s.Run("Decode", func() {
		res := s.runCli("abi", "decode", "get", "0x000000000000000000000000000000000000000000000000000000000001e1ba", "--path", abiFileName)
		s.Equal("uint256: 123322", res)
	})
}

func (s *SuiteCli) TestCliEncodeInternalMessage() {
	dir := s.T().TempDir()

	abiFileName := dir + "/Incrementer.abi"
	s.compileIncrementerAndSaveToFile("", abiFileName)

	calldata := s.runCli("abi", "encode", "get", "--path", abiFileName)
	s.Equal("0x6d4ce63c", calldata)

	addr := "0x00041945255839dcbd3001fd5e6abe9ee970a797"
	res := s.runCli("message", "encode-internal", "--to", addr, "--data", calldata, "--fee-credit", "5000000")

	expected := "0x0000404b4c0000000000000000000000000000000000000000000000000000000000030000000000000000041945255839dcbd3001fd5e6abe9ee970a797000000000000000000000000000000000000000000000000000000000000000000000000000000009600000000000000000000000000000000000000000000000000000000000000000000009600000000000000000000006d4ce63c"
	s.Contains(res, "\"feeCredit\": \"5000000\"")
	s.Contains(res, "\"forwardKind\": 3")
	s.Contains(res, "Result: "+expected)

	res = s.runCli("message", "encode-internal", "--to", addr, "--data", calldata, "--fee-credit", "5000000", "-q")
	s.Equal(expected, res)
}

func (s *SuiteCli) TestCliConfig() {
	dir := s.T().TempDir()

	cfgPath := dir + "/config.ini"

	s.Run("Create config", func() {
		res := s.runCli("-c", cfgPath, "config", "init")
		s.Contains(res, "Config initialized successfully: "+cfgPath)
	})

	s.Run("Set config value", func() {
		res := s.runCli("-c", cfgPath, "config", "set", "rpc_endpoint", s.endpoint)
		s.Contains(res, fmt.Sprintf("Set \"rpc_endpoint\" to %q", s.endpoint))
	})

	s.Run("Read config value", func() {
		res := s.runCli("-c", cfgPath, "config", "get", "rpc_endpoint")
		s.Contains(res, "rpc_endpoint: "+s.endpoint)
	})

	s.Run("Show config", func() {
		res := s.runCli("-c", cfgPath, "config", "show")
		s.Contains(res, "rpc_endpoint: "+s.endpoint)
	})
}

func TestSuiteCli(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteCli))
}
