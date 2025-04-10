package cli_tests

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	nilcrypto "github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteCliBase struct {
	tests.ShardedSuite
	cli *cliservice.Service

	endpoint       string
	cometaEndpoint string
	faucetEndpoint string
	incBinPath     string
	incAbiPath     string
	basePort       int
}

func (s *SuiteCliBase) SetupSuite() {
	s.TmpDir = s.T().TempDir()

	s.incBinPath = s.TmpDir + "/Incrementer.bin"
	s.incAbiPath = s.TmpDir + "/Incrementer.abi"
	compileIncrementerAndSaveToFile(s.T(), s.incBinPath, s.incAbiPath)

	s.Start(&nilservice.Config{
		NShards:              3,
		CollatorTickPeriodMs: 200,
	}, s.basePort)

	s.DefaultClient, s.endpoint = s.StartRPCNode(tests.WithDhtBootstrapByValidators, nil)
	s.cometaEndpoint = rpc.GetSockPathService(s.T(), "cometa")

	var fc *faucet.Client
	fc, s.faucetEndpoint = tests.StartFaucetService(s.Context, s.T(), &s.Wg, s.DefaultClient)
	s.cli = cliservice.NewService(s.Context, s.DefaultClient, execution.MainPrivateKey, fc)
	s.Require().NotNil(s.cli)
}

func (s *SuiteCliBase) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteCliBase) toJSON(v any) string {
	s.T().Helper()

	data, err := json.MarshalIndent(v, "", "  ")
	s.Require().NoError(err)

	return string(data)
}

type SuiteCliService struct {
	SuiteCliBase
}

func (s *SuiteCliService) TestCliBlock() {
	block, err := s.DefaultClient.GetBlock(s.Context, types.BaseShardId, 0, false)
	s.Require().NoError(err)

	res, err := s.cli.FetchDebugBlock(types.BaseShardId, block.Hash.Hex(), true, false, true)
	s.Require().NoError(err)
	var blockRes map[string]any
	s.Require().NoError(json.Unmarshal(res, &blockRes))
	s.EqualValues(block.Number, blockRes["id"])

	res, err = s.cli.FetchDebugBlock(types.BaseShardId, "0", true, false, true)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(res, &blockRes))
	s.EqualValues(block.Number, blockRes["id"])

	res, err = s.cli.FetchDebugBlock(types.BaseShardId, "10000000000", true, false, true)
	s.Require().NoError(err)
	s.Require().Empty(res)
}

func (s *SuiteCliService) TestCliTransaction() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, common.Hash{0x1}, contractCode, big.NewInt(0))

	_, receipt := s.DeployContractViaMainSmartAccount(types.BaseShardId, deployPayload, types.GasToValue(5_000_000))
	s.Require().True(receipt.Success)

	txn, err := s.DefaultClient.GetInTransactionByHash(s.Context, receipt.TxnHash)
	s.Require().NoError(err)
	s.Require().NotNil(txn)
	s.Require().True(txn.Success)

	res, err := s.cli.FetchTransactionByHashJson(receipt.TxnHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(txn), string(res))

	res, err = s.cli.FetchReceiptByHashJson(receipt.TxnHash)
	s.Require().NoError(err)
	s.JSONEq(s.toJSON(receipt), string(res))
}

func (s *SuiteCliService) TestReadContract() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, common.Hash{0x2}, contractCode, big.NewInt(1))

	addr, receipt := s.DeployContractViaMainSmartAccount(types.BaseShardId, deployPayload, types.GasToValue(5_000_000))
	s.Require().True(receipt.Success)

	res, err := s.cli.GetCode(addr)
	s.Require().NoError(err)
	s.NotEmpty(res)
	s.NotEqual("0x", res)

	res, err = s.cli.GetCode(types.EmptyAddress)
	s.Require().NoError(err)
	s.Equal("0x", res)
}

