package tests

import (
	"math"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteRpcNode struct {
	tests.ShardedSuite
}

func (s *SuiteRpcNode) SetupTest() {
	port := 11001
	nShards := 5
	s.Start(&nilservice.Config{
		NShards: uint32(nShards),
		RunMode: nilservice.NormalRunMode,
	}, port)

	_, archiveNodeAddr := s.StartArchiveNode(port+nShards, true)
	s.DefaultClient, _ = s.StartRPCNode(tests.WithoutDhtBootstrapByValidators, network.AddrInfoSlice{archiveNodeAddr})
}

func (s *SuiteRpcNode) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRpcNode) TestGetDebugBlock() {
	debugBlock, err := s.DefaultClient.GetDebugBlock(s.Context, types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.NotNil(debugBlock)

	debugBlock, err = s.DefaultClient.GetDebugBlock(s.Context, types.BaseShardId, 0x1, true)
	s.Require().NoError(err)
	s.NotNil(debugBlock)
}

func (s *SuiteRpcNode) TestGetBlock() {
	block, err := s.DefaultClient.GetBlock(s.Context, types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.NotNil(block)

	block, err = s.DefaultClient.GetBlock(s.Context, types.BaseShardId, 0x1, true)
	s.Require().NoError(err)
	s.NotNil(block)

	block, err = s.DefaultClient.GetBlock(s.Context, types.MainShardId, 0x1, true)
	s.Require().NoError(err)
	s.Require().NotNil(block)
	s.NotEmpty(block.ChildBlocks)
	s.Positive(block.DbTimestamp)
}

func (s *SuiteRpcNode) TestGetBlockTransactionCount() {
	count, err := s.DefaultClient.GetBlockTransactionCount(s.Context, types.BaseShardId, "latest")
	s.Require().NoError(err)
	s.Zero(count)

	count, err = s.DefaultClient.GetBlockTransactionCount(s.Context, types.BaseShardId, 0x1)
	s.Require().NoError(err)
	s.Zero(count)

	count, err = s.DefaultClient.GetBlockTransactionCount(s.Context, types.MainShardId, 0x1)
	s.Require().NoError(err)
	s.Zero(count)

	count, err = s.DefaultClient.GetBlockTransactionCount(s.Context, types.MainShardId, math.MaxUint32)
	s.Require().NoError(err)
	s.Zero(count)
}

func (s *SuiteRpcNode) TestGetBalance() {
	balance, err := s.DefaultClient.GetBalance(s.Context, types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotNil(balance)

	balance, err = s.DefaultClient.GetBalance(s.Context, types.FaucetAddress, 0x1)
	s.Require().NoError(err)
	s.NotNil(balance)
}

func (s *SuiteRpcNode) TestGetCode() {
	code, err := s.DefaultClient.GetCode(s.Context, types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotNil(code)

	code, err = s.DefaultClient.GetCode(s.Context, types.FaucetAddress, 0x1)
	s.Require().NoError(err)
	s.NotNil(code)
}

func (s *SuiteRpcNode) TestGetCurrencies() {
	currencies, err := s.DefaultClient.GetCurrencies(s.Context, types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotNil(currencies)

	currencies, err = s.DefaultClient.GetCurrencies(s.Context, types.FaucetAddress, 0x1)
	s.Require().NoError(err)
	s.NotNil(currencies)
}

func (s *SuiteRpcNode) TestGasPrice() {
	value, err := s.DefaultClient.GasPrice(s.Context, types.MainShardId)
	s.Require().NoError(err)
	s.Positive(value.Uint64())

	value, err = s.DefaultClient.GasPrice(s.Context, types.BaseShardId)
	s.Require().NoError(err)
	s.Positive(value.Uint64())
}

func TestSuiteRpcNode(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpcNode))
}
