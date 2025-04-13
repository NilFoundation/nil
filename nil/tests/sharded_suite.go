//go:build test

package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"slices"
	"sync"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/keys"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/rs/zerolog"
)

type InstanceId uint

type Instance struct {
	Db         db.DB
	RpcUrl     string
	P2pAddress network.AddrInfo
	Client     client.Client
	nm         network.Manager
	Config     *nilservice.Config
}

func getShardAddress(s Instance) network.AddrInfo {
	return s.P2pAddress
}

type ShardedSuite struct {
	CliRunner

	DefaultClient client.Client
	Context       context.Context
	ctxCancel     context.CancelFunc
	Wg            sync.WaitGroup

	DbInit func() db.DB

	Instances []Instance
}

func (s *ShardedSuite) Cancel() {
	s.T().Helper()

	s.ctxCancel()
	s.Wg.Wait()
	for _, shard := range s.Instances {
		shard.Db.Close()
	}
}

func newZeroState(
	oldZeroState *execution.ZeroStateConfig,
	validators []config.ListValidators,
) *execution.ZeroStateConfig {
	return &execution.ZeroStateConfig{
		ConfigParams: execution.ConfigParams{
			Validators: config.ParamValidators{
				Validators: validators,
			},
		},
		Contracts: oldZeroState.Contracts,
	}
}

func createOneShardOneValidatorCfg(
	s *ShardedSuite,
	index InstanceId,
	cfg *nilservice.Config,
	netCfg *network.Config,
	keyManagers map[InstanceId]*keys.ValidatorKeysManager,
) *nilservice.Config {
	validators := make([]config.ListValidators, cfg.NShards-1)
	for i := range validators {
		km := keyManagers[InstanceId(i)]
		pkey, err := km.GetPublicKey()
		s.Require().NoError(err)
		validators[i] = config.ListValidators{
			List: []config.ValidatorInfo{
				{PublicKey: config.Pubkey(pkey)},
			},
		}
	}

	validatorKeysPath := keyManagers[index].GetKeysPath()
	s.Require().NotEmpty(validatorKeysPath)

	shardId := uint(index + 1)
	myShards := []uint{uint(types.MainShardId), shardId}
	if cfg.DisableConsensus {
		if index != 0 {
			myShards = []uint{shardId}
		}
	}

	return &nilservice.Config{
		NShards:               cfg.NShards,
		MyShards:              myShards,
		SplitShards:           true,
		HttpUrl:               s.Instances[index].RpcUrl,
		Topology:              cfg.Topology,
		CollatorTickPeriodMs:  cfg.CollatorTickPeriodMs,
		Network:               netCfg,
		ValidatorKeysPath:     validatorKeysPath,
		ZeroState:             newZeroState(cfg.ZeroState, validators),
		DisableConsensus:      cfg.DisableConsensus,
		NetworkManagerFactory: cfg.NetworkManagerFactory,
	}
}

func createAllShardsAllValidatorsCfg(
	s *ShardedSuite,
	index InstanceId,
	cfg *nilservice.Config,
	netCfg *network.Config,
	keyManagers map[InstanceId]*keys.ValidatorKeysManager,
) *nilservice.Config {
	if cfg.DisableConsensus {
		s.Require().Fail("Consensus is disabled")
	}

	myShards := slices.Collect(common.Range(0, uint(cfg.NShards)))

	validatorKeysPath := keyManagers[index].GetKeysPath()
	validators := make([]config.ListValidators, cfg.NShards-1)

	// Order of validators is important and should be the same for all instances
	for kmId := InstanceId(0); kmId < InstanceId(len(keyManagers)); kmId++ {
		pubkey, err := keyManagers[kmId].GetPublicKey()
		s.Require().NoError(err)

		for i := range validators {
			validators[i].List = append(validators[i].List, config.ValidatorInfo{
				PublicKey: config.Pubkey(pubkey),
			})
		}
	}

	return &nilservice.Config{
		NShards:              cfg.NShards,
		MyShards:             myShards,
		SplitShards:          true,
		HttpUrl:              s.Instances[index].RpcUrl,
		Topology:             cfg.Topology,
		CollatorTickPeriodMs: cfg.CollatorTickPeriodMs,
		Network:              netCfg,
		ValidatorKeysPath:    validatorKeysPath,
		ZeroState:            newZeroState(cfg.ZeroState, validators),
	}
}

