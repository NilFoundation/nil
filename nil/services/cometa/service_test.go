package cometa

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteServiceTest struct {
	suite.Suite

	service *Service
	client  client.ClientMock
	ctx     context.Context
}

func (s *SuiteServiceTest) SetupSuite() {
	s.ctx = context.Background()
	var err error
	cfg := &Config{
		UseBadger: true,
		DbPath:    s.T().TempDir() + "/cometa.db",
	}
	s.service, err = NewService(s.ctx, cfg, &s.client)
	s.Require().NoError(err)
}

func (s *SuiteServiceTest) TestCase1() {
	task := s.getCompilerTask("input_1")

	contractData, err := Compile(task)
	s.Require().NoError(err)

	code := contractData.Code
	address := types.CreateAddress(types.ShardId(1), types.BuildDeployPayload(code, common.EmptyHash))

	s.client.GetCodeFunc = func(ctx context.Context, addr types.Address, blockId any) (types.Code, error) {
		s.Require().Equal(address, addr)
		return code, nil
	}

	err = s.service.RegisterContractData(s.ctx, contractData, address)
	s.Require().NoError(err)

	contract, err := s.service.GetContractControl(s.ctx, address)
	s.Require().NoError(err)

	s.Require().Equal(contractData, contract.Data)

	loc, err := s.service.GetLocationRaw(s.ctx, address, 0)
	s.Require().NoError(err)
	s.Require().NotNil(loc)
	s.Require().Equal("Test.sol:88", loc.String())

	jsonContract, err := s.service.GetContractAsJson(s.ctx, address)
	s.Require().NoError(err)
	s.Require().NotEmpty(jsonContract)

	source, err := s.service.GetSourceCodeForFile(s.ctx, address, loc.FileName)
	s.Require().NoError(err)
	s.Require().Equal("contract", source[loc.Position:loc.Position+8])
}

func (s *SuiteServiceTest) TestCase2() {
	task := s.getCompilerTask("input_2")

	contractData, err := Compile(task)
	s.Require().NoError(err)

	code := contractData.Code
	address := types.CreateAddress(types.ShardId(1), types.BuildDeployPayload(code, common.EmptyHash))

	s.client.GetCodeFunc = func(ctx context.Context, addr types.Address, blockId any) (types.Code, error) {
		s.Require().Equal(address, addr)
		return code, nil
	}

	err = s.service.RegisterContractData(s.ctx, contractData, address)
	s.Require().NoError(err)

	contract, err := s.service.GetContractControl(s.ctx, address)
	s.Require().NoError(err)

	s.Require().Equal(contractData, contract.Data)

	loc, err := s.service.GetLocation(s.ctx, address, 0)
	s.Require().NoError(err)
	s.Require().NotNil(loc)
	s.Require().Equal("Test.sol:3, function: #function_selector", loc.String())

	jsonContract, err := s.service.GetContractAsJson(s.ctx, address)
	s.Require().NoError(err)
	s.Require().NotEmpty(jsonContract)
}

// TestTwinContracts checks that the same contract is returned for the same code
func (s *SuiteServiceTest) TestTwinContracts() {
	task := s.getCompilerTask("input_1")

	contractData, err := Compile(task)
	s.Require().NoError(err)

	code := contractData.Code
	address := types.CreateAddress(types.ShardId(1), types.BuildDeployPayload(code, common.HexToHash("0x5678")))

	s.client.GetCodeFunc = func(ctx context.Context, addr types.Address, blockId any) (types.Code, error) {
		return code, nil
	}

	err = s.service.RegisterContractData(s.ctx, contractData, address)
	s.Require().NoError(err)

	contract, err := s.service.GetContractControl(s.ctx, address)
	s.Require().NoError(err)

	s.Require().Equal(contractData, contract.Data)

	address2 := types.CreateAddress(types.ShardId(1), types.BuildDeployPayload(code, common.HexToHash("0x1234")))
	contract2, err := s.service.GetContractControl(s.ctx, address2)
	s.Require().NoError(err)

	s.Require().Equal(contract, contract2)
}

func (s *SuiteServiceTest) getCompilerTask(name string) *CompilerTask {
	s.T().Helper()

	input, err := os.ReadFile(fmt.Sprintf("./tests/%s.json", name))
	s.Require().NoError(err)

	task, err := NewCompilerTask(string(input))
	s.Require().NoError(err)
	err = task.Normalize("./tests")
	s.Require().NoError(err)

	return task
}

func TestCometa(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteServiceTest))
}