func (s *SuiteCliService) TestContract() {
	smartAccount := types.MainSmartAccountAddress

	// Deploy contract
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployCode := s.PrepareDefaultDeployPayload(abi, common.Hash{0x3}, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaSmartAccount(
		smartAccount.ShardId()+1, smartAccount, deployCode, types.Value{})
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.AllSuccess())

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, types.NewFeePackFromGas(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res.Data.String())

	// Call contract method
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(smartAccount, calldata, types.FeePack{}, types.Value{}, nil, addr)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, types.NewFeePackFromGas(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res.Data.String())

	// Inc value via read-only call
	res, err = s.cli.CallContract(addr, types.NewFeePackFromGas(100000), calldata, nil)
	s.Require().NoError(err)

	// Get updated value with overrides
	res, err = s.cli.CallContract(addr, types.NewFeePackFromGas(100000), getCalldata, &res.StateOverrides)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000004", res.Data.String())

	// Get value without overrides
	res, err = s.cli.CallContract(addr, types.NewFeePackFromGas(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000003", res.Data.String())

	// Test value transfer
	balanceBefore, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	txHash, err = s.cli.RunContract(smartAccount, nil, types.FeePack{}, types.NewValueFromUint64(100), nil, addr)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balanceAfter, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)

	s.Equal(uint64(100), balanceAfter.Uint64()-balanceBefore.Uint64())
}

func (s *SuiteCliService) testNewSmartAccountOnShard(shardId types.ShardId) {
	s.T().Helper()

	ownerPrivateKey, err := crypto.GenerateKey()
	s.Require().NoError(err)

	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(
		crypto.CompressPubkey(&ownerPrivateKey.PublicKey))
	code := types.BuildDeployPayload(smartAccountCode, common.EmptyHash)
	expectedAddress := types.CreateAddress(shardId, code)
	smartAccountAddres, err := s.cli.CreateSmartAccount(shardId, types.NewUint256(0), types.GasToValue(10_000_000),
		types.FeePack{}, &ownerPrivateKey.PublicKey)
	s.Require().NoError(err)
	s.Require().Equal(expectedAddress, smartAccountAddres)
}

func (s *SuiteCliService) TestNewSmartAccountOnFaucetShard() {
	s.testNewSmartAccountOnShard(types.FaucetAddress.ShardId())
}

func (s *SuiteCliService) TestNewSmartAccountOnRandomShard() {
	s.testNewSmartAccountOnShard(types.FaucetAddress.ShardId() + 1)
}

func (s *SuiteCliService) TestSendExternalTransaction() {
	smartAccount := types.MainSmartAccountAddress

	contractCode, abi := s.LoadContract(
		common.GetAbsolutePath("../contracts/external_increment.sol"), "ExternalIncrementer")
	deployCode := s.PrepareDefaultDeployPayload(abi, common.Hash{0x4}, contractCode, big.NewInt(2))
	txHash, addr, err := s.cli.DeployContractViaSmartAccount(
		types.BaseShardId, smartAccount, deployCode, types.GasToValue(10_000_000))
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	balance, err := s.cli.GetBalance(addr)
	s.Require().NoError(err)
	s.Equal(uint64(200000000000000), balance.Uint64())

	getCalldata, err := abi.Pack("get")
	s.Require().NoError(err)

	// Get current value
	res, err := s.cli.CallContract(addr, types.NewFeePackFromGas(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x0000000000000000000000000000000000000000000000000000000000000002", res.Data.String())

	// Call contract method
	calldata, err := abi.Pack("increment", big.NewInt(123))
	s.Require().NoError(err)

	txHash, err = s.cli.SendExternalTransaction(calldata, addr, true)
	s.Require().NoError(err)

	receipt = s.WaitIncludedInMain(txHash)
	s.Require().True(receipt.Success)

	// Get updated value
	res, err = s.cli.CallContract(addr, types.NewFeePackFromGas(100000), getCalldata, nil)
	s.Require().NoError(err)
	s.Equal("0x000000000000000000000000000000000000000000000000000000000000007d", res.Data.String())
}

func (s *SuiteCliService) TestToken() {
	smartAccount := types.MainSmartAccountAddress
	value := types.NewValueFromUint64(12345)

	// create token
	_, err := s.cli.TokenCreate(smartAccount, value, "token1")
	s.Require().NoError(err)
	tok, err := s.cli.GetTokens(smartAccount)
	s.Require().NoError(err)
	s.Require().Len(tok, 1)

	tokenId := *types.TokenIdForAddress(smartAccount)
	val, ok := tok[tokenId]
	s.Require().True(ok)
	s.Require().Equal(value, val)

	// mint
	_, err = s.cli.ChangeTokenAmount(smartAccount, value, true)
	s.Require().NoError(err)
	tok, err = s.cli.GetTokens(smartAccount)
	s.Require().NoError(err)
	s.Require().Len(tok, 1)

	val, ok = tok[tokenId]
	s.Require().True(ok)
	s.Require().Equal(2*value.Uint64(), val.Uint64())

	// burn
	_, err = s.cli.ChangeTokenAmount(smartAccount, types.NewValueFromUint64(2*value.Uint64()), false)
	s.Require().NoError(err)
	tok, err = s.cli.GetTokens(smartAccount)
	s.Require().NoError(err)
	s.Require().Empty(tok)
}

type SuiteCliExec struct {
	SuiteCliBase
}

func (s *SuiteCliExec) TestCallCliBasic() {
	cfgPath := s.createConfigFile()

	block, err := s.DefaultClient.GetBlock(s.Context, types.BaseShardId, "latest", false)
	s.Require().NoError(err)

	res := s.RunCli("-c", cfgPath, "block", "--json", block.Number.String())
	s.Contains(res, block.Number.String())
	s.Contains(res, block.Hash.String())

	res = s.RunCli("-c", cfgPath, "block", "--json", "10000000000000")
	s.Equal("null", res)
}

func (s *SuiteCliExec) TestCliSmartAccount() {
	dir := s.T().TempDir()

	iniDataTmpl := `[nil]
rpc_endpoint = {{ .HttpUrl }}
faucet_endpoint = {{ .FaucetUrl }}
`
	iniData, err := common.ParseTemplate(iniDataTmpl, map[string]any{
		"HttpUrl":   s.endpoint,
		"FaucetUrl": s.faucetEndpoint,
	})
	s.Require().NoError(err)

	cfgPath := dir + "/config.ini"
	s.Require().NoError(os.WriteFile(cfgPath, []byte(iniData), 0o600))

	s.Run("Deploy new smart account", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "smart-account", "new")
		s.Require().Error(err)
		s.Contains(res, "Error: private_key not specified in config")
	})

	res := s.RunCli("-c", cfgPath, "keygen", "new")
	s.Run("Generate a key", func() {
		s.Contains(res, "Private key:")
	})

	s.Run("Address not specified", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "smart-account", "info")
		s.Require().Error(err)
		s.Contains(res, "Error: address not specified in config")
	})

	s.Run("Deploy new smart account", func() {
		res := s.RunCli("-c", cfgPath, "smart-account", "new")
		s.Contains(res, "New smart account address:")
	})

	s.Run("Check seqno", func() {
		res := s.RunCli("-c", cfgPath, "smart-account", "seqno")
		s.Contains(res, "Smart account seqno: 1")
	})

	var addr string
	s.Run("Get contract address", func() {
		addr = s.RunCli("-c", cfgPath, "contract", "address", s.incBinPath, "123321", "--abi", s.incAbiPath, "-q")
	})

	res = s.RunCli("-c", cfgPath, "smart-account", "deploy", s.incBinPath, "123321", "--abi", s.incAbiPath, "-q")
	parts := strings.Split(res, "\n")
	s.Require().Len(parts, 2)
	s.Run("Deploy contract", func() {
		s.Equal(addr, parts[1])
		receipt := s.WaitIncludedInMain(common.HexToHash(parts[0]))
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Check deploy transaction result and receipt", func() {
		res = s.RunCli("-c", cfgPath, "transaction", parts[0])
		s.Contains(res, "Transaction data:")
		s.Contains(res, "\"success\": true")

		res = s.RunCli("-c", cfgPath, "receipt", parts[0])
		s.Contains(res, "Receipt data:")
		s.Contains(res, "\"success\": true")
	})

	s.Run("Check seqno", func() {
		res := s.RunCli("-c", cfgPath, "smart-account", "seqno")
		s.Contains(res, "Smart account seqno: 2")

		res = s.RunCli("-c", cfgPath, "contract", "seqno", addr)
		s.Contains(res, "Contract seqno: 0")
	})

	s.Run("Check contract code", func() {
		res := s.RunCli("-c", cfgPath, "contract", "code", addr)
		s.Contains(res, "Contract code: 0x6080")
	})

	s.Run("Call read-only 'get' function of contract", func() {
		res := s.RunCli("-c", cfgPath, "contract", "call-readonly", addr, "get", "--abi", s.incAbiPath)
		s.Contains(res, "uint256: 123321")
	})

	s.Run("Call read-only 'get' function of contract", func() {
		res := s.RunCli("-c", cfgPath, "contract", addr)
		s.Contains(res, "Hash: ")
		s.Contains(res, "Address: "+addr)
		s.Contains(res, "Balance: 0")
		s.Contains(res, "Seqno: 2")
		s.Contains(res, "ExtSeqno: 0")
		s.Contains(res, "StorageRoot: ")
	})

	s.Run("Estimate fee", func() {
		isNum := func(str string) {
			s.T().Helper()
			_, err := strconv.ParseUint(str, 0, 64)
			s.Require().NoError(err)
		}

		isNums := func(str string) {
			s.T().Helper()
			lines := strings.Split(str, "\n")
			for _, line := range lines {
				isNum(line)
			}
		}

		resExt := s.RunCli("-c", cfgPath, "contract", "estimate-fee", addr, "increment", "--abi", s.incAbiPath, "-q")
		fmt.Println(resExt)
		isNums(resExt)

		resInt := s.RunCli(
			"-c", cfgPath, "contract", "estimate-fee", addr, "increment", "--abi", s.incAbiPath, "-q", "--internal")
		isNums(resInt)

		resSmartAccount := s.RunCli(
			"-c", cfgPath, "smart-account", "estimate-fee", addr, "increment", "--abi", s.incAbiPath, "-q")
		isNums(resSmartAccount)
	})

	s.Run("Call 'increment' function of contract", func() {
		res := s.RunCli(
			"-c", cfgPath, "smart-account", "send-transaction", addr, "increment", "--abi", s.incAbiPath, "-q")
		receipt := s.WaitIncludedInMain(common.HexToHash(res))
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Call read-only 'get' function of contract once again", func() {
		res := s.RunCli("-c", cfgPath, "contract", "call-readonly", addr, "get", "--abi", s.incAbiPath)
		s.Contains(res, "uint256: 123322")
	})

	overridesFile := dir + "/overrides.json"
	s.Run("Call read-only 'increment' via the smart account", func() {
		res := s.RunCli(
			"-c", cfgPath,
			"smart-account", "call-readonly", addr,
			"increment",
			"--abi", s.incAbiPath,
			"--out-overrides", overridesFile)
		s.Contains(res, "Success, no result")
	})

	s.Run("Check overrides file content", func() {
		res := make(map[string]any)
		data, err := os.ReadFile(overridesFile)
		s.Require().NoError(err)
		s.Require().NoError(json.Unmarshal(data, &res))
		s.Require().Len(res, 2)
		s.Contains(res, addr)
	})

	s.Run("Call read-only 'get' via the smart account", func() {
		res := s.RunCli(
			"-c", cfgPath,
			"smart-account", "call-readonly", addr,
			"get",
			"--abi", s.incAbiPath,
			"--in-overrides", overridesFile)
		s.Contains(res, "uint256: 123323")
	})
}

