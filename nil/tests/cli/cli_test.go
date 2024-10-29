package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	nilcrypto "github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteCli struct {
	tests.ShardedSuite
	client client.Client
	cli    *cliservice.Service

	endpoint       string
	cometaEndpoint string
	incBinPath     string
	incAbiPath     string
}

func (s *SuiteCli) SetupSuite() {
	s.TmpDir = s.T().TempDir()

	s.incBinPath = s.TmpDir + "/Incrementer.bin"
	s.incAbiPath = s.TmpDir + "/Incrementer.abi"
	s.compileIncrementerAndSaveToFile(s.incBinPath, s.incAbiPath)
}

func (s *SuiteCli) SetupTest() {
	s.Start(&nilservice.Config{
		NShards:              5,
		CollatorTickPeriodMs: 200,
	}, 10225)

	time.Sleep(1 * time.Second)

	s.client, s.endpoint = s.StartRPCNode(10130)
	s.cometaEndpoint = strings.Replace(rpc.GetSockPathService(s.T(), "cometa"), "tcp://", "http://", 1)

	s.cli = cliservice.NewService(s.client, execution.MainPrivateKey)
	s.Require().NotNil(s.cli)
}

func (s *SuiteCli) TearDownTest() {
	s.Cancel()
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
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	_, receipt := s.DeployContractViaMainWallet(s.client, types.BaseShardId, deployPayload, types.NewValueFromUint64(5_000_000))
	s.Require().True(receipt.Success)

	msg, err := s.client.GetInMessageByHash(types.MainWalletAddress.ShardId(), receipt.MsgHash)
	s.Require().NoError(err)
	s.Require().NotNil(msg)
	s.Require().True(msg.Success)

	res, err := s.cli.FetchMessageByHashJson(types.MainWalletAddress.ShardId(), receipt.MsgHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(msg), string(res))

	res, err = s.cli.FetchReceiptByHashJson(types.MainWalletAddress.ShardId(), receipt.MsgHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(receipt), string(res))
}

func (s *SuiteCli) TestReadContract() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(1))

	addr, receipt := s.DeployContractViaMainWallet(s.client, types.BaseShardId, deployPayload, types.NewValueFromUint64(5_000_000))
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

	contractData, err := solc.CompileSource(common.GetAbsolutePath("../contracts/increment.sol"))
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
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployCode := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaWallet(wallet.ShardId()+1, wallet, deployCode, types.Value{})
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(s.client, wallet.ShardId(), txHash)
	s.Require().True(receipt.AllSuccess())

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, s.GasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res.Data.String())

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, calldata, types.Value{}, types.Value{}, nil, addr)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(s.client, wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, s.GasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res.Data.String())

	// Inc value via read-only call
	res, err = s.cli.CallContract(addr, s.GasToValue(100000), calldata, nil)
	s.Require().NoError(err)

	// Get updated value with overrides
	res, err = s.cli.CallContract(addr, s.GasToValue(100000), getCalldata, &res.StateOverrides)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000004", res.Data.String())

	// Get value without overrides
	res, err = s.cli.CallContract(addr, s.GasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res.Data.String())

	// Test value transfer
	balanceBefore, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(wallet, nil, types.Value{}, types.NewValueFromUint64(100), nil, addr)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(s.client, wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balanceAfter, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	s.EqualValues(uint64(100), balanceAfter.Uint64()-balanceBefore.Uint64())
}

