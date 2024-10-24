package tests

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	nilcrypto "github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteCliTestCall struct {
	// Inherit RPC suite because if we inherit SuiteRpc something goes wrong and a lot of tests fail.
	tests.RpcSuite
	zerostateCfg string
	testAddress  types.Address
	cfgPath      string
	tmpPath      string
}

func (s *SuiteCliTestCall) SetupSuite() {
	var err error

	s.ShardsNum = 2
	s.tmpPath = s.T().TempDir()
	s.cfgPath = s.tmpPath + "/config.ini"

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
	s.StartWithRPC(&nilservice.Config{
		NShards:       s.ShardsNum,
		ZeroStateYaml: s.zerostateCfg,
	}, 11005, false)

	iniDataTmpl := `[nil]
rpc_endpoint = {{ .HttpUrl }}
private_key = {{ .PrivateKey }}
address = {{ .Address }}
`

	iniData, err := common.ParseTemplate(iniDataTmpl, map[string]interface{}{
		"HttpUrl":    s.Endpoint,
		"PrivateKey": nilcrypto.PrivateKeyToEthereumFormat(execution.MainPrivateKey),
		"Address":    types.MainWalletAddress.Hex(),
	})
	s.Require().NoError(err)

	err = os.WriteFile(s.cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)
}

func (s *SuiteCliTestCall) TearDownTest() {
	s.Cancel()
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

	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "noReturn", "--abi", abiFile)
	s.testResult(res, "Success, no result")

	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getSum", "1", "2", "--abi", abiFile)
	s.testResult(res, "Success, result:", "uint256: 3")

	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getString", "--abi", abiFile)
	s.testResult(res, "Success, result:", "string: Very long string with many characters and words and spaces and numbers and symbols and everything else that can be in a string")

	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getNumAndString", "--abi", abiFile)
	s.testResult(res, "Success, result:", "uint256: 123456789012345678901234567890", "string: Simple string")

	res, err = s.RunCliNoCheck("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "nonExistingMethod", "--abi", abiFile)
	s.Require().Error(err)
	s.testResult(res, "Error: failed to pack method call: method 'nonExistingMethod' not found")

	overridesFile := s.tmpPath + "/overrides.json"
	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getValue", "--abi", abiFile)
	s.testResult(res, "Success, result:", "uint32: 0")

	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "setValue", "123", "--abi", abiFile, "--out-overrides", overridesFile)
	s.testResult(res, "Success, no result")

	res = s.RunCli("-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "getValue", "--abi", abiFile, "--in-overrides", overridesFile)
	s.testResult(res, "Success, result:", "uint32: 123")
}

func TestCliCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteCliTestCall))
}
