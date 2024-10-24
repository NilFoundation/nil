//go:build test

package tests

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
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
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/suite"
)

var DefaultContractValue = types.NewValueFromUint64(50_000_000)

type RpcSuite struct {
	suite.Suite

	Context   context.Context
	CtxCancel context.CancelFunc
	Wg        sync.WaitGroup

	DbInit func() db.DB
	Db     db.DB

	Client         client.Client
	ShardsNum      uint32
	Endpoint       string
	CometaEndpoint string
	TmpDir         string
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
		s.CometaEndpoint = strings.Replace(rpc.GetSockPathIdx(s.T(), 1), "tcp://", "http://", 1)
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
	s.CometaEndpoint = strings.Replace(rpc.GetSockPathIdx(s.T(), 1), "tcp://", "http://", 1)

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

func (s *RpcSuite) WaitForReceiptCommon(shardId types.ShardId, hash common.Hash, check func(*jsonrpc.RPCReceipt) bool) *jsonrpc.RPCReceipt {
	s.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	s.Require().Eventually(func() bool {
		receipt, err = s.Client.GetInMessageReceipt(shardId, hash)
		s.Require().NoError(err)
		return check(receipt)
	}, ReceiptWaitTimeout, ReceiptPollInterval)

	s.Equal(hash, receipt.MsgHash)

	return receipt
}

func (s *RpcSuite) WaitForReceipt(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return s.WaitForReceiptCommon(shardId, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsComplete()
	})
}

func (s *RpcSuite) WaitIncludedInMain(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	return s.WaitForReceiptCommon(shardId, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsCommitted()
	})
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

// Deploy contract to specific shard
func (s *RpcSuite) deployContractViaWallet(
	addrFrom types.Address, key *ecdsa.PrivateKey, shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := s.Client.SendMessageViaWallet(addrFrom, types.Code{}, s.GasToValue(100_000), initialAmount,
		[]types.CurrencyBalance{}, contractAddr, key)
	s.Require().NoError(err)
	receipt := s.WaitForReceipt(addrFrom.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)

	txHash, addr, err := s.Client.DeployContract(shardId, addrFrom, payload, types.Value{}, key)
	s.Require().NoError(err)
	s.Require().Equal(contractAddr, addr)

	receipt = s.WaitIncludedInMain(addrFrom.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)
	return addr, receipt
}

