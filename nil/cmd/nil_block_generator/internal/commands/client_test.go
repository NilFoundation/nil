package commands

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/stretchr/testify/suite"
)

type NilBlockGeneratorTestSuite struct {
	tests.RpcSuite
	contractCodePath string
	contractBasePath string
	contractName     string
	method           string
	deployArgs       []string
	callArgs         []string
	url              string
	logger           logging.Logger
}

func NewNilBlockGeneratorTestSuite(
	contractCodePath string,
	contractName string,
	method string,
	deployArgs string,
	callArgs string,
) *NilBlockGeneratorTestSuite {
	return &NilBlockGeneratorTestSuite{
		contractCodePath: contractCodePath,
		contractName:     contractName,
		method:           method,
		deployArgs:       strings.Fields(deployArgs),
		callArgs:         strings.Fields(callArgs),
	}
}

func CompileContract(path, name, basePath string) error {
	contracts, err := solc.CompileSource(path)
	if err != nil {
		return err
	}

	// write bytecode to file
	bytecodePath := basePath + ".bin"
	err = os.WriteFile(bytecodePath, []byte(contracts[name].Code), fileMode)
	if err != nil {
		return err
	}

	// write ABI to file
	data, err := json.Marshal(contracts[name].Info.AbiDefinition)
	if err != nil {
		return err
	}
	abiPath := basePath + ".abi"
	err = os.WriteFile(abiPath, data, fileMode)
	if err != nil {
		return err
	}

	return nil
}

func (s *NilBlockGeneratorTestSuite) SetupTest() {
	s.TmpDir = s.T().TempDir()

	s.logger = logging.NewLogger("test_nil_block_generator")

	// prepare compiled contract
	s.contractBasePath = s.TmpDir + "/TestContract"
	err := CompileContract(s.contractCodePath, s.contractName, s.contractBasePath)
	s.Require().NoError(err)

	s.url = rpc.GetSockPath(s.T())

	nilserviceCfg := &nilservice.Config{
		NShards:              2,
		HttpUrl:              s.url,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
	}

	s.Start(nilserviceCfg)
}

func (s *NilBlockGeneratorTestSuite) TearDownTest() {
	s.Cancel()
}

func (s *NilBlockGeneratorTestSuite) TestGetBlock() {
	smartAccountAdr, hexKey, err := CreateNewSmartAccount(s.url, s.logger)
	s.Require().NoError(err)
	s.Require().NotEqual("", smartAccountAdr)
	s.Require().NotEqual("", hexKey)

	contractAddress, err := DeployContract(s.url, smartAccountAdr, s.contractBasePath, hexKey, s.deployArgs, s.logger)
	s.Require().NoError(err)
	s.Require().NotEqual("", contractAddress)

	var calls []Call
	calls = append(calls, *NewCall(s.contractName, s.method, s.contractBasePath+".abi", contractAddress, s.callArgs, 1))
	blockHash, err := CallContract(s.url, smartAccountAdr, hexKey, calls, s.logger)
	s.Require().NoError(err)
	s.Require().NotEqual("", blockHash)
}

func TestNilBlockGeneratorTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, NewNilBlockGeneratorTestSuite(
		"../../../../tests/contracts/increment.sol", "Incrementer", "increment", "0", ""))
}
