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

	client client.Client
}

func (s *SuiteArchiveNode) SetupTest() {
	s.Start(&nilservice.Config{
		NShards:              5,
		CollatorTickPeriodMs: 200,
	}, 10005)

	time.Sleep(1 * time.Second)

	s.client = s.StartArchiveNode(10010)
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

func TestArchiveNode(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteArchiveNode))
}
