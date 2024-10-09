package cometa

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
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
	s.service, err = NewService(s.T().TempDir(), &s.client)
	s.Require().NoError(err)
}

func (s *SuiteServiceTest) TestCase1() {
	input := getInputJson(s.T(), "input_1")

	compData, err := Compile(input)
	s.Require().NoError(err)

	code := compData.Code
	address := types.CreateAddress(types.ShardId(1), types.BuildDeployPayload(code, common.EmptyHash))

	s.client.GetCodeFunc = func(addr types.Address, blockId any) (types.Code, error) {
		s.Require().Equal(address, addr)
		return code, nil
	}

	contractData, err := s.service.CompileContract(input)
	s.Require().NoError(err)

	err = s.service.RegisterContract(s.ctx, contractData, address)
	s.Require().NoError(err)

	contract, err := s.service.GetContract(s.ctx, address)
	s.Require().NoError(err)

	s.Require().Equal(compData, contract.Data)

	loc, err := s.service.GetLocation(s.ctx, address, 0)
	s.Require().NoError(err)
	s.Require().NotNil(loc)
	s.Require().Equal("Test.sol:88", loc.String())

	jsonContract, err := s.service.GetContractAsJson(s.ctx, address)
	s.Require().NoError(err)
	s.Require().NotEmpty(jsonContract)

	source, err := s.service.GetSourceCode(s.ctx, address, loc.FileName)
	s.Require().NoError(err)
	s.Require().Equal("contract", source[loc.Position:loc.Position+8])
}

func (s *SuiteServiceTest) TestCase2() {
	input := getInputJson(s.T(), "input_2")

	compData, err := Compile(input)
	s.Require().NoError(err)

	code := compData.Code
	address := types.CreateAddress(types.ShardId(1), types.BuildDeployPayload(code, common.EmptyHash))

	s.client.GetCodeFunc = func(addr types.Address, blockId any) (types.Code, error) {
		s.Require().Equal(address, addr)
		return code, nil
	}

	contractData, err := s.service.CompileContract(input)
	s.Require().NoError(err)

	err = s.service.RegisterContract(s.ctx, contractData, address)
	s.Require().NoError(err)

	contract, err := s.service.GetContract(s.ctx, address)
	s.Require().NoError(err)

	s.Require().Equal(compData, contract.Data)

	loc, err := s.service.GetLocation(s.ctx, address, 0)
	s.Require().NoError(err)
	s.Require().NotNil(loc)
	s.Require().Equal("Test.sol:59", loc.String())

	jsonContract, err := s.service.GetContractAsJson(s.ctx, address)
	s.Require().NoError(err)
	s.Require().NotEmpty(jsonContract)
}

func getInputJson(t *testing.T, name string) string {
	t.Helper()

	input, err := os.ReadFile(fmt.Sprintf("./tests/%s.json", name))
	require.NoError(t, err)

	return string(input)
}

func TestCometa(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteServiceTest))
}