func (s *SuiteCli) testNewWalletOnShard(shardId types.ShardId) {
	s.T().Helper()

	ownerPrivateKey, err := crypto.GenerateKey()
	s.Require().NoError(err)

	walletCode := contracts.PrepareDefaultWalletForOwnerCode(crypto.CompressPubkey(&ownerPrivateKey.PublicKey))
	code := types.BuildDeployPayload(walletCode, common.EmptyHash)
	expectedAddress := types.CreateAddress(shardId, code)
	walletAddres, err := s.cli.CreateWallet(shardId, types.NewUint256(0), types.NewValueFromUint64(10_000_000), types.Value{}, &ownerPrivateKey.PublicKey)
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

	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/external_increment.sol"), "ExternalIncrementer")
	deployCode := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaWallet(types.BaseShardId, wallet, deployCode, types.NewValueFromUint64(10_000_000))
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(s.client, wallet.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balance, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)
	s.Equal(uint64(10000000), balance.Uint64())

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, s.GasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res.Data.String())

	// Call contract method
	calldata, err := abi.Pack("increment", big.NewInt(123))
	s.Require().NoError(err)

	txHash, err = s.cli.SendExternalMessage(calldata, addr, true)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(s.client, addr.ShardId(), txHash)
	s.Require().True(receipt.Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, s.GasToValue(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x000000000000000000000000000000000000000000000000000000000000007d", res.Data.String())
}

func (s *SuiteCli) TestCurrency() {
	wallet := types.MainWalletAddress
	value := types.NewValueFromUint64(12345)

	// create currency
	_, err := s.cli.CurrencyCreate(wallet, value, "token1")
	s.Require().NoError(err)
	cur, err := s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	currencyId := *types.CurrencyIdForAddress(wallet)
	val, ok := cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(value, val)

	// mint
	_, err = s.cli.ChangeCurrencyAmount(wallet, value, true)
	s.Require().NoError(err)
	cur, err = s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	val, ok = cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(2*value.Uint64(), val.Uint64())

	// burn
	_, err = s.cli.ChangeCurrencyAmount(wallet, types.NewValueFromUint64(2*value.Uint64()), false)
	s.Require().NoError(err)
	cur, err = s.cli.GetCurrencies(wallet)
	s.Require().NoError(err)
	s.Require().Len(cur, 1)

	val, ok = cur[currencyId]
	s.Require().True(ok)
	s.Require().Equal(uint64(0), val.Uint64())
}

func (s *SuiteCli) TestCallCliHelp() {
	res := s.RunCli("help")

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

	res := s.RunCli("-c", cfgPath, "block", "--json", block.Number.String())
	s.Contains(res, block.Number.String())
	s.Contains(res, block.Hash.String())
}

func (s *SuiteCli) TestCliP2pKeygen() {
	res := s.RunCli("keygen", "new-p2p", "-q")
	lines := strings.Split(res, "\n")
	s.Require().Len(lines, 3)
}

func (s *SuiteCli) TestCliWallet() {
	dir := s.T().TempDir()

	cfgPath := dir + "/config.ini"
	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	s.Run("Deploy new wallet", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "wallet", "new")
		s.Require().Error(err)
		s.Contains(res, "Error: private_key not specified in config")
	})

	res := s.RunCli("-c", cfgPath, "keygen", "new")
	s.Run("Generate a key", func() {
		s.Contains(res, "Private key:")
	})

	s.Run("Address not specified", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "wallet", "info")
		s.Require().Error(err)
		s.Contains(res, "Error: address not specified in config")
	})

	s.Run("Deploy new wallet", func() {
		res := s.RunCli("-c", cfgPath, "wallet", "new")
		s.Contains(res, "New wallet address:")
	})

	var addr string
	s.Run("Get contract address", func() {
		addr = s.RunCli("-c", cfgPath, "contract", "address", s.incBinPath, "123321", "--abi", s.incAbiPath, "-q")
	})

	res = s.RunCliCfg("-c", cfgPath, "wallet", "deploy", s.incBinPath, "123321", "--abi", s.incAbiPath)
	s.Run("Deploy contract", func() {
		s.Contains(res, "Contract address")
		s.Contains(res, addr)
	})

	s.Run("Check deploy message result and receipt", func() {
		hash := strings.TrimPrefix(res, "Message hash: ")[:66]

		res = s.RunCli("-c", cfgPath, "message", hash)
		s.Contains(res, "Message data:")
		s.Contains(res, "\"success\": true")

		res = s.RunCli("-c", cfgPath, "receipt", hash)
		s.Contains(res, "Receipt data:")
		s.Contains(res, "\"success\": true")
	})

	s.Run("Check contract code", func() {
		res := s.RunCli("-c", cfgPath, "contract", "code", addr)
		s.Contains(res, "Contract code: 0x6080")
	})

	s.Run("Call read-only 'get' function of contract", func() {
		res := s.RunCli("-c", cfgPath, "contract", "call-readonly", addr, "get", "--abi", s.incAbiPath)
		s.Contains(res, "uint256: 123321")
	})

	s.Run("Estimate fee", func() {
		isNum := func(str string) {
			s.T().Helper()
			_, err := strconv.ParseUint(str, 0, 64)
			s.Require().NoError(err)
		}

		resExt := s.RunCli("-c", cfgPath, "contract", "estimate-fee", addr, "increment", "--abi", s.incAbiPath, "-q")
		isNum(resExt)

		resInt := s.RunCli("-c", cfgPath, "contract", "estimate-fee", addr, "increment", "--abi", s.incAbiPath, "-q", "--internal")
		isNum(resInt)

		resWallet := s.RunCli("-c", cfgPath, "wallet", "estimate-fee", addr, "increment", "--abi", s.incAbiPath, "-q")
		isNum(resWallet)
	})

	s.Run("Call 'increment' function of contract", func() {
		res := s.RunCli("-c", cfgPath, "wallet", "send-message", addr, "increment", "--abi", s.incAbiPath)
		s.Contains(res, "Message hash")
	})

	s.Run("Call read-only 'get' function of contract once again", func() {
		res := s.RunCli("-c", cfgPath, "contract", "call-readonly", addr, "get", "--abi", s.incAbiPath)
		s.Contains(res, "uint256: 123322")
	})

	overridesFile := dir + "/overrides.json"
	s.Run("Call read-only 'increment' via the wallet", func() {
		res := s.RunCli("-c", cfgPath, "wallet", "call-readonly", addr, "increment", "--abi", s.incAbiPath, "--out-overrides", overridesFile)
		s.Contains(res, "Success, no result")
	})

	s.Run("Check overrides file content", func() {
		res := make(map[string]interface{})
		data, err := os.ReadFile(overridesFile)
		s.Require().NoError(err)
		s.Require().NoError(json.Unmarshal(data, &res))
		s.Require().Len(res, 2)
		s.Contains(res, addr)
	})

	s.Run("Call read-only 'get' via the wallet", func() {
		res := s.RunCli("-c", cfgPath, "wallet", "call-readonly", addr, "get", "--abi", s.incAbiPath, "--in-overrides", overridesFile)
		s.Contains(res, "uint256: 123323")
	})
}

