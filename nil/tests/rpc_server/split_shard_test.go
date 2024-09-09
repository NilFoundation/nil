package rpctest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
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

type shard struct {
	id  types.ShardId
	db  db.DB
	url string

	client client.Client
}

type SuiteSplitShard struct {
	suite.Suite

	context   context.Context
	ctxCancel context.CancelFunc
	wg        sync.WaitGroup

	dbInit func() db.DB

	shards []shard
}

func (s *SuiteSplitShard) cancel() {
	s.T().Helper()

	s.ctxCancel()
	s.wg.Wait()
	for _, shard := range s.shards {
		shard.db.Close()
	}
}

func (s *SuiteSplitShard) start(cfg *nilservice.Config) {
	s.T().Helper()
	s.context, s.ctxCancel = context.WithCancel(context.Background())

	if s.dbInit == nil {
		s.dbInit = func() db.DB {
			db, err := db.NewBadgerDbInMemory()
			s.Require().NoError(err)
			return db
		}
	}

	networkConfigs := network.GenerateConfigs(s.T(), cfg.NShards)

	for i := range cfg.NShards {
		shardId := types.ShardId(i)
		url := rpc.GetSockPathIdx(s.T(), int(i))
		shard := shard{
			id:  shardId,
			db:  s.dbInit(),
			url: url,
		}
		shard.client = rpc_client.NewClient(shard.url, zerolog.New(os.Stderr))
		s.shards = append(s.shards, shard)
	}

	PatchConfigWithTestDefaults(cfg)
	for i := range cfg.NShards {
		s.wg.Add(1)
		go func() {
			shardConfig := nilservice.Config{
				NShards:              cfg.NShards,
				MyShard:              s.shards[i].id,
				SplitShards:          true,
				HttpUrl:              s.shards[i].url,
				Topology:             cfg.Topology,
				CollatorTickPeriodMs: cfg.CollatorTickPeriodMs,
				GasBasePrice:         cfg.GasBasePrice,
				Network:              networkConfigs[i],
			}
			nilservice.Run(s.context, &shardConfig, s.shards[i].db, nil)
			s.wg.Done()
		}()
	}
	s.waitZerostate()
}

func (s *SuiteSplitShard) waitZerostate() {
	s.T().Helper()
	for i := range s.shards {
		shard := &s.shards[i]
		s.Require().Eventually(func() bool {
			block, err := shard.client.GetBlock(shard.id, transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}

func (s *SuiteSplitShard) SetupSuite() {
	s.start(&nilservice.Config{
		NShards:              3,
		CollatorTickPeriodMs: 1000,
	})
}

func (s *SuiteSplitShard) TearDownSuite() {
	s.cancel()
}

func (s *SuiteSplitShard) TestBasic() {
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

func TestExampleTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteSplitShard))
}