func (s *SuiteCliExec) TestCliToken() {
	cfgPath := s.createConfigFile()

	s.Run("Deploy new smart account", func() {
		s.RunCliCfg("keygen", "new")
		res := s.RunCliCfg("smart-account", "new")
		s.Contains(res, "New smart account address:")
	})

	var addr types.Address
	s.Run("Get address", func() {
		res := s.RunCliCfg("smart-account", "info", "-q")
		s.Require().NoError(addr.Set(strings.Split(res, "\n")[0]))
	})

	s.Run("Top-up smart account BTC", func() {
		res := s.RunCliCfg("smart-account", "top-up", "10000")
		s.Contains(res, "Smart Account balance:")
		s.Contains(res, "[NIL]")

		res = s.RunCliCfg("smart-account", "top-up", "10000", "BTC")
		s.Contains(res, "Smart Account balance: 10000 [BTC]")

		res = s.RunCliCfg("contract", "tokens", addr.Hex())
		s.Contains(res, types.BtcFaucetAddress.Hex()+"\t10000\t[BTC]")

		s.RunCliCfg("smart-account", "top-up", "20000", types.BtcFaucetAddress.Hex())
		res = s.RunCliCfg("contract", "tokens", addr.Hex())
		s.Contains(res, types.BtcFaucetAddress.Hex()+"\t30000\t[BTC]")
	})

	s.Run("Top-up contract BTC", func() {
		res := s.RunCliCfg("contract", "top-up", addr.Hex(), "10000")
		s.Contains(res, "Contract balance:")
		s.Contains(res, "[NIL]")

		res = s.RunCliCfg("contract", "top-up", addr.Hex(), "10000", "BTC")
		s.Contains(res, "Contract balance: 40000 [BTC]")

		res = s.RunCliCfg("contract", "tokens", addr.Hex())
		s.Contains(res, types.BtcFaucetAddress.Hex()+"\t40000\t[BTC]")

		s.RunCliCfg("contract", "top-up", addr.Hex(), "20000", types.BtcFaucetAddress.Hex())
		res = s.RunCliCfg("contract", "tokens", addr.Hex())
		s.Contains(res, types.BtcFaucetAddress.Hex()+"\t60000\t[BTC]")
	})

	s.Run("Top-up smart account unknown token", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "smart-account", "top-up", "123", "Unknown")
		s.Require().Error(err)
		s.Contains(res, "Error: undefined token id: Unknown")
	})

	s.Run("Top-up contract token", func() {
		res, err := s.RunCliNoCheck("-c", cfgPath, "contract", "top-up", addr.Hex(), "123", "Unknown")
		s.Require().Error(err)
		s.Contains(res, "Error: undefined token id: Unknown")
	})
}