func (s *SuiteCli) TestCliWalletCurrency() {
	dir := s.T().TempDir()

	cfgPath := dir + "/config.ini"
	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	s.Run("Deploy new wallet", func() {
		s.RunCli("-c", cfgPath, "keygen", "new")
		res := s.RunCli("-c", cfgPath, "wallet", "new")
		s.Contains(res, "New wallet address:")
	})

	var addr types.Address
	s.Run("Get address", func() {
		res := s.RunCli("-c", cfgPath, "wallet", "info", "-q")
		s.Require().NoError(addr.Set(strings.Split(res, "\n")[0]))
	})

	s.Run("Top-up BTC", func() {
		res := s.RunCli("-c", cfgPath, "wallet", "top-up", "10000")
		s.Contains(res, "Wallet balance:")
		s.Contains(res, "[NIL]")

		res = s.RunCli("-c", cfgPath, "wallet", "top-up", "10000", "BTC")
		s.Contains(res, "Wallet balance: 10000 [BTC]")

		res = s.RunCli("-c", cfgPath, "contract", "currencies", addr.Hex())
		s.Contains(res, types.BtcFaucetAddress.Hex()+"\t10000\t[BTC]")

		s.RunCli("-c", cfgPath, "wallet", "top-up", "20000", types.BtcFaucetAddress.Hex())
		res = s.RunCli("-c", cfgPath, "contract", "currencies", addr.Hex())
		s.Contains(res, types.BtcFaucetAddress.Hex()+"\t30000\t[BTC]")
	})

	s.Run("Top-up unknown currency", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "wallet", "top-up", "123", "Unknown")
		s.Require().Error(err)
		s.Contains(res, "Error: undefined currency id: Unknown")
	})
}

func (s *SuiteCli) TestCliAbi() {
	s.Run("Encode", func() {
		res := s.RunCli("abi", "encode", "get", "--path", s.incAbiPath)
		s.Equal("0x6d4ce63c", res)
	})

	s.Run("Decode", func() {
		res := s.RunCli("abi", "decode", "get", "0x000000000000000000000000000000000000000000000000000000000001e1ba", "--path", s.incAbiPath)
		s.Equal("uint256: 123322", res)
	})
}

func (s *SuiteCli) TestCliEncodeInternalMessage() {
	calldata := s.RunCli("abi", "encode", "get", "--path", s.incAbiPath)
	s.Equal("0x6d4ce63c", calldata)

	addr := "0x00041945255839dcbd3001fd5e6abe9ee970a797"
	res := s.RunCli("message", "encode-internal", "--to", addr, "--data", calldata, "--fee-credit", "5000000")

	expected := "0x0000404b4c0000000000000000000000000000000000000000000000000000000000030000000000000000041945255839dcbd3001fd5e6abe9ee970a797000000000000000000000000000000000000000000000000000000000000000000000000000000009a00000000000000000000000000000000000000000000000000000000000000000000009a00000000000000000000009e0000006d4ce63c"
	s.Contains(res, "\"feeCredit\": \"5000000\"")
	s.Contains(res, "\"forwardKind\": 3")
	s.Contains(res, "Result: "+expected)

	res = s.RunCli("message", "encode-internal", "--to", addr, "--data", calldata, "--fee-credit", "5000000", "-q")
	s.Equal(expected, res)
}

