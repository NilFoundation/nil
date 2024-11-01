//go:build test

package tests

import (
	"context"
	"fmt"
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
	"github.com/rs/zerolog"
)

type Shard struct {
	Id         types.ShardId
	Db         db.DB
	RpcUrl     string
	P2pAddress string
	Client     client.Client
	nm         *network.Manager
}

type ShardedSuite struct {
	CliRunner

	DefaultClient client.Client
	Context       context.Context
	ctxCancel     context.CancelFunc
	wg            sync.WaitGroup

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
		shardConfig := &nilservice.Config{
			NShards:              cfg.NShards,
			MyShards:             []uint{uint(s.Shards[i].Id)},
			SplitShards:          true,
			HttpUrl:              s.Shards[i].RpcUrl,
			Topology:             cfg.Topology,
			CollatorTickPeriodMs: cfg.CollatorTickPeriodMs,
			GasBasePrice:         cfg.GasBasePrice,
			Network:              networkConfigs[i],
			ZeroStateYaml:        cfg.ZeroStateYaml,
		}
		node, err := nilservice.CreateNode(s.Context, fmt.Sprintf("shard-%d", i), shardConfig, s.Shards[i].Db, nil)
		s.Require().NoError(err)
		s.Shards[i].nm = node.NetworkManager

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer node.Close()
			s.NoError(node.Run())
		}()
	}

	for _, shard := range s.Shards {
		s.connectToShards(shard.nm)
	}

	s.waitZerostate()
}

func (s *ShardedSuite) connectToShards(nm *network.Manager) {
	s.T().Helper()

	var wg sync.WaitGroup
	for _, shard := range s.Shards {
		if shard.nm != nm {
			wg.Add(1)
			go func() {
				defer wg.Done()
				network.ConnectManagers(s.T(), nm, shard.nm)
			}()
		}
	}
	wg.Wait()
}

func (s *ShardedSuite) StartArchiveNode(port int, withBootstrapPeers bool) client.Client {
	s.T().Helper()

	netCfg, _ := network.GenerateConfig(s.T(), port)
	serviceName := fmt.Sprintf("archive-%d", port)

	cfg := &nilservice.Config{
		NShards: uint32(len(s.Shards)),
		Network: netCfg,
		HttpUrl: rpc.GetSockPathService(s.T(), serviceName),
		RunMode: nilservice.ArchiveRunMode,
	}

	for shardId := range cfg.NShards {
		cfg.MyShards = append(cfg.MyShards, uint(shardId))
		netCfg.DHTBootstrapPeers = append(netCfg.DHTBootstrapPeers, s.Shards[shardId].P2pAddress)
		if withBootstrapPeers {
			cfg.BootstrapPeers = append(cfg.BootstrapPeers, s.Shards[shardId].P2pAddress)
		}
	}

	node, err := nilservice.CreateNode(s.Context, serviceName, cfg, s.dbInit(), nil)
	s.Require().NoError(err)
	s.connectToShards(node.NetworkManager)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer node.Close()
		s.NoError(node.Run())
	}()

	c := rpc_client.NewClient(cfg.HttpUrl, zerolog.New(os.Stderr))
	s.checkNodeStart(cfg.NShards, c)
	return c
}

func (s *ShardedSuite) StartRPCNode(port int) (client.Client, string) {
	s.T().Helper()

	netCfg, _ := network.GenerateConfig(s.T(), port)
	serviceName := fmt.Sprintf("rpc-%d", port)

	cfg := &nilservice.Config{
		NShards: uint32(len(s.Shards)),
		Network: netCfg,
		HttpUrl: rpc.GetSockPathService(s.T(), serviceName),
		RunMode: nilservice.RpcRunMode,
	}

	for shardId := range cfg.NShards {
		netCfg.DHTBootstrapPeers = append(netCfg.DHTBootstrapPeers, s.Shards[shardId].P2pAddress)
	}

	node, err := nilservice.CreateNode(s.Context, serviceName, cfg, s.dbInit(), nil)
	s.Require().NoError(err)
	s.connectToShards(node.NetworkManager)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer node.Close()
		s.NoError(node.Run())
	}()

	endpoint := strings.Replace(cfg.HttpUrl, "tcp://", "http://", 1)
	c := rpc_client.NewClient(endpoint, zerolog.New(os.Stderr))
	s.checkNodeStart(cfg.NShards, c)
	return c, endpoint
}

func (s *ShardedSuite) WaitForReceipt(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitForReceipt(s.T(), s.DefaultClient, shardId, hash)
}

func (s *ShardedSuite) WaitIncludedInMain(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitIncludedInMain(s.T(), s.DefaultClient, shardId, hash)
}

func (s *ShardedSuite) GasToValue(gas uint64) types.Value {
	return GasToValue(gas)
}

func (s *ShardedSuite) DeployContractViaMainWallet(shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	return DeployContractViaWallet(s.T(), s.DefaultClient, types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
}

func (s *ShardedSuite) checkNodeStart(nShards uint32, client client.Client) {
	var wg sync.WaitGroup
	wg.Add(int(nShards))
	for shardId := range types.ShardId(nShards) {
		go func() {
			defer wg.Done()
			WaitZerostate(s.T(), client, shardId)
		}()
	}
	wg.Wait()
}

func (s *ShardedSuite) waitZerostate() {
	s.T().Helper()

	var wg sync.WaitGroup
	wg.Add(len(s.Shards))
	for _, shard := range s.Shards {
		go func() {
			defer wg.Done()
			WaitZerostate(s.T(), shard.Client, shard.Id)
		}()
	}
	wg.Wait()
}

func (s *ShardedSuite) LoadContract(path string, name string) (types.Code, abi.ABI) {
	s.T().Helper()
	return LoadContract(s.T(), path, name)
}

func (s *ShardedSuite) PrepareDefaultDeployPayload(abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	s.T().Helper()
	return PrepareDefaultDeployPayload(s.T(), abi, code, args...)
}

func (s *ShardedSuite) GetBalance(address types.Address) types.Value {
	s.T().Helper()
	return GetBalance(s.T(), s.DefaultClient, address)
}

func (s *ShardedSuite) AbiPack(abi *abi.ABI, name string, args ...any) []byte {
	s.T().Helper()
	return AbiPack(s.T(), abi, name, args...)
}

func (s *ShardedSuite) SendExternalMessageNoCheck(bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	s.T().Helper()
	return SendExternalMessageNoCheck(s.T(), s.DefaultClient, bytecode, contractAddress)
}

func (s *ShardedSuite) AnalyzeReceipt(receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	s.T().Helper()
	return AnalyzeReceipt(s.T(), s.DefaultClient, receipt, namesMap)
}

func (s *ShardedSuite) CheckBalance(infoMap ReceiptInfo, balance types.Value, accounts []types.Address) types.Value {
	s.T().Helper()
	return CheckBalance(s.T(), s.DefaultClient, infoMap, balance, accounts)
}

func (s *ShardedSuite) CallGetter(addr types.Address, calldata []byte, blockId any, overrides *jsonrpc.StateOverrides) []byte {
	s.T().Helper()
	return CallGetter(s.T(), s.DefaultClient, addr, calldata, blockId, overrides)
}
