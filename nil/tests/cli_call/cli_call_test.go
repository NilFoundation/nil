package tests

import (
	"os"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
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
	tests.ShardedSuite

	client      client.Client
	endpoint    string
	testAddress types.Address
	cfgPath     string
}

func (s *SuiteCliTestCall) SetupSuite() {
	var err error
	s.TmpDir = s.T().TempDir()
	s.cfgPath = s.TmpDir + "/config.ini"

	s.testAddress, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{1})
	s.Require().NoError(err)

	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{{
			Name:     "Test",
			Contract: "tests/Test",
			Address:  s.testAddress,
			Value:    types.NewValueFromUint64(100000000000000),
		}},
	}

	s.Start(&nilservice.Config{
		NShards:              2,
		CollatorTickPeriodMs: 200,
		ZeroState:            zeroState,
	}, 10525)

	s.client, s.endpoint = s.StartRPCNode(&tests.RpcNodeConfig{
		WithDhtBootstrapByValidators: true,
		ArchiveNodes:                 nil,
	})

	iniDataTmpl := `[nil]
rpc_endpoint = {{ .HttpUrl }}
private_key = {{ .PrivateKey }}
address = {{ .Address }}
`
	iniData, err := common.ParseTemplate(iniDataTmpl, map[string]any{
		"HttpUrl":    s.endpoint,
		"PrivateKey": nilcrypto.PrivateKeyToEthereumFormat(execution.MainPrivateKey),
		"Address":    types.MainSmartAccountAddress.Hex(),
	})
	s.Require().NoError(err)

	err = os.WriteFile(s.cfgPath, []byte(iniData), 0o600)
	s.Require().NoError(err)
}

func (s *SuiteCliTestCall) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteCliTestCall) TestCliCall() {
	abiData, err := contracts.GetAbiData(contracts.NameTest)
	s.Require().NoError(err)
	abiFile := s.TmpDir + "/Test.abi"
	err = os.WriteFile(abiFile, []byte(abiData), 0o600)
	s.Require().NoError(err)

	var res string

	res = s.RunCallReadonly("noReturn", "--abi", abiFile)
	s.CheckResult(res, "Success, no result")

	res = s.RunCallReadonly("noReturn", "--abi", abiFile, "--json")
	s.JSONEq(`{ "result": [] }`, res)

	res = s.RunCallReadonly("getSum", "1", "2", "--abi", abiFile)
	s.CheckResult(res, "Success, result:", "uint256: 3")

	res = s.RunCallReadonly("getSum", "1", "2", "--abi", abiFile, "--json")
	s.JSONEq(`{ "result": [ { "type": "uint256", "value": 3 } ] }`, res)

	res = s.RunCallReadonly("getString", "--abi", abiFile)
	s.CheckResult(
		res,
		"Success, result:",
		"string: \"Very long string with many characters and words and spaces and numbers and symbols and everything"+
			" else that can be in a string\"")

	res = s.RunCallReadonly("getNumAndString", "--abi", abiFile)
	s.CheckResult(res, "Success, result:", "uint256: 123456789012345678901234567890", "string: \"Simple string\"")

	res, err = s.RunCliNoCheck(
		"-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(), "nonExistingMethod", "--abi", abiFile)
	s.Require().Error(err)
	s.CheckResult(res, "Error: failed to pack method call: method 'nonExistingMethod' not found")

	overridesFile := s.TmpDir + "/overrides.json"
	res = s.RunCallReadonly("getValue", "--abi", abiFile)
	s.CheckResult(res, "Success, result:", "uint32: 0")

	res = s.RunCallReadonly("setValue", "321", "--abi", abiFile, "--out-overrides", overridesFile, "--with-details")
	s.CheckResult(
		res, "Success, no result", "Logs:", "Event: stubCalled", "uint32: 321", "Debug logs:", "Value set to [321]")

	res = s.RunCallReadonly("setValue", "123", "--abi", abiFile, "--out-overrides", overridesFile)
	s.CheckResult(res, "Success, no result")

	res = s.RunCallReadonly("getValue", "--abi", abiFile, "--in-overrides", overridesFile)
	s.CheckResult(res, "Success, result:", "uint32: 123")
}

func (s *SuiteCliTestCall) RunCallReadonly(args ...string) string {
	return s.RunCli(append([]string{
		"-c", s.cfgPath, "contract", "call-readonly", s.testAddress.Hex(),
	}, args...)...)
}

func TestCliCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteCliTestCall))
}