func (s *SuiteCli) TestCliConfig() {
	dir := s.T().TempDir()

	cfgPath := dir + "/config.ini"

	s.Run("Create config", func() {
		res := s.RunCli("-c", cfgPath, "config", "init")
		s.Contains(res, "The config file has been initialized successfully: "+cfgPath)
	})

	s.Run("Set config value", func() {
		res := s.RunCli("-c", cfgPath, "config", "set", "rpc_endpoint", s.endpoint)
		s.Contains(res, fmt.Sprintf("Set \"rpc_endpoint\" to %q", s.endpoint))
	})

	s.Run("Read config value", func() {
		res := s.RunCli("-c", cfgPath, "config", "get", "rpc_endpoint")
		s.Contains(res, "rpc_endpoint: "+s.endpoint)
	})

	s.Run("Show config", func() {
		res := s.RunCli("-c", cfgPath, "config", "show")
		s.Contains(res, "rpc_endpoint: "+s.endpoint)
	})
}

func (s *SuiteCli) TestCliCometa() {
	cfg := &cometa.Config{
		UseBadger:   true,
		DbPath:      s.T().TempDir() + "/cometa.db",
		OwnEndpoint: s.cometaEndpoint,
	}
	com, err := cometa.NewService(s.Context, cfg, s.client)
	s.Require().NoError(err)
	go func() {
		check.PanicIfErr(com.Run(s.Context, cfg))
	}()
	s.createConfigFile()
	abiFile := "../../contracts/compiled/tests/Counter.abi"

	var address types.Address
	var msgHash string

	s.Run("Deploy counter", func() {
		out := s.RunCliCfg("wallet", "deploy", "--compile-input", "../contracts/counter-compile.json", "--shard-id", "1")
		parts := strings.Split(out, "\n")
		s.Require().Len(parts, 2)
		parts = strings.Split(parts[1], ": ")
		s.Require().Len(parts, 2)
		address = types.HexToAddress(parts[1])
	})

	s.Run("Get metadata", func() {
		out := s.RunCliCfg("cometa", "info", "--address", address.Hex())
		s.Contains(out, "Name: Counter.sol:Counter")
	})

	s.Run("Call Counter.get()", func() {
		out := s.RunCliCfg("wallet", "send-message", address.Hex(), "--abi", abiFile, "--fee-credit", "50000000", "get")
		parts := strings.Split(out, ": ")
		s.Require().Len(parts, 2)
		msgHash = parts[1]
	})

	s.Run("Debug", func() {
		out := s.RunCliCfg("debug", msgHash)
		fmt.Println(out)
		result := parseCometaOutput(out)
		s.Require().Len(result, 3)
		s.Require().Equal("unknown", result[0]["Contract"])
		s.Require().Equal("0x2bb1ae7c", result[0]["CallData"][:10])
		s.Require().Contains(result[0]["Message"], msgHash)
		s.Require().Equal("Counter", result[1]["Contract"])
		s.Require().Equal("get()", result[1]["CallData"])
		s.Require().Equal("unknown", result[2]["Contract"])
		s.Contains(out, "Contract   : Counter")
	})

	s.Run("Deploy wallet to test ctor arguments", func() {
		out := s.RunCliCfg("wallet", "deploy",
			"--compile-input", "../../contracts/solidity/compile-wallet.json",
			"--abi", "../../contracts/compiled/Wallet.abi",
			"--shard-id", "1",
			"0x12345678")
		parts := strings.Split(out, "\n")
		s.Require().Len(parts, 2)
		parts = strings.Split(parts[1], ": ")
		s.Require().Len(parts, 2)
		address = types.HexToAddress(parts[1])
	})

	s.Run("Get wallet metadata", func() {
		out := s.RunCliCfg("cometa", "info", "--address", address.Hex())
		s.Contains(out, "Name: Wallet.sol:Wallet")
	})

	s.Run("Register metadata for main wallet", func() {
		out := s.RunCliCfg("cometa", "register",
			"--address", types.MainWalletAddress.Hex(),
			"--compile-input", "../../contracts/solidity/compile-wallet.json")
		s.Require().Equal(
			"Contract metadata for address 0x0001111111111111111111111111111111111111 has been registered", out)
	})
}

func parseCometaOutput(out string) []map[string]string {
	res := make([]map[string]string, 0)
	var currMsg map[string]string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, ": ")
		if strings.Contains(parts[0], "Message") {
			currMsg = make(map[string]string, 0)
			res = append(res, currMsg)
			currMsg["Message"] = strings.TrimSpace(parts[1])
		} else {
			currMsg[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return res
}

func (s *SuiteCli) createConfigFile() {
	s.T().Helper()

	cfgPath := s.TmpDir + "/config.ini"

	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	iniData += "cometa_endpoint = " + s.cometaEndpoint + "\n"
	iniData += "private_key = " + nilcrypto.PrivateKeyToEthereumFormat(execution.MainPrivateKey) + "\n"
	iniData += "address = 0x0001111111111111111111111111111111111111\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)
}

func (s *SuiteCli) RunCliCfg(args ...string) string {
	s.T().Helper()
	args = append([]string{"-c", s.TmpDir + "/config.ini"}, args...)
	return s.RunCli(args...)
}

func TestSuiteCli(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteCli))
}
