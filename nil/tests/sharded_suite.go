//go:build test

package tests

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type Shard struct {
	Id         types.ShardId
	Db         db.DB
	RpcUrl     string
	P2pAddress string
	Client     client.Client
}

type ShardedSuite struct {
	CliRunner

	Context   context.Context
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
	s.Context, s.ctxCancel = context.WithCancel(context.Background())

	if s.dbInit == nil {
		s.dbInit = func() db.DB {
			db, err := db.NewBadgerDbInMemory()
			s.Require().NoError(err)
			return db
		}
	}

	networkConfigs, p2pAddresses := network.GenerateConfigs(s.T(), cfg.NShards, port)

	s.Shards = make([]Shard, 0, cfg.NShards)
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
			nilservice.Run(s.Context, &shardConfig, s.Shards[i].Db, nil)
			s.wg.Done()
		}()
	}
	s.waitZerostate()
}

func (s *ShardedSuite) checkNodeStart(nShards uint32, client client.Client) {
	for i := range nShards {
		s.Require().Eventually(func() bool {
			block, err := client.GetBlock(types.ShardId(i), transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}

func (s *ShardedSuite) StartArchiveNode(port int, withBootstrapPeers bool) client.Client {
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
		netCfg.DHTBootstrapPeers = append(netCfg.DHTBootstrapPeers, s.Shards[shardId].P2pAddress)
		if withBootstrapPeers {
			cfg.BootstrapPeers = append(cfg.BootstrapPeers, s.Shards[shardId].P2pAddress)
		}
	}

	s.wg.Add(1)
	go func() {
		nilservice.Run(s.Context, cfg, s.dbInit(), nil)
		s.wg.Done()
	}()

	c := rpc_client.NewClient(cfg.HttpUrl, zerolog.New(os.Stderr))
	s.checkNodeStart(cfg.NShards, c)
	return c
}

func (s *ShardedSuite) StartRPCNode(port int) (client.Client, string) {
	s.T().Helper()

	netCfg, _ := network.GenerateConfig(s.T(), port)

	cfg := &nilservice.Config{
		NShards: uint32(len(s.Shards)),
		Network: netCfg,
		HttpUrl: rpc.GetSockPath(s.T()),
		RunMode: nilservice.RpcRunMode,
	}

	for shardId := range cfg.NShards {
		netCfg.DHTBootstrapPeers = append(netCfg.DHTBootstrapPeers, s.Shards[shardId].P2pAddress)
	}

	s.wg.Add(1)
	go func() {
		nilservice.Run(s.Context, cfg, s.dbInit(), nil)
		s.wg.Done()
	}()

	endpoint := strings.Replace(cfg.HttpUrl, "tcp://", "http://", 1)
	c := rpc_client.NewClient(endpoint, zerolog.New(os.Stderr))
	s.checkNodeStart(cfg.NShards, c)
	return c, endpoint
}

func (s *ShardedSuite) WaitForReceipt(client client.Client, shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitForReceipt(s.T(), client, shardId, hash)
}

func (s *ShardedSuite) WaitIncludedInMain(client client.Client, shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitIncludedInMain(s.T(), client, shardId, hash)
}

func (s *ShardedSuite) GasToValue(gas uint64) types.Value {
	return GasToValue(gas)
}

func (s *ShardedSuite) DeployContractViaMainWallet(client client.Client, shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	return DeployContractViaWallet(s.T(), client, types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
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

func (s *ShardedSuite) LoadContract(path string, name string) (types.Code, abi.ABI) {
	s.T().Helper()
	return LoadContract(s.T(), path, name)
}

func (s *ShardedSuite) PrepareDefaultDeployPayload(abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	s.T().Helper()
	return PrepareDefaultDeployPayload(s.T(), abi, code, args...)
}
