package rpctest

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/stretchr/testify/suite"
)

type SuiteArchiveNode struct {
	RpcSuite
}

func (s *SuiteArchiveNode) SetupTest() {
	s.startWithRPC(&nilservice.Config{
		NShards: 5,
	}, 11007, true)
}

func (s *SuiteArchiveNode) TearDownTest() {
	s.cancel()
}

func (s *SuiteArchiveNode) TestGetDebugBlock() {
	for shardId := range s.shardsNum {
		debugBlock, err := s.client.GetDebugBlock(types.ShardId(shardId), "latest", true)
		s.Require().NoError(err)
		s.NotNil(debugBlock)

		b, err := debugBlock.DecodeSSZ()
		s.Require().NoError(err)

		s.Eventually(func() bool {
			nextBlock, err := s.client.GetDebugBlock(types.ShardId(shardId), b.Block.Id.Uint64()+1, true)
			s.Require().NoError(err)
			return nextBlock != nil
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func TestArchiveNode(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteArchiveNode))
}
