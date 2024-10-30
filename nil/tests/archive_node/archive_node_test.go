package tests

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client"
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
	client             client.Client
}

func (s *SuiteArchiveNode) SetupTest() {
	s.nshards = 5

	s.Start(&nilservice.Config{
		NShards:              s.nshards,
		CollatorTickPeriodMs: 200,
	}, s.port)

	s.client = s.StartArchiveNode(s.port+int(s.nshards), s.withBootstrapPeers)
}

func (s *SuiteArchiveNode) TearDownTest() {
	s.Cancel()
}

func (s *SuiteArchiveNode) TestGetDebugBlock() {
	for shardId := range len(s.Shards) {
		debugBlock, err := s.client.GetDebugBlock(types.ShardId(shardId), "latest", true)
		s.Require().NoError(err)
		s.NotNil(debugBlock)

		b, err := debugBlock.DecodeSSZ()
		s.Require().NoError(err)

		s.Eventually(func() bool {
			nextBlock, err := s.client.GetDebugBlock(types.ShardId(shardId), b.Block.Id.Uint64()+1, true)
			s.Require().NoError(err)
			return nextBlock != nil
		}, 5*time.Second, 100*time.Millisecond)
	}
}

func (s *SuiteArchiveNode) TestGetFaucetBalance() {
	value, err := s.client.GetBalance(types.FaucetAddress, "latest")
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
