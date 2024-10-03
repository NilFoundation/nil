package rpctest

import (
	"context"
	"os"
	"testing"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SuiteRpcNode struct {
	RpcSuite
}

func (s *SuiteRpcNode) SetupTest() {
	var err error

	s.shardsNum = 5
	s.context, s.ctxCancel = context.WithCancel(context.Background())
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	validatorNetCfg, validatorAddr := network.GenerateConfig(s.T(), 11001)
	rpcNetCfg, _ := network.GenerateConfig(s.T(), 11002)

	validatorCfg := &nilservice.Config{
		NShards: s.shardsNum,
		Network: validatorNetCfg,
		RunMode: nilservice.NormalRunMode,
	}
	PatchConfigWithTestDefaults(validatorCfg)

	rpcCfg := &nilservice.Config{
		NShards:       s.shardsNum,
		Network:       rpcNetCfg,
		RunMode:       nilservice.RpcRunMode,
		BootstrapPeer: validatorAddr,
		HttpUrl:       rpc.GetSockPath(s.T()),
	}

	s.wg.Add(2)
	go func() {
		nilservice.Run(s.context, validatorCfg, s.db, nil)
		s.wg.Done()
	}()

	go func() {
		nilservice.Run(s.context, rpcCfg, nil, nil)
		s.wg.Done()
	}()

	s.client = rpc_client.NewClient(rpcCfg.HttpUrl, zerolog.New(os.Stderr))

	s.waitZerostateFunc(func(i uint32) bool {
		block, err := s.client.GetDebugBlock(types.ShardId(i), transport.BlockNumber(0), false)
		return err == nil && block != nil
	})
}

func (s *SuiteRpcNode) TearDownTest() {
	s.cancel()
}

func (s *SuiteRpcNode) TestGetDebugBlock() {
	debugBlock, err := s.client.GetDebugBlock(types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.NotNil(debugBlock)

	debugBlock, err = s.client.GetDebugBlock(types.BaseShardId, 0x1, true)
	s.Require().NoError(err)
	s.NotNil(debugBlock)
}

func (s *SuiteRpcNode) TestGetBlock() {
	block, err := s.client.GetBlock(types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.NotNil(block)

	block, err = s.client.GetBlock(types.BaseShardId, 0x1, true)
	s.Require().NoError(err)
	s.NotNil(block)

	block, err = s.client.GetBlock(types.MainShardId, 0x1, true)
	s.Require().NoError(err)
	s.Require().NotNil(block)
	s.NotEmpty(block.ChildBlocks)
	s.Positive(block.DbTimestamp)
}

func TestSuiteRpcNode(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpcNode))
}
