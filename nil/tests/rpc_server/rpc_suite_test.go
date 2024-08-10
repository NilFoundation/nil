package rpctest

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/NilFoundation/nil/nil/client"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/suite"
)

type RpcSuite struct {
	suite.Suite

	context   context.Context
	ctxCancel context.CancelFunc
	wg        sync.WaitGroup

	dbInit func() db.DB
	db     db.DB

	client    client.Client
	shardsNum int
	endpoint  string
}

func init() {
	logging.SetupGlobalLogger("debug")
}

func (s *RpcSuite) start(cfg *nilservice.Config) {
	s.T().Helper()

	s.shardsNum = cfg.NShards
	s.context, s.ctxCancel = context.WithCancel(context.Background())

	if s.dbInit == nil {
		s.dbInit = func() db.DB {
			db, err := db.NewBadgerDbInMemory()
			s.Require().NoError(err)
			return db
		}
	}
	s.db = s.dbInit()

	var serviceInterop chan nilservice.ServiceInterop
	if cfg.RunMode == nilservice.CollatorsOnlyRunMode {
		serviceInterop = make(chan nilservice.ServiceInterop, 1)
	}

	s.wg.Add(1)
	go func() {
		nilservice.Run(s.context, cfg, s.db, serviceInterop)
		s.wg.Done()
	}()

	if cfg.RunMode == nilservice.CollatorsOnlyRunMode {
		service := <-serviceInterop
		client, err := client.NewEthClient(s.context, s.db, service.MsgPools, zerolog.New(os.Stderr))
		s.Require().NoError(err)
		s.client = client
	} else {
		s.endpoint = fmt.Sprintf("http://127.0.0.1:%d", cfg.HttpPort)
		s.client = rpc_client.NewClient(s.endpoint, zerolog.New(os.Stderr))
	}

	if cfg.RunMode == nilservice.NormalRunMode || cfg.RunMode == nilservice.CollatorsOnlyRunMode {
		s.waitZerostate()
	} else {
		s.Require().Eventually(func() bool {
			block, err := s.client.GetBlock(cfg.ReplayShardId, "latest", false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}

func (s *RpcSuite) cancel() {
	s.T().Helper()

	s.ctxCancel()
	s.wg.Wait()
	s.db.Close()
}

func (s *RpcSuite) waitForReceipt(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	s.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	s.Require().Eventually(func() bool {
		receipt, err = s.client.GetInMessageReceipt(shardId, hash)
		s.Require().NoError(err)
		return receipt.IsComplete()
	}, ReceiptWaitTimeout, ReceiptPollInterval)

	s.Equal(hash, receipt.MsgHash)

	return receipt
}

func (s *RpcSuite) waitZerostate() {
	s.T().Helper()
	for i := range s.shardsNum {
		s.Require().Eventually(func() bool {
			block, err := s.client.GetBlock(types.ShardId(i), transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, ZeroStateWaitTimeout, ZeroStatePollInterval)
	}
}

// Deploy contract to specific shard
func (s *RpcSuite) deployContractViaWallet(
	addrFrom types.Address, key *ecdsa.PrivateKey, shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := s.client.SendMessageViaWallet(addrFrom, types.Code{}, s.gasToValue(100_000), initialAmount,
		[]types.CurrencyBalance{}, contractAddr, key)
	s.Require().NoError(err)
	receipt := s.waitForReceipt(addrFrom.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)

	txHash, addr, err := s.client.DeployContract(shardId, addrFrom, payload, types.Value{}, key)
	s.Require().NoError(err)
	s.Require().Equal(contractAddr, addr)

	receipt = s.waitForReceipt(addrFrom.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)
	return addr, receipt
}

func (s *RpcSuite) deployContractViaMainWallet(shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value) (types.Address, *jsonrpc.RPCReceipt) {
	s.T().Helper()

	return s.deployContractViaWallet(types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
}

func (s *RpcSuite) sendMessageViaWallet(addrFrom types.Address, addrTo types.Address, key *ecdsa.PrivateKey, calldata []byte, value types.Value) *jsonrpc.RPCReceipt {
	s.T().Helper()

	txHash, err := s.client.SendMessageViaWallet(addrFrom, calldata, s.gasToValue(10_000_000), value,
		[]types.CurrencyBalance{}, addrTo, key)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(addrFrom.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)

	return receipt
}

func (s *RpcSuite) sendExternalMessage(bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	s.T().Helper()

	receipt := s.sendExternalMessageNoCheck(bytecode, contractAddress)

	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)

	return receipt
}

func (s *RpcSuite) sendExternalMessageNoCheck(bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	s.T().Helper()

	txHash, err := s.client.SendExternalMessage(bytecode, contractAddress, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(contractAddress.ShardId(), txHash)

	return receipt
}

// sendMessageViaWalletNoCheck sends a message via a wallet contract. Doesn't require the receipt be successful.
func (s *RpcSuite) sendMessageViaWalletNoCheck(addrWallet types.Address, addrTo types.Address, key *ecdsa.PrivateKey,
	calldata []byte, feeCredit types.Value, value types.Value, currencies []types.CurrencyBalance,
) *jsonrpc.RPCReceipt {
	s.T().Helper()

	// Send the raw transaction
	txHash, err := s.client.SendMessageViaWallet(addrWallet, calldata, feeCredit, value, currencies,
		addrTo, key)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(addrWallet.ShardId(), txHash)
	// We don't check the receipt for success here, as it can be failed on purpose
	if receipt.Success {
		// But if it is successful, we expect exactly one out receipt
		s.Require().Len(receipt.OutReceipts, 1)
	} else {
		s.Require().NotEqual("Success", receipt.Status)
	}

	return receipt
}

func (s *RpcSuite) CallGetter(addr types.Address, callData []byte, blockId any, overrides *jsonrpc.StateOverrides) []byte {
	s.T().Helper()

	seqno, err := s.client.GetTransactionCount(addr, blockId)
	s.Require().NoError(err)

	log.Debug().Str("contract", addr.String()).Uint64("seqno", uint64(seqno)).Msg("sending external message getter")

	callArgs := &jsonrpc.CallArgs{
		Data:      callData,
		To:        addr,
		FeeCredit: s.gasToValue(100_000_000),
		Seqno:     seqno,
	}
	res, err := s.client.Call(callArgs, blockId, overrides)
	s.Require().NoError(err)
	return res.Data
}

func (s *RpcSuite) runCli(args ...string) string {
	s.T().Helper()

	data, err := s.runCliNoCheck(args...)
	s.Require().NoErrorf(err, data)
	return data
}

func (s *RpcSuite) runCliNoCheck(args ...string) (string, error) {
	s.T().Helper()

	mainPath, err := filepath.Abs("../../cmd/nil/main.go")
	s.Require().NoError(err)

	args = append([]string{"run", mainPath}, args...)
	cmd := exec.Command("go", args...)

	data, err := cmd.CombinedOutput()
	return string(data), err
}

func (s *RpcSuite) loadContract(path string, name string) (types.Code, abi.ABI) {
	s.T().Helper()

	contracts, err := solc.CompileSource(path)
	s.Require().NoError(err)
	code := hexutil.FromHex(contracts[name].Code)
	abi := solc.ExtractABI(contracts[name])
	return code, abi
}

func (s *RpcSuite) getBalance(address types.Address) types.Value {
	s.T().Helper()
	balance, err := s.client.GetBalance(address, "latest")
	s.Require().NoError(err)
	return balance
}

func (s *RpcSuite) AbiPack(abi *abi.ABI, name string, args ...any) []byte {
	s.T().Helper()
	data, err := abi.Pack(name, args...)
	s.Require().NoError(err)
	return data
}

func (s *RpcSuite) gasToValue(gas uint64) types.Value {
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

// analyzeReceipt analyzes the receipt and returns the receipt info.
func (s *RpcSuite) analyzeReceipt(receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	s.T().Helper()
	res := make(ReceiptInfo)
	s.analyzeReceiptRec(receipt, res, namesMap)
	return res
}

func (s *RpcSuite) getRandomBytes(size int) []byte {
	s.T().Helper()
	data := make([]byte, size)
	_, err := rand.Read(data)
	s.Require().NoError(err)
	return data
}

// analyzeReceiptRec is a recursive function that analyzes the receipt and fills the receipt info.
func (s *RpcSuite) analyzeReceiptRec(receipt *jsonrpc.RPCReceipt, valuesMap ReceiptInfo, namesMap map[types.Address]string) {
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
	msg, err := s.client.GetInMessageByHash(receipt.ShardId, receipt.MsgHash)
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
		s.analyzeReceiptRec(outReceipt, valuesMap, namesMap)
	}
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