func (s *RpcSuite) DeployContractViaMainWallet(shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	return s.deployContractViaWallet(types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
}

func (s *RpcSuite) SendMessageViaWallet(addrFrom types.Address, addrTo types.Address, key *ecdsa.PrivateKey,
	calldata []byte,
) *jsonrpc.RPCReceipt {
	s.T().Helper()

	txHash, err := s.Client.SendMessageViaWallet(addrFrom, calldata, s.GasToValue(1_000_000), types.NewZeroValue(),
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

	txHash, err := s.Client.SendExternalMessage(bytecode, contractAddress, execution.MainPrivateKey, s.GasToValue(500_000))
	s.Require().NoError(err)

	receipt := s.WaitIncludedInMain(contractAddress.ShardId(), txHash)

	return receipt
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

	seqno, err := s.Client.GetTransactionCount(addr, blockId)
	s.Require().NoError(err)

	log.Debug().Str("contract", addr.String()).Uint64("seqno", uint64(seqno)).Msg("sending external message getter")

	callArgs := &jsonrpc.CallArgs{
		Data:      (*hexutil.Bytes)(&calldata),
		To:        addr,
		FeeCredit: s.GasToValue(100_000_000),
		Seqno:     seqno,
	}
	res, err := s.Client.Call(callArgs, blockId, overrides)
	s.Require().NoError(err)
	s.Require().Empty(res.Error)
	return res.Data
}

func (s *RpcSuite) RunCli(args ...string) string {
	s.T().Helper()

	data, err := s.RunCliNoCheck(args...)
	s.Require().NoErrorf(err, data)
	return data
}

func (s *RpcSuite) RunCliNoCheck(args ...string) (string, error) {
	s.T().Helper()

	if s.TmpDir == "" {
		s.TmpDir = s.T().TempDir()
	}

	binPath := s.TmpDir + "/nil.bin"
	if _, err := os.Stat(binPath); err != nil {
		mainPath := common.GetAbsolutePath("../../cmd/nil/main.go")
		cmd := exec.Command("go", "build", "-o", binPath, mainPath)
		s.NoError(cmd.Run())
	}

	cmd := exec.Command(binPath, args...)
	data, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(data)), err
}

func (s *RpcSuite) LoadContract(path string, name string) (types.Code, abi.ABI) {
	s.T().Helper()

	contracts, err := solc.CompileSource(path)
	s.Require().NoError(err)
	code := hexutil.FromHex(contracts[name].Code)
	abi := solc.ExtractABI(contracts[name])
	return code, abi
}

func (s *RpcSuite) GetBalance(address types.Address) types.Value {
	s.T().Helper()
	balance, err := s.Client.GetBalance(address, "latest")
	s.Require().NoError(err)
	return balance
}

func (s *RpcSuite) GetContract(address types.Address) *types.SmartContract {
	s.T().Helper()

	tx, err := s.Db.CreateRwTx(s.Context)
	s.Require().NoError(err)
	defer tx.Rollback()

	block, _, err := db.ReadLastBlock(tx, address.ShardId())
	s.Require().NoError(err)

	contractTree := execution.NewDbContractTrie(tx, address.ShardId())
	contractTree.SetRootHash(block.SmartContractsRoot)

	contract, err := contractTree.Fetch(address.Hash())
	s.Require().NoError(err)
	return contract
}

func (s *RpcSuite) AbiPack(abi *abi.ABI, name string, args ...any) []byte {
	s.T().Helper()
	data, err := abi.Pack(name, args...)
	s.Require().NoError(err)
	return data
}

func (s *RpcSuite) AbiUnpack(abi *abi.ABI, name string, data []byte) []interface{} {
	s.T().Helper()
	res, err := abi.Unpack(name, data)
	s.Require().NoError(err)
	return res
}

func (s *RpcSuite) PrepareDefaultDeployPayload(abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	s.T().Helper()

	constructor, err := abi.Pack("", args...)
	s.Require().NoError(err)
	code = append(code, constructor...)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

func CheckContractValueEqual[T any](s *RpcSuite, inAbi *abi.ABI, address types.Address, name string, value T) {
	s.T().Helper()

	data := s.AbiPack(inAbi, name)
	data = s.CallGetter(address, data, "latest", nil)
	nameRes, err := inAbi.Unpack(name, data)
	s.Require().NoError(err)
	gotValue, ok := nameRes[0].(T)
	s.Require().True(ok)
	s.Require().Equal(value, gotValue)
}

func (s *RpcSuite) GasToValue(gas uint64) types.Value {
	return types.Gas(gas).ToValue(types.DefaultGasPrice)
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

// AnalyzeReceipt analyzes the receipt and returns the receipt info.
func (s *RpcSuite) AnalyzeReceipt(receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	s.T().Helper()
	res := make(ReceiptInfo)
	s.AnalyzeReceiptRec(receipt, res, namesMap)
	return res
}

func (s *RpcSuite) GetRandomBytes(size int) []byte {
	s.T().Helper()
	data := make([]byte, size)
	_, err := rand.Read(data)
	s.Require().NoError(err)
	return data
}

// AnalyzeReceiptRec is a recursive function that analyzes the receipt and fills the receipt info.
func (s *RpcSuite) AnalyzeReceiptRec(receipt *jsonrpc.RPCReceipt, valuesMap ReceiptInfo, namesMap map[types.Address]string) {
	s.T().Helper()

	value := getContractInfo(receipt.ContractAddress, valuesMap)
	if namesMap != nil {
		value.Name = namesMap[receipt.ContractAddress]
	}

	if receipt.Success {
		value.NumSuccess += 1
	} else {
		value.NumFail += 1
	}
	msg, err := s.Client.GetInMessageByHash(receipt.ShardId, receipt.MsgHash)
	s.Require().NoError(err)

	value.ValueUsed = value.ValueUsed.Add(receipt.GasUsed.ToValue(receipt.GasPrice))
	value.ValueForwarded = value.ValueForwarded.Add(receipt.Forwarded)
	caller := getContractInfo(msg.From, valuesMap)

	if msg.Flags.GetBit(types.MessageFlagInternal) {
		caller.OutMessages[receipt.ContractAddress] = msg
	}

	switch {
	case msg.Flags.GetBit(types.MessageFlagBounce):
		value.BounceReceived = value.BounceReceived.Add(msg.Value)
		// Bounce message also bears refunded gas. If `To` address is equal to `RefundTo`, fee should be credited to
		// this account.
		if msg.To == msg.RefundTo {
			value.RefundReceived = value.RefundReceived.Add(msg.FeeCredit).Sub(receipt.GasUsed.ToValue(receipt.GasPrice))
		}
		// Remove the gas used by bounce message from the sent value
		value.ValueSent = value.ValueSent.Sub(receipt.GasUsed.ToValue(receipt.GasPrice))

		caller.BounceSent = caller.BounceSent.Add(msg.Value)
	case msg.Flags.GetBit(types.MessageFlagRefund):
		value.RefundReceived = value.RefundReceived.Add(msg.Value)
		caller.RefundSent = caller.RefundSent.Add(msg.Value)
	default:
		// Receive value only if message was successful.
		if receipt.Success {
			value.ValueReceived = value.ValueReceived.Add(msg.Value)
		}
		caller.ValueSent = caller.ValueSent.Add(msg.Value)
		// For internal message we need to add gas limit to sent value
		if msg.Flags.GetBit(types.MessageFlagInternal) {
			caller.ValueSent = caller.ValueSent.Add(msg.FeeCredit)
		}
	}

	for _, outReceipt := range receipt.OutReceipts {
		s.AnalyzeReceiptRec(outReceipt, valuesMap, namesMap)
	}
}

func (s *RpcSuite) CheckBalance(infoMap ReceiptInfo, balance types.Value, accounts []types.Address) types.Value {
	s.T().Helper()

	newBalance := types.NewZeroValue()

	for _, addr := range accounts {
		newBalance = newBalance.Add(s.GetBalance(addr))
	}

	newRealBalance := newBalance

	for _, info := range infoMap {
		newBalance = newBalance.Add(info.ValueUsed)
	}
	s.Require().Equal(balance, newBalance)

	return newRealBalance
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
