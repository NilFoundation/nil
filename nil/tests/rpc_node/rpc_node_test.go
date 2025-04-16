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
	nShards := uint32(3)

	s.Start(&nilservice.Config{
		NShards: nShards,
		RunMode: nilservice.NormalRunMode,
	}, port)

	_, archiveNodeAddr := s.StartArchiveNode(&tests.ArchiveNodeConfig{
		Port:               port + int(nShards),
		WithBootstrapPeers: true,
	})
	s.DefaultClient, _ = s.StartRPCNode(&tests.RpcNodeConfig{
		WithDhtBootstrapByValidators: false,
		ArchiveNodes:                 network.AddrInfoSlice{archiveNodeAddr},
	})
}

func (s *SuiteRpcNode) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRpcNode) TestRpcNode() {
	s.Run("TestGetDebugBlock", func() {
		debugBlock, err := s.DefaultClient.GetDebugBlock(s.Context, types.BaseShardId, "latest", true)
		s.Require().NoError(err)
		s.NotNil(debugBlock)

		debugBlock, err = s.DefaultClient.GetDebugBlock(s.Context, types.BaseShardId, 0x1, true)
		s.Require().NoError(err)
		s.NotNil(debugBlock)
	})

	s.Run("TestGetBlock", func() {
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
		s.NotZero(block.DbTimestamp)
	})

	s.Run("TestGetBlockTransactionCount", func() {
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
	})

	s.Run("TestGetBalance", func() {
		balance, err := s.DefaultClient.GetBalance(s.Context, types.FaucetAddress, "latest")
		s.Require().NoError(err)
		s.NotNil(balance)

		balance, err = s.DefaultClient.GetBalance(s.Context, types.FaucetAddress, 0x1)
		s.Require().NoError(err)
		s.NotNil(balance)
	})

	s.Run("TestGetCode", func() {
		code, err := s.DefaultClient.GetCode(s.Context, types.FaucetAddress, "latest")
		s.Require().NoError(err)
		s.NotNil(code)

		code, err = s.DefaultClient.GetCode(s.Context, types.FaucetAddress, 0x1)
		s.Require().NoError(err)
		s.NotNil(code)
	})

	s.Run("TestGetTokens", func() {
		tokens, err := s.DefaultClient.GetTokens(s.Context, types.FaucetAddress, "latest")
		s.Require().NoError(err)
		s.NotNil(tokens)

		tokens, err = s.DefaultClient.GetTokens(s.Context, types.FaucetAddress, 0x1)
		s.Require().NoError(err)
		s.NotNil(tokens)
	})

	s.Run("TestGasPrice", func() {
		value, err := s.DefaultClient.GasPrice(s.Context, types.MainShardId)
		s.Require().NoError(err)
		s.NotZero(value.Uint64())

		value, err = s.DefaultClient.GasPrice(s.Context, types.BaseShardId)
		s.Require().NoError(err)
		s.NotZero(value.Uint64())
	})
}

func TestSuiteRpcNode(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpcNode))
}
