package cli_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SuiteCliNoServer struct {
	tests.CliRunner

	incAbiPath string
}

func (s *SuiteCliNoServer) SetupSuite() {
	s.TmpDir = s.T().TempDir()

	s.incAbiPath = s.TmpDir + "/Incrementer.abi"
	compileIncrementerAndSaveToFile(s.T(), "", s.incAbiPath)
}

func compileIncrementerAndSaveToFile(t *testing.T, binFileName string, abiFileName string) {
	t.Helper()

	contractData, err := solc.CompileSource(common.GetAbsolutePath("../contracts/increment.sol"))
	require.NoError(t, err)

	if len(binFileName) > 0 {
		err = os.WriteFile(binFileName, []byte(contractData["Incrementer"].Code), 0o600)
		require.NoError(t, err)
	}

	if len(abiFileName) > 0 {
		abiData, err := json.Marshal(contractData["Incrementer"].Info.AbiDefinition)
		require.NoError(t, err)
		err = os.WriteFile(abiFileName, abiData, 0o600)
		require.NoError(t, err)
	}
}

func (s *SuiteCliNoServer) TestCallCliHelp() {
	res := s.RunCli("help")

	for _, cmd := range []string{"block", "transaction", "contract", "smart-account", "completion"} {
		s.Contains(res, cmd)
	}
}

func (s *SuiteCliNoServer) TestCliP2pKeygen() {
	res := s.RunCli("keygen", "new-p2p", "-q")
	lines := strings.Split(res, "\n")
	s.Len(lines, 3)
}

func (s *SuiteCliNoServer) TestCliAbi() {
	s.Run("Encode", func() {
		res := s.RunCli("abi", "encode", "get", "--path", s.incAbiPath)
		s.Equal("0x6d4ce63c", res)
	})

	s.Run("Decode", func() {
		res := s.RunCli(
			"abi",
			"decode",
			"get",
			"0x000000000000000000000000000000000000000000000000000000000001e1ba",
			"--path",
			s.incAbiPath)
		s.Equal("uint256: 123322", res)
	})
}

func (s *SuiteCliNoServer) TestCliEncodeInternalTransaction() {
	calldata := s.RunCli("abi", "encode", "get", "--path", s.incAbiPath)
	s.Equal("0x6d4ce63c", calldata)

	addr := "0x00041945255839dcbd3001fd5e6abe9ee970a797"
	res := s.RunCli("transaction", "encode-internal", "--to", addr, "--data", calldata, "--fee-credit", "5000000")

	expected := "0x0000404b4c0000000000000000000000000000000000000000000000000000000000030000000000000000041945255839dcbd3001fd5e6abe9ee970a797000000000000000000000000000000000000000000000000000000000000000000000000000000009a00000000000000000000000000000000000000000000000000000000000000000000009a00000000000000000000009e0000006d4ce63c" //nolint:lll
	s.Contains(res, "\"feeCredit\": \"5000000\"")
	s.Contains(res, "\"forwardKind\": 3")
	s.Contains(res, "Result: "+expected)

	res = s.RunCli("transaction", "encode-internal", "--to", addr, "--data", calldata, "--fee-credit", "5000000", "-q")
	s.Contains(expected, res)
}

func (s *SuiteCliNoServer) TestCliConfig() {
	cfgPath := s.TmpDir + "/config.ini"
	endpoint := "localhost:10325"

	s.Run("Create config", func() {
		res := s.RunCli("-c", cfgPath, "config", "init")
		s.Contains(res, "The config file has been initialized successfully: "+cfgPath)
	})

	s.Run("Set config value", func() {
		res := s.RunCli("-c", cfgPath, "config", "set", "rpc_endpoint", endpoint)
		s.Contains(res, fmt.Sprintf("Set \"rpc_endpoint\" to %q", endpoint))
	})

	s.Run("Read config value", func() {
		res := s.RunCli("-c", cfgPath, "config", "get", "rpc_endpoint")
		s.Contains(res, "rpc_endpoint: "+endpoint)
	})

	s.Run("Show config", func() {
		res := s.RunCli("-c", cfgPath, "config", "show")
		s.Contains(res, "rpc_endpoint      : "+endpoint)
	})
}

func TestSuiteCliNoServer(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteCliNoServer))
}
