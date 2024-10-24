package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/stretchr/testify/suite"
)

type BasicShardSuite struct {
	ShardedSuite
}

func (s *BasicShardSuite) SetupSuite() {
	s.start(&nilservice.Config{
		NShards:              3,
		CollatorTickPeriodMs: 1000,
	}, 10000)
}

func (s *BasicShardSuite) TearDownSuite() {
	s.cancel()
}

func (s *BasicShardSuite) TestBasic() {
	// get latest blocks from all shards
	for i := range s.shards {
		shard := &s.shards[i]
		rpcBlock, err := shard.client.GetBlock(shard.id, "latest", false)
		s.Require().NoError(err)
		s.Require().NotNil(rpcBlock)

		// check that the block makes it to other shards
		for j := range s.shards {
			if i == j {
				continue
			}
			otherShard := &s.shards[j]
			s.Require().Eventually(func() bool {
				otherBlock, err := otherShard.client.GetBlock(shard.id, transport.BlockNumber(rpcBlock.Number), false)
				if err != nil || otherBlock == nil {
					return false
				}
				return otherBlock.Hash == rpcBlock.Hash
			}, ZeroStateWaitTimeout, ZeroStatePollInterval)
		}
	}
}

func TestShards(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BasicShardSuite))
}
