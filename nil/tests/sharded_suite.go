//go:build test

package tests

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

type Shard struct {
	Id         types.ShardId
	Db         db.DB
	RpcUrl     string
	P2pAddress string
	Client     client.Client
}

type ShardedSuite struct {
	suite.Suite

	context   context.Context
	ctxCancel context.CancelFunc
	wg        sync.WaitGroup

	dbInit func() db.DB

	Shards []Shard
}

func (s *ShardedSuite) Cancel() {
	s.T().Helper()

	s.ctxCancel()
	s.wg.Wait()
	for _, shard := range s.Shards {
		shard.Db.Close()
	}
}

func (s *ShardedSuite) Start(cfg *nilservice.Config, port int) {
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
		shard := Shard{
			Id:         shardId,
			Db:         s.dbInit(),
			RpcUrl:     url,
			P2pAddress: p2pAddresses[i],
		}
		shard.Client = rpc_client.NewClient(shard.RpcUrl, zerolog.New(os.Stderr))
		s.Shards = append(s.Shards, shard)
	}

	PatchConfigWithTestDefaults(cfg)
	for i := range cfg.NShards {
		s.wg.Add(1)
		go func() {
			shardConfig := nilservice.Config{
				NShards:              cfg.NShards,
				MyShards:             []uint{uint(s.Shards[i].Id)},
				SplitShards:          true,
				HttpUrl:              s.Shards[i].RpcUrl,
				Topology:             cfg.Topology,
				CollatorTickPeriodMs: cfg.CollatorTickPeriodMs,
				GasBasePrice:         cfg.GasBasePrice,
				Network:              networkConfigs[i],
			}
			nilservice.Run(s.context, &shardConfig, s.Shards[i].Db, nil)
			s.wg.Done()
		}()
	}
	s.waitZerostate()
}

func (s *ShardedSuite) StartArchiveNode(port int) client.Client {
	s.T().Helper()

	netCfg, _ := network.GenerateConfig(s.T(), port)

	cfg := &nilservice.Config{
		NShards: uint32(len(s.Shards)),
		Network: netCfg,
		HttpUrl: rpc.GetSockPath(s.T()),
		RunMode: nilservice.ArchiveRunMode,
	}

	for shardId := range cfg.NShards {
		cfg.MyShards = append(cfg.MyShards, uint(shardId))
		cfg.BootstrapPeers = append(cfg.BootstrapPeers, s.Shards[shardId].P2pAddress)
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
	for i := range s.Shards {
		shard := &s.Shards[i]
		s.Require().Eventually(func() bool {
			block, err := shard.Client.GetBlock(shard.Id, transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}
