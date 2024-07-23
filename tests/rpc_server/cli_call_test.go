package rpctest

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	nilcrypto "github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type SuiteCliTestCall struct {
	// Inherit RPC suite because if we inherit SuiteRpc something goes wrong and a lot of tests fail.
	RpcSuite
	zerostateCfg string
	testAddress  types.Address
	cfgPath      string
	tmpPath      string
	port         int
}

func (s *SuiteCliTestCall) SetupSuite() {
	var err error

	s.port = 8543
	s.shardsNum = 2
	s.tmpPath = s.T().TempDir()
	s.cfgPath = s.tmpPath + "/config.ini"

	iniDataTmpl := `[nil]
rpc_endpoint = http://127.0.0.1:{{ .Port }}
private_key = {{ .PrivateKey }}
address = {{ .Address }}
`
	iniData, err := common.ParseTemplate(iniDataTmpl, map[string]interface{}{
		"Port":       s.port,
		"PrivateKey": nilcrypto.PrivateKeyToEthereumFormat(execution.MainPrivateKey),
		"Address":    types.MainWalletAddress.Hex(),
	})
	s.Require().NoError(err)

	err = os.WriteFile(s.cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)

	s.testAddress, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{1})
	s.Require().NoError(err)

	s.zerostateCfg = fmt.Sprintf(`
contracts:
- name: Test
  address: %s
  value: 100000000000000
  contract: tests/Test
`, s.testAddress.Hex())
}

func (s *SuiteCliTestCall) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              s.shardsNum,
		HttpPort:             s.port,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            s.zerostateCfg,
		CollatorTickPeriodMs: 100,
		GracefulShutdown:     false,
		GasPriceScale:        0,
		GasBasePrice:         10,
	}, false)
}

func (s *SuiteCliTestCall) TearDownTest() {
	s.cancel()
}

func (s *SuiteCliTestCall) testResult(res string, expectedLines ...string) {
	s.T().Helper()

	lines := strings.Split(strings.Trim(res, "\n"), "\n")
	s.Require().GreaterOrEqual(len(lines), len(expectedLines))

	for i, line := range expectedLines {
		s.Require().Equal(line, lines[i])
	}
}

func (s *SuiteCliTestCall) TestCliCall() {
	abiData, err := contracts.GetAbiData(contracts.NameTest)
	s.Require().NoError(err)
	abiFile := s.tmpPath + "/Test.abi"
	err = os.WriteFile(abiFile, []byte(abiData), 0o600)
	s.Require().NoError(err)

	var res string

	res = s.runCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "noReturn", "--abi", abiFile)
	s.testResult(res, "Success, no result")

	res = s.runCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getSum", "1", "2", "--abi", abiFile)
	s.testResult(res, "Success, result:", "uint256: 3")

	res = s.runCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getString", "--abi", abiFile)
	s.testResult(res, "Success, result:", "string: Very long string with many characters and words and spaces and numbers and symbols and everything else that can be in a string")

	res = s.runCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getNumAndString", "--abi", abiFile)
	s.testResult(res, "Success, result:", "uint256: 123456789012345678901234567890", "string: Simple string")

	res, err = s.runCliNoCheck("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "nonExistingMethod", "--abi", abiFile)
	s.Require().Error(err)
	s.testResult(res, "Error: failed to pack method call: method 'nonExistingMethod' not found")
}

func TestCLiCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteCliTestCall))
}
