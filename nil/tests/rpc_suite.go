//go:build test

package tests

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/collate"
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

var DefaultContractValue = types.NewValueFromUint64(50_000_000)

type RpcSuite struct {
	CliRunner

	Context   context.Context
	CtxCancel context.CancelFunc
	Wg        sync.WaitGroup

	DbInit func() db.DB
	Db     db.DB

	Client    client.Client
	ShardsNum uint32
	Endpoint  string
}

func init() {
	logging.SetupGlobalLogger("debug")
}

func PatchConfigWithTestDefaults(cfg *nilservice.Config) {
	if cfg.Topology == "" {
		cfg.Topology = collate.TrivialShardTopologyId
	}
	if cfg.CollatorTickPeriodMs == 0 {
		cfg.CollatorTickPeriodMs = 100
	}
	if cfg.GasBasePrice == 0 {
		cfg.GasBasePrice = 10
	}
}

func (s *RpcSuite) Start(cfg *nilservice.Config) {
	s.T().Helper()

	s.ShardsNum = cfg.NShards
	s.Context, s.CtxCancel = context.WithCancel(context.Background())

	if s.DbInit == nil {
		s.DbInit = func() db.DB {
			db, err := db.NewBadgerDbInMemory()
			s.Require().NoError(err)
			return db
		}
	}
	s.Db = s.DbInit()

	var serviceInterop chan nilservice.ServiceInterop
	if cfg.RunMode == nilservice.CollatorsOnlyRunMode {
		serviceInterop = make(chan nilservice.ServiceInterop, 1)
	}

	PatchConfigWithTestDefaults(cfg)

	s.Wg.Add(1)
	go func() {
		nilservice.Run(s.Context, cfg, s.Db, serviceInterop)
		s.Wg.Done()
	}()

	if cfg.RunMode == nilservice.CollatorsOnlyRunMode {
		service := <-serviceInterop
		c, err := client.NewEthClient(s.Context, &s.Wg, s.Db, types.ShardId(s.ShardsNum), service.MsgPools, zerolog.New(os.Stderr))
		s.Require().NoError(err)
		s.Client = c
	} else {
		s.Endpoint = strings.Replace(cfg.HttpUrl, "tcp://", "http://", 1)
		s.Client = rpc_client.NewClient(s.Endpoint, zerolog.New(os.Stderr))
	}

	if cfg.RunMode == nilservice.BlockReplayRunMode {
		s.Require().Eventually(func() bool {
			block, err := s.Client.GetBlock(cfg.Replay.ShardId, "latest", false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	} else {
		s.waitZerostate()
	}
}

func (s *RpcSuite) StartWithRPC(cfg *nilservice.Config, port int, archive bool) {
	s.T().Helper()

	var err error
	s.ShardsNum = cfg.NShards
	s.Context, s.CtxCancel = context.WithCancel(context.Background())
	s.Db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	var rpcDb db.DB
	if archive {
		rpcDb, err = db.NewBadgerDbInMemory()
		s.Require().NoError(err)
	}

	validatorNetCfg, validatorAddr := network.GenerateConfig(s.T(), port)
	rpcNetCfg, _ := network.GenerateConfig(s.T(), port+1)
	rpcNetCfg.DHTBootstrapPeers = []string{validatorAddr}

	cfg.Network = validatorNetCfg
	PatchConfigWithTestDefaults(cfg)

	rpcCfg := &nilservice.Config{
		NShards: s.ShardsNum,
		Network: rpcNetCfg,
		HttpUrl: rpc.GetSockPath(s.T()),
	}

	for shardId := range s.ShardsNum {
		rpcCfg.MyShards = append(rpcCfg.MyShards, uint(shardId))
		rpcCfg.BootstrapPeers = append(rpcCfg.BootstrapPeers, validatorAddr)
	}

	if archive {
		rpcCfg.RunMode = nilservice.ArchiveRunMode
	} else {
		rpcCfg.RunMode = nilservice.RpcRunMode
	}

	s.Wg.Add(2)
	go func() {
		nilservice.Run(s.Context, cfg, s.Db, nil)
		s.Wg.Done()
	}()

	// TODO: wait be sure that validator is ready
	time.Sleep(time.Second)

	go func() {
		nilservice.Run(s.Context, rpcCfg, rpcDb, nil)
		s.Wg.Done()
	}()

	s.Client = rpc_client.NewClient(rpcCfg.HttpUrl, zerolog.New(os.Stderr))
	s.Endpoint = rpcCfg.HttpUrl

	s.waitZerostateFunc(func(i uint32) bool {
		block, err := s.Client.GetDebugBlock(types.ShardId(i), transport.BlockNumber(0), false)
		return err == nil && block != nil
	})
}

func (s *RpcSuite) Cancel() {
	s.T().Helper()

	s.CtxCancel()
	s.Wg.Wait()
	s.Db.Close()
}

func (s *RpcSuite) WaitForReceipt(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitForReceipt(s.T(), s.Client, shardId, hash)
}

func (s *RpcSuite) WaitIncludedInMain(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return WaitIncludedInMain(s.T(), s.Client, shardId, hash)
}

func (s *RpcSuite) waitZerostate() {
	s.T().Helper()

	s.waitZerostateFunc(func(i uint32) bool {
		block, err := s.Client.GetBlock(types.ShardId(i), transport.BlockNumber(0), false)
		return err == nil && block != nil
	})
}

func (s *RpcSuite) waitZerostateFunc(fun func(i uint32) bool) {
	s.T().Helper()

	for i := range s.ShardsNum {
		s.Require().Eventually(func() bool { return fun(i) }, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}

func (s *RpcSuite) DeployContractViaMainWallet(shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	return DeployContractViaWallet(s.T(), s.Client, types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
}

func (s *RpcSuite) SendMessageViaWallet(addrFrom types.Address, addrTo types.Address, key *ecdsa.PrivateKey,
	calldata []byte,
) *jsonrpc.RPCReceipt {
	s.T().Helper()

	txHash, err := s.Client.SendMessageViaWallet(addrFrom, calldata, GasToValue(1_000_000), types.NewZeroValue(),
		[]types.CurrencyBalance{}, addrTo, key)
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(addrFrom.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)

	return receipt
}

func (s *RpcSuite) SendExternalMessage(bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	s.T().Helper()

	receipt := s.SendExternalMessageNoCheck(bytecode, contractAddress)

	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)

	return receipt
}

func (s *RpcSuite) SendExternalMessageNoCheck(bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	s.T().Helper()
	return SendExternalMessageNoCheck(s.T(), s.Client, bytecode, contractAddress)
}

// SendMessageViaWalletNoCheck sends a message via a wallet contract. Doesn't require the receipt be successful.
func (s *RpcSuite) SendMessageViaWalletNoCheck(addrWallet types.Address, addrTo types.Address, key *ecdsa.PrivateKey,
	calldata []byte, feeCredit, value types.Value, currencies []types.CurrencyBalance,
) *jsonrpc.RPCReceipt {
	s.T().Helper()

	// Send the raw transaction
	txHash, err := s.Client.SendMessageViaWallet(addrWallet, calldata, feeCredit, value, currencies, addrTo, key)
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(addrWallet.ShardId(), txHash)
	// We don't check the receipt for success here, as it can be failed on purpose
	if receipt.Success {
		// But if it is successful, we expect exactly one out receipt
		s.Require().Len(receipt.OutReceipts, 1)
	} else {
		s.Require().NotEqual("Success", receipt.Status)
	}

	return receipt
}

func (s *RpcSuite) CallGetter(addr types.Address, calldata []byte, blockId any, overrides *jsonrpc.StateOverrides) []byte {
	s.T().Helper()
	return CallGetter(s.T(), s.Client, addr, calldata, blockId, overrides)
}

func (s *RpcSuite) LoadContract(path string, name string) (types.Code, abi.ABI) {
	s.T().Helper()
	return LoadContract(s.T(), path, name)
}

func (s *RpcSuite) GetBalance(address types.Address) types.Value {
	s.T().Helper()
	return GetBalance(s.T(), s.Client, address)
}

func (s *RpcSuite) AbiPack(abi *abi.ABI, name string, args ...any) []byte {
	s.T().Helper()
	return AbiPack(s.T(), abi, name, args...)
}

func (s *RpcSuite) AbiUnpack(abi *abi.ABI, name string, data []byte) []interface{} {
	s.T().Helper()
	res, err := abi.Unpack(name, data)
	s.Require().NoError(err)
	return res
}

func (s *RpcSuite) PrepareDefaultDeployPayload(abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	s.T().Helper()
	return PrepareDefaultDeployPayload(s.T(), abi, code, args...)
}

func (s *RpcSuite) GasToValue(gas uint64) types.Value {
	return GasToValue(gas)
}

type ReceiptInfo map[types.Address]*ContractInfo

type ContractInfo struct {
	Name        string
	OutMessages map[types.Address]*jsonrpc.RPCInMessage

	ValueUsed      types.Value
	ValueForwarded types.Value

	ValueReceived  types.Value
	RefundReceived types.Value
	BounceReceived types.Value

	ValueSent  types.Value
	RefundSent types.Value
	BounceSent types.Value

	NumSuccess int
	NumFail    int
}

func (r *ReceiptInfo) Dump() {
	data, err := json.MarshalIndent(r, "", "  ")
	check.PanicIfErr(err)
	fmt.Printf("info: %s\n", string(data))
}

// AllSuccess checks if the contract info contains only successful receipts.
func (r *ReceiptInfo) AllSuccess() bool {
	for _, info := range *r {
		if !info.IsSuccess() {
			return false
		}
	}
	return true
}

// ContainsOnly checks that the receipt info contains only the specified addresses.
func (r *ReceiptInfo) ContainsOnly(addresses ...types.Address) bool {
	if len(*r) != len(addresses) {
		return false
	}
	for _, addr := range addresses {
		if _, found := (*r)[addr]; !found {
			return false
		}
	}
	return true
}

func (c *ContractInfo) IsSuccess() bool {
	return c.NumFail == 0 && c.NumSuccess > 0
}

// GetValueSpent returns the total value spent by the contract.
func (c *ContractInfo) GetValueSpent() types.Value {
	return c.ValueSent.Sub(c.RefundReceived).Sub(c.BounceReceived)
}

func (s *RpcSuite) GetRandomBytes(size int) []byte {
	s.T().Helper()
	data := make([]byte, size)
	_, err := rand.Read(data)
	s.Require().NoError(err)
	return data
}

func (s *RpcSuite) AnalyzeReceipt(receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	s.T().Helper()
	return AnalyzeReceipt(s.T(), s.Client, receipt, namesMap)
}

func getContractInfo(addr types.Address, valuesMap ReceiptInfo) *ContractInfo {
	value, found := valuesMap[addr]
	if !found {
		value = &ContractInfo{}
		value.OutMessages = make(map[types.Address]*jsonrpc.RPCInMessage)
		valuesMap[addr] = value
	}
	return value
}
