package rpctest

import (
	"context"
	"os"
	"sync"

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
	id         types.ShardId
	db         db.DB
	rpcUrl     string
	p2pAddress string

	client client.Client
}

type ShardedSuite struct {
	suite.Suite

	context   context.Context
	ctxCancel context.CancelFunc
	wg        sync.WaitGroup

	dbInit func() db.DB

	shards []shard
}

func (s *ShardedSuite) cancel() {
	s.T().Helper()

	s.ctxCancel()
	s.wg.Wait()
	for _, shard := range s.shards {
		shard.db.Close()
	}
}

func (s *ShardedSuite) start(cfg *nilservice.Config, port int) {
	s.T().Helper()
	s.context, s.ctxCancel = context.WithCancel(context.Background())

	if s.dbInit == nil {
		s.dbInit = func() db.DB {
			db, err := db.NewBadgerDbInMemory()
			s.Require().NoError(err)
			return db
		}
	}

	networkConfigs, p2pAddresses := network.GenerateConfigs(s.T(), cfg.NShards, port)

	for i := range cfg.NShards {
		shardId := types.ShardId(i)
		url := rpc.GetSockPathIdx(s.T(), int(i))
		shard := shard{
			id:         shardId,
			db:         s.dbInit(),
			rpcUrl:     url,
			p2pAddress: p2pAddresses[i],
		}
		shard.client = rpc_client.NewClient(shard.rpcUrl, zerolog.New(os.Stderr))
		s.shards = append(s.shards, shard)
	}

	PatchConfigWithTestDefaults(cfg)
	for i := range cfg.NShards {
		s.wg.Add(1)
		go func() {
			shardConfig := nilservice.Config{
				NShards:              cfg.NShards,
				MyShards:             []uint{uint(s.shards[i].id)},
				SplitShards:          true,
				HttpUrl:              s.shards[i].rpcUrl,
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

func (s *ShardedSuite) startArchiveNode(port int) client.Client {
	s.T().Helper()

	netCfg, _ := network.GenerateConfig(s.T(), port)

	cfg := &nilservice.Config{
		NShards: uint32(len(s.shards)),
		Network: netCfg,
		HttpUrl: rpc.GetSockPath(s.T()),
		RunMode: nilservice.ArchiveRunMode,
	}

	for shardId := range cfg.NShards {
		cfg.MyShards = append(cfg.MyShards, uint(shardId))
		cfg.BootstrapPeers = append(cfg.BootstrapPeers, s.shards[shardId].p2pAddress)
	}

	s.wg.Add(1)
	go func() {
		nilservice.Run(s.context, cfg, s.dbInit(), nil)
		s.wg.Done()
	}()

	c := rpc_client.NewClient(cfg.HttpUrl, zerolog.New(os.Stderr))

	for i := range cfg.NShards {
		s.Require().Eventually(func() bool {
			block, err := c.GetBlock(types.ShardId(i), transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}

	return c
}

func (s *ShardedSuite) waitZerostate() {
	s.T().Helper()
	for i := range s.shards {
		shard := &s.shards[i]
		s.Require().Eventually(func() bool {
			block, err := shard.client.GetBlock(shard.id, transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}