func (s *ShardedSuite) start(
	cfg *nilservice.Config,
	port int,
	shardCfgGen func(
		*ShardedSuite,
		InstanceId,
		*nilservice.Config,
		*network.Config,
		map[InstanceId]*keys.ValidatorKeysManager,
	) *nilservice.Config,
	options ...network.Option,
) {
	s.T().Helper()
	s.Context, s.ctxCancel = context.WithCancel(
		context.WithValue(context.Background(), concurrent.RootContextNameLabel, "cluster lifecycle"))

	if s.DbInit == nil {
		s.DbInit = func() db.DB {
			d, err := db.NewBadgerDbInMemory()
			s.Require().NoError(err)
			return d
		}
	}

	instanceCount := cfg.NShards - 1
	networkConfigs, p2pAddresses := network.GenerateConfigs(s.T(), instanceCount, port)
	for _, networkConfig := range networkConfigs {
		s.Require().NoError(networkConfig.Apply(options...))
	}
	keysManagers := make(map[InstanceId]*keys.ValidatorKeysManager)
	s.Instances = make([]Instance, instanceCount)

	if cfg.L1Fetcher == nil {
		cfg.L1Fetcher = GetDummyL1Fetcher()
	}

	if cfg.ZeroState == nil {
		var err error
		cfg.ZeroState, err = execution.CreateDefaultZeroStateConfig(execution.MainPublicKey)
		s.Require().NoError(err)
	}

	for index := range InstanceId(instanceCount) {
		keysPath := s.T().TempDir() + fmt.Sprintf("/validator-keys-%d.yaml", index)
		km := keys.NewValidatorKeyManager(keysPath)
		s.Require().NotNil(km)
		s.Require().NoError(km.InitKey())
		keysManagers[index] = km

		url := rpc.GetSockPathIdx(s.T(), int(index))
		s.Instances[index] = Instance{
			Db:         s.DbInit(),
			RpcUrl:     url,
			P2pAddress: p2pAddresses[index],
			Client:     rpc_client.NewClient(url, logging.NewFromZerolog(zerolog.New(os.Stderr))),
		}
	}

	PatchConfigWithTestDefaults(cfg)

	for index := range InstanceId(instanceCount) {
		shardConfig := shardCfgGen(s, index, cfg, networkConfigs[index], keysManagers)
		shardConfig.L1Fetcher = cfg.L1Fetcher

		node, err := nilservice.CreateNode(
			s.Context, fmt.Sprintf("shard-%d", index), shardConfig, s.Instances[index].Db, nil)
		s.Require().NoError(err)
		s.Instances[index].nm = node.NetworkManager
		s.Instances[index].Config = shardConfig

		s.Wg.Add(1)
		go func() {
			defer s.Wg.Done()
			defer node.Close(s.Context)
			s.NoError(node.Run())
		}()
	}

	for _, shard := range s.Instances {
		s.connectToInstances(shard.nm)
	}

	s.waitZerostate()
	s.waitShardsTick(cfg.NShards)
}

func (s *ShardedSuite) Start(cfg *nilservice.Config, port int, options ...network.Option) {
	s.T().Helper()

	s.start(cfg, port, createOneShardOneValidatorCfg, options...)
}

func (s *ShardedSuite) StartShardAllValidators(cfg *nilservice.Config, port int) {
	s.T().Helper()

	s.start(cfg, port, createAllShardsAllValidatorsCfg)
}