func (s *SuiteCliExec) TestCliCometa() {
	cfg := &cometa.Config{
		UseBadger:   true,
		DbPath:      s.T().TempDir() + "/cometa.db",
		OwnEndpoint: s.cometaEndpoint,
	}
	com, err := cometa.NewService(s.Context, cfg, s.DefaultClient)
	s.Require().NoError(err)
	go func() {
		check.PanicIfErr(com.Run(s.Context, cfg))
	}()
	s.createConfigFile()
	abiFile := "../../contracts/compiled/tests/Counter.abi"

	var address types.Address
	var txnHash string

	s.Run("Deploy counter", func() {
		out := s.RunCliCfg(
			"smart-account", "deploy", "--compile-input", "../contracts/counter-compile.json", "--shard-id", "1", "-q")
		parts := strings.Split(out, "\n")
		s.Require().Len(parts, 2)
		address = types.HexToAddress(parts[1])
		txHash := common.HexToHash(parts[0])
		receipt := s.WaitIncludedInMain(txHash)
		s.Require().True(receipt.AllSuccess())
	})

	s.Run("Get metadata", func() {
		out := s.RunCliCfg("cometa", "info", "--address", address.Hex())
		s.Contains(out, "Name: Counter.sol:Counter")
	})

	s.Run("Call Counter.get()", func() {
		out := s.RunCliCfg(
			"smart-account", "send-transaction", address.Hex(),
			"--abi", abiFile,
			"--fee-credit", "500000000000000",
			"get")
		parts := strings.Split(out, ": ")
		s.Require().Len(parts, 2)
		txnHash = parts[1]
	})

	s.Run("Debug", func() {
		out := s.RunCliCfg("debug", txnHash)
		result := parseCometaOutput(out)
		s.Require().Len(result, 3)
		s.Require().Equal("2bb1ae7c00", result[0]["CallData"][:10])
		s.Require().Contains(result[0]["Transaction"], txnHash)
		s.Require().Equal("Counter", result[1]["Contract"])
		s.Require().Equal("get()", result[1]["CallData"])
		s.Require().Equal("Counter", result[1]["Contract"])
		s.Contains(out, "└ eventValue: [0]")
	})

	s.Run("Fetch abi from cometa for call-readonly", func() {
		out := s.RunCliCfg("contract", "call-readonly", address.Hex(), "get")
		s.CheckResult(out, "Success, result:", "int32: 0")
	})

	s.Run("Deploy smart account to test ctor arguments", func() {
		out := s.RunCliCfg("smart-account", "deploy",
			"--compile-input", "../../contracts/solidity/compile-smart-account.json",
			"--abi", "../../contracts/compiled/SmartAccount.abi",
			"--shard-id", "1",
			"0x12345678")
		parts := strings.Split(out, "\n")
		s.Require().Len(parts, 2)
		parts = strings.Split(parts[1], ": ")
		s.Require().Len(parts, 2)
		address = types.HexToAddress(parts[1])
	})

	s.Run("Get smart account metadata", func() {
		out := s.RunCliCfg("cometa", "info", "--address", address.Hex())
		s.Contains(out, "Name: SmartAccount.sol:SmartAccount")
	})

	s.Run("Register metadata for main smart account", func() {
		out := s.RunCliCfg("cometa", "register",
			"--address", types.MainSmartAccountAddress.Hex(),
			"--compile-input", "../../contracts/solidity/compile-smart-account.json")
		s.Require().Equal(
			"Contract metadata for address 0x0001111111111111111111111111111111111111 has been registered", out)
	})
}

