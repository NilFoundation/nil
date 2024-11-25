package cometa

import (
	"os/exec"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"
)

type SuiteCometa struct {
	tests.RpcSuite
	cometaClient cometa.Client
	cometaCfg    cometa.Config
	zerostateCfg string
	testAddress  types.Address
}

type SuiteCometaBadger struct {
	SuiteCometa
}

type SuiteCometaClickhouse struct {
	SuiteCometa
	clickhouse *exec.Cmd
}

func (s *SuiteCometa) SetupSuite() {
	s.cometaCfg.DbPath = s.T().TempDir() + "/cometa.db"
	s.cometaCfg.OwnEndpoint = ""
	var err error

	s.testAddress, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{1})
	s.Require().NoError(err)

	zerostateTmpl := `
contracts:
- name: MainWallet
  address: {{ .WalletAddress }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: Test
  address: {{ .TestAddress }}
  value: 100000000
  contract: tests/Test
`
	s.zerostateCfg, err = common.ParseTemplate(zerostateTmpl, map[string]any{
		"WalletAddress": types.MainWalletAddress.Hex(),
		"MainPublicKey": hexutil.Encode(execution.MainPublicKey),
		"TestAddress":   s.testAddress.Hex(),
	})
	s.Require().NoError(err)
}

func (s *SuiteCometaClickhouse) SetupSuite() {
	s.cometaCfg.UseBadger = false

	s.cometaCfg.ResetToDefault()

	suiteSetupDone := false

	defer func() {
		if !suiteSetupDone {
			s.TearDownSuite()
		}
	}()

	s.clickhouse = exec.Command("clickhouse", "server", "--", "--listen_host=0.0.0.0")
	s.clickhouse.Dir = s.T().TempDir()
	err := s.clickhouse.Start()
	s.Require().NoError(err)

	time.Sleep(1 * time.Second)
	createDb := exec.Command("clickhouse-client", "--query", "CREATE DATABASE IF NOT EXISTS "+s.cometaCfg.DbName) //nolint:gosec
	out, err := createDb.CombinedOutput()
	s.Require().NoErrorf(err, "output: %s", out)

	s.SuiteCometa.SetupSuite()

	suiteSetupDone = true
}

func (s *SuiteCometaClickhouse) TearDownSuite() {
	if s.clickhouse != nil {
		err := s.clickhouse.Process.Kill()
		s.Require().NoError(err)
	}
}

func (s *SuiteCometaBadger) SetupSuite() {
	s.cometaCfg.ResetToDefault()
	s.cometaCfg.UseBadger = true
	s.SuiteCometa.SetupSuite()
}

func (s *SuiteCometa) SetupTest() {
	s.cometaCfg.DbPath = s.T().TempDir() + "/cometa.db"
	s.Start(&nilservice.Config{
		NShards:              2,
		CollatorTickPeriodMs: 200,
		HttpUrl:              rpc.GetSockPath(s.T()),
		Cometa:               &s.cometaCfg,
		ZeroStateYaml:        s.zerostateCfg,
	})
	s.cometaClient = *cometa.NewClient(s.Endpoint)
}

func (s *SuiteCometa) TestTwinContracts() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)
	pub := crypto.CompressPubkey(&pk.PublicKey)
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
	deployCode1 := types.BuildDeployPayload(walletCode, common.EmptyHash)
	deployCode2 := types.BuildDeployPayload(walletCode, common.HexToHash("0x1234"))

	walletAddr1, _ := s.DeployContractViaMainWallet(types.BaseShardId, deployCode1, types.NewValueFromUint64(10_000_000))
	walletAddr2, _ := s.DeployContractViaMainWallet(types.BaseShardId, deployCode2, types.NewValueFromUint64(10_000_000))

	err = s.cometaClient.RegisterContractFromFile("../../contracts/solidity/compile-wallet.json", walletAddr1)
	s.Require().NoError(err)

	contract1, err := s.cometaClient.GetContract(walletAddr1)
	s.Require().NoError(err)

	contract2, err := s.cometaClient.GetContract(walletAddr2)
	s.Require().NoError(err)

	s.Require().Equal(contract1, contract2)
}

func (s *SuiteCometa) TestGeneratedCode() {
	if !s.cometaCfg.UseBadger {
		s.T().Skip()
	}
	var (
		receipt *jsonrpc.RPCReceipt
		data    []byte
		loc     *cometa.Location
	)
	testAbi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	contractData, err := s.cometaClient.CompileContract("../../contracts/solidity/tests/compile-test.json")
	s.Require().NoError(err)
	deployCode := types.BuildDeployPayload(contractData.InitCode, common.EmptyHash)
	testAddress, _ := s.DeployContractViaMainWallet(types.BaseShardId, deployCode, types.NewValueFromUint64(10_000_000))

	err = s.cometaClient.RegisterContractData(contractData, testAddress)
	s.Require().NoError(err)

	data = []byte("invalid calldata")
	receipt = s.SendExternalMessageNoCheck(data, testAddress)
	s.Require().False(receipt.AllSuccess())

	loc, err = s.cometaClient.GetLocation(testAddress, uint64(receipt.FailedPc))
	s.Require().NoError(err)
	s.Require().Equal("Test.sol:7, function: #function_selector", loc.String())

	data = s.AbiPack(testAbi, "makeFail", int32(1))
	receipt = s.SendExternalMessageNoCheck(data, testAddress)
	s.Require().False(receipt.AllSuccess())

	loc, err = s.cometaClient.GetLocation(testAddress, uint64(receipt.FailedPc))
	s.Require().NoError(err)
	s.Require().Equal("#utility.yul:8, function: revert_error_dbdddcbe895c83990c08b3492a0e83918d802a52331272ac6fdb6a7c4aea3b1b", loc.String())
}

func checkClickhouseInstalled() bool {
	cmd := exec.Command("clickhouse", "--version")
	err := cmd.Run()
	return err == nil
}

func TestCometaClickhouse(t *testing.T) {
	if !checkClickhouseInstalled() {
		if assert.Enable {
			t.Fatal("Clickhouse is not installed")
		} else {
			t.Skip("Clickhouse is not installed")
		}
	}
	t.Parallel()
	suite.Run(t, new(SuiteCometaClickhouse))
}

func TestCometaBadger(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SuiteCometaBadger))
}
