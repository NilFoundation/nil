package main

import (
	"testing"

	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type BasicShardSuite struct {
	tests.ShardedSuite
}

func (s *BasicShardSuite) SetupSuite() {
	s.Start(&nilservice.Config{
		NShards:              3,
		CollatorTickPeriodMs: 1000,
	}, 10000)
}

func (s *BasicShardSuite) TearDownSuite() {
	s.Cancel()
}

func (s *BasicShardSuite) TestBasic() {
	// get latest blocks from all shards
	for i := range s.Shards {
		shard := &s.Shards[i]
		rpcBlock, err := shard.Client.GetBlock(s.Context, shard.Id, "latest", false)
		s.Require().NoError(err)
		s.Require().NotNil(rpcBlock)

		// check that the block makes it to other shards
		for j := range s.Shards {
			if i == j {
				continue
			}
			otherShard := &s.Shards[j]
			s.Require().Eventually(func() bool {
				otherBlock, err := otherShard.Client.GetBlock(s.Context, shard.Id, transport.BlockNumber(rpcBlock.Number), false)
				if err != nil || otherBlock == nil {
					return false
				}
				return otherBlock.Hash == rpcBlock.Hash
			}, tests.BlockWaitTimeout, tests.BlockPollInterval)
		}
	}
}

func TestShards(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BasicShardSuite))
}
