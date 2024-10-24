package tests

import (
	"math"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteRpcNode struct {
	tests.RpcSuite
}

func (s *SuiteRpcNode) SetupTest() {
	s.StartWithRPC(&nilservice.Config{
		NShards: 5,
		RunMode: nilservice.NormalRunMode,
	}, 11001, false)
}

func (s *SuiteRpcNode) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRpcNode) TestGetDebugBlock() {
	debugBlock, err := s.Client.GetDebugBlock(types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.NotNil(debugBlock)

	debugBlock, err = s.Client.GetDebugBlock(types.BaseShardId, 0x1, true)
	s.Require().NoError(err)
	s.NotNil(debugBlock)
}

func (s *SuiteRpcNode) TestGetBlock() {
	block, err := s.Client.GetBlock(types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.NotNil(block)

	block, err = s.Client.GetBlock(types.BaseShardId, 0x1, true)
	s.Require().NoError(err)
	s.NotNil(block)

	block, err = s.Client.GetBlock(types.MainShardId, 0x1, true)
	s.Require().NoError(err)
	s.Require().NotNil(block)
	s.NotEmpty(block.ChildBlocks)
	s.Positive(block.DbTimestamp)
}

func (s *SuiteRpcNode) TestGetBlockTransactionCount() {
	count, err := s.Client.GetBlockTransactionCount(types.BaseShardId, "latest")
	s.Require().NoError(err)
	s.Zero(count)

	count, err = s.Client.GetBlockTransactionCount(types.BaseShardId, 0x1)
	s.Require().NoError(err)
	s.Zero(count)

	count, err = s.Client.GetBlockTransactionCount(types.MainShardId, 0x1)
	s.Require().NoError(err)
	s.Zero(count)

	count, err = s.Client.GetBlockTransactionCount(types.MainShardId, math.MaxUint32)
	s.Require().NoError(err)
	s.Zero(count)
}

func (s *SuiteRpcNode) TestGetBalance() {
	balance, err := s.Client.GetBalance(types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotNil(balance)

	balance, err = s.Client.GetBalance(types.FaucetAddress, 0x1)
	s.Require().NoError(err)
	s.NotNil(balance)
}

func (s *SuiteRpcNode) TestGetCode() {
	code, err := s.Client.GetCode(types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotNil(code)

	code, err = s.Client.GetCode(types.FaucetAddress, 0x1)
	s.Require().NoError(err)
	s.NotNil(code)
}

func (s *SuiteRpcNode) TestGetCurrencies() {
	currencies, err := s.Client.GetCurrencies(types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotNil(currencies)

	currencies, err = s.Client.GetCurrencies(types.FaucetAddress, 0x1)
	s.Require().NoError(err)
	s.NotNil(currencies)
}

func (s *SuiteRpcNode) TestGasPrice() {
	value, err := s.Client.GasPrice(types.MainShardId)
	s.Require().NoError(err)
	s.Positive(value.Uint64())

	value, err = s.Client.GasPrice(types.BaseShardId)
	s.Require().NoError(err)
	s.Positive(value.Uint64())
}

func TestSuiteRpcNode(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpcNode))
}
