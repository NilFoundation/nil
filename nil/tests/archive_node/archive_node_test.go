package tests

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteArchiveNode struct {
	tests.ShardedSuite

	nshards            uint32
	withBootstrapPeers bool
	port               int
}

func (s *SuiteArchiveNode) SetupTest() {
	s.nshards = 5

	s.Start(&nilservice.Config{
		NShards:              s.nshards,
		CollatorTickPeriodMs: 200,
	}, s.port)

	s.DefaultClient, _ = s.StartArchiveNode(s.port+int(s.nshards), s.withBootstrapPeers)
}

func (s *SuiteArchiveNode) TearDownTest() {
	s.Cancel()
}

func (s *SuiteArchiveNode) TestGetDebugBlock() {
	for shardId := range len(s.Shards) {
		debugBlock, err := s.DefaultClient.GetDebugBlock(s.Context, types.ShardId(shardId), "latest", true)
		s.Require().NoError(err)
		s.NotNil(debugBlock)

		b, err := debugBlock.DecodeSSZ()
		s.Require().NoError(err)

		s.Eventually(func() bool {
			nextBlock, err := s.DefaultClient.GetDebugBlock(s.Context, types.ShardId(shardId), b.Block.Id.Uint64()+1, true)
			s.Require().NoError(err)
			return nextBlock != nil
		}, 5*time.Second, 100*time.Millisecond)
	}
}

func (s *SuiteArchiveNode) TestGetFaucetBalance() {
	value, err := s.DefaultClient.GetBalance(s.Context, types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.Positive(value.Uint64())
}

func TestArchiveNodeWithBootstrapPeers(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SuiteArchiveNode{
		withBootstrapPeers: true,
		port:               10005,
	})
}

func TestArchiveNodeWithoutBootstrapPeers(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SuiteArchiveNode{
		withBootstrapPeers: false,
		port:               10015,
	})
}