func (s *ShardedSuite) connectToInstances(nm network.Manager) {
	s.T().Helper()

	var wg sync.WaitGroup
	for _, shard := range s.Instances {
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

func (s *ShardedSuite) GetNShards() uint32 {
	return s.Instances[0].Config.NShards
}

type ArchiveNodeConfig struct {
	Ctx                   context.Context
	Wg                    *sync.WaitGroup
	AllowDbDrop           bool
	Port                  int
	WithBootstrapPeers    bool
	DisableConsensus      bool
	NetworkOptions        []network.Option
	SyncTimeoutFactor     uint32
	NetworkManagerFactory func(context.Context, *nilservice.Config, db.DB) (network.Manager, error)
}

type RpcNodeConfig struct {
	WithDhtBootstrapByValidators bool
	ArchiveNodes                 network.AddrInfoSlice
}

func (s *ShardedSuite) RunArchiveNode(params *ArchiveNodeConfig) (*nilservice.Config, network.AddrInfo, chan error) {
	s.T().Helper()

	ctx := s.Context
	if params.Ctx != nil {
		ctx = params.Ctx
	}
	ctx = context.WithValue(ctx, concurrent.RootContextNameLabel, "archive node lifecycle")

	wg := &s.Wg
	if params.Wg != nil {
		wg = params.Wg
	}

	s.Require().NotEmpty(s.Instances)
	netCfg, addr := network.GenerateConfig(s.T(), params.Port)
	s.Require().NoError(netCfg.Apply(params.NetworkOptions...))
	serviceName := fmt.Sprintf("archive-%d", params.Port)

	cfg := &nilservice.Config{
		AllowDbDrop:           params.AllowDbDrop,
		NShards:               s.GetNShards(),
		Network:               netCfg,
		HttpUrl:               rpc.GetSockPathService(s.T(), serviceName),
		RunMode:               nilservice.ArchiveRunMode,
		ZeroState:             s.Instances[0].Config.ZeroState,
		DisableConsensus:      params.DisableConsensus,
		SyncTimeoutFactor:     params.SyncTimeoutFactor,
		NetworkManagerFactory: params.NetworkManagerFactory,
	}
	PatchConfigWithTestDefaults(cfg)

	cfg.MyShards = slices.Collect(common.Range(0, uint(cfg.NShards)))
	netCfg.DHTBootstrapPeers = slices.Collect(common.Transform(slices.Values(s.Instances), getShardAddress))
	if params.WithBootstrapPeers {
		cfg.BootstrapPeers = slices.Insert(slices.Clone(netCfg.DHTBootstrapPeers), 0, netCfg.DHTBootstrapPeers[0])
	}

	node, err := nilservice.CreateNode(ctx, serviceName, cfg, s.DbInit(), nil)
	s.Require().NoError(err)
	s.connectToInstances(node.NetworkManager)

	wg.Add(1)
	rc := make(chan error, 1)
	go func() {
		defer wg.Done()
		defer node.Close(ctx)
		rc <- node.Run()
	}()

	return cfg, addr, rc
}

func (s *ShardedSuite) EnsureArchiveNodeStarted(
	cfg *nilservice.Config,
	addr network.AddrInfo,
	rc chan error,
) (client.Client, network.AddrInfo) {
	s.T().Helper()

	s.Wg.Add(1)
	go func() {
		defer s.Wg.Done()
		err := <-rc
		s.NoError(err)
	}()

	c := rpc_client.NewClient(cfg.HttpUrl, logging.NewFromZerolog(zerolog.New(os.Stderr)))
	s.checkNodeStart(cfg.NShards, c)
	return c, addr
}

func (s *ShardedSuite) StartArchiveNode(params *ArchiveNodeConfig) (client.Client, network.AddrInfo) {
	s.T().Helper()

	return s.EnsureArchiveNodeStarted(s.RunArchiveNode(params))
}

func (s *ShardedSuite) StartRPCNode(params *RpcNodeConfig) (client.Client, string) {
	s.T().Helper()

	netCfg, _ := network.GenerateConfig(s.T(), 0)
	const serviceName = "rpc"

	s.Require().NotEmpty(s.Instances)
	cfg := &nilservice.Config{
		NShards: s.GetNShards(),
		Network: netCfg,
		HttpUrl: rpc.GetSockPathService(s.T(), serviceName),
		RunMode: nilservice.RpcRunMode,
		RpcNode: nilservice.NewDefaultRpcNodeConfig(),
	}

	if params.WithDhtBootstrapByValidators {
		netCfg.DHTBootstrapPeers = slices.Collect(common.Transform(slices.Values(s.Instances), getShardAddress))
	}
	cfg.RpcNode.ArchiveNodeList = params.ArchiveNodes

	node, err := nilservice.CreateNode(s.Context, serviceName, cfg, s.DbInit(), nil)
	s.Require().NoError(err)
	if params.WithDhtBootstrapByValidators {
		s.connectToInstances(node.NetworkManager)
	}

	s.Wg.Add(1)
	go func() {
		defer s.Wg.Done()
		defer node.Close(s.Context)
		s.NoError(node.Run())
	}()

	c := rpc_client.NewClient(cfg.HttpUrl, logging.NewFromZerolog(zerolog.New(os.Stderr)))
	s.checkNodeStart(cfg.NShards, c)
	return c, cfg.HttpUrl
}

func (s *ShardedSuite) WaitForReceipt(hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitForReceipt(s.T(), s.DefaultClient, hash)
}

func (s *ShardedSuite) WaitIncludedInMain(hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitIncludedInMain(s.T(), s.DefaultClient, hash)
}

func (*ShardedSuite) GasToValue(gas uint64) types.Value {
	return GasToValue(gas)
}

func (s *ShardedSuite) DeployContractViaMainSmartAccount(
	shardId types.ShardId,
	payload types.DeployPayload,
	initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	return DeployContractViaSmartAccount(
		s.T(),
		s.DefaultClient,
		types.MainSmartAccountAddress,
		execution.MainPrivateKey,
		shardId,
		payload,
		initialAmount)
}

func (s *ShardedSuite) checkNodeStart(nShards uint32, client client.Client) {
	s.T().Helper()

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
	wg.Add(len(s.Instances))
	for _, instance := range s.Instances {
		go func() {
			defer wg.Done()

			for _, shard := range instance.Config.MyShards {
				WaitZerostate(s.T(), instance.Client, types.ShardId(shard))
			}
		}()
	}
	wg.Wait()
}

func (s *ShardedSuite) waitShardsTick(nShards uint32) {
	for _, instance := range s.Instances {
		for shardId := range types.ShardId(nShards) {
			WaitShardTick(s.T(), instance.Client, shardId)
		}
	}
}

func (s *ShardedSuite) LoadContract(path string, name string) (types.Code, abi.ABI) {
	s.T().Helper()
	return LoadContract(s.T(), path, name)
}

func (s *ShardedSuite) PrepareDefaultDeployPayload(
	abi abi.ABI, salt common.Hash, code []byte, args ...any,
) types.DeployPayload {
	s.T().Helper()
	return PrepareDefaultDeployPayload(s.T(), abi, salt, code, args...)
}

func (s *ShardedSuite) GetBalance(address types.Address) types.Value {
	s.T().Helper()
	return GetBalance(s.T(), s.DefaultClient, address)
}

func (s *ShardedSuite) AbiPack(abi *abi.ABI, name string, args ...any) []byte {
	s.T().Helper()
	return AbiPack(s.T(), abi, name, args...)
}

func (s *ShardedSuite) SendExternalTransactionNoCheck(
	bytecode types.Code,
	contractAddress types.Address,
) *jsonrpc.RPCReceipt {
	s.T().Helper()
	return SendExternalTransactionNoCheck(s.T(), s.DefaultClient, bytecode, contractAddress)
}

func (s *ShardedSuite) AnalyzeReceipt(receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	s.T().Helper()
	return AnalyzeReceipt(s.T(), s.DefaultClient, receipt, namesMap)
}

func (s *ShardedSuite) CheckBalance(infoMap ReceiptInfo, balance types.Value, accounts []types.Address) types.Value {
	s.T().Helper()
	return CheckBalance(s.T(), s.DefaultClient, infoMap, balance, accounts)
}

func (s *ShardedSuite) CallGetter(
	addr types.Address,
	calldata []byte,
	blockId any,
	overrides *jsonrpc.StateOverrides,
) []byte {
	s.T().Helper()
	return CallGetter(s.T(), s.DefaultClient, addr, calldata, blockId, overrides)
}

func (s *ShardedSuite) SendTransactionViaSmartAccountNoCheck(
	addrSmartAccount types.Address,
	addrTo types.Address,
	key *ecdsa.PrivateKey,
	calldata []byte,
	fee types.FeePack,
	value types.Value,
	tokens []types.TokenBalance,
) *jsonrpc.RPCReceipt {
	s.T().Helper()
	return SendTransactionViaSmartAccountNoCheck(
		s.T(), s.DefaultClient, addrSmartAccount, addrTo, key, calldata, fee, value, tokens)
}