func parseCometaOutput(out string) []map[string]string {
	res := make([]map[string]string, 0)
	var currTxn map[string]string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		parts := strings.Split(line, ": ")
		if strings.Contains(parts[0], "Transaction") {
			currTxn = make(map[string]string, 0)
			res = append(res, currTxn)
			currTxn["Transaction"] = strings.TrimSpace(parts[1])
		} else {
			currTxn[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return res
}

func (s *SuiteCliBase) createConfigFile() string {
	s.T().Helper()

	cfgPath := s.TmpDir + "/config.ini"

	iniData := "[nil]\nrpc_endpoint = " + s.endpoint + "\n"
	iniData += "cometa_endpoint = " + s.cometaEndpoint + "\n"
	iniData += "faucet_endpoint = " + s.faucetEndpoint + "\n"
	iniData += "private_key = " + nilcrypto.PrivateKeyToEthereumFormat(execution.MainPrivateKey) + "\n"
	iniData += "address = 0x0001111111111111111111111111111111111111\n"
	err := os.WriteFile(cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)
	return cfgPath
}

func (s *SuiteCliBase) RunCliCfg(args ...string) string {
	s.T().Helper()
	args = append([]string{"-c", s.TmpDir + "/config.ini"}, args...)
	return s.RunCli(args...)
}

func TestSuiteCliService(t *testing.T) {
	t.Parallel()

	s := new(SuiteCliService)
	s.basePort = 10325
	suite.Run(t, s)
}

func TestSuiteCliExec(t *testing.T) {
	t.Parallel()

	s := new(SuiteCliExec)
	s.basePort = 10335
	suite.Run(t, s)
}
