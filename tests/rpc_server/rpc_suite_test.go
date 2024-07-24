package rpctest

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/NilFoundation/nil/client"
	rpc_client "github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/rs/zerolog"
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
	txHash, err := s.client.SendMessageViaWallet(addrFrom, types.Code{}, 100_000, initialAmount,
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

	txHash, err := s.client.SendMessageViaWallet(addrFrom, calldata, 10_000_000, value,
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

	txHash, err := s.client.SendExternalMessage(bytecode, contractAddress, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(contractAddress.ShardId(), txHash)
	s.Require().True(receipt.Success)
	s.Require().Equal("Success", receipt.Status)
	s.Require().Len(receipt.OutReceipts, 1)

	return receipt
}

// sendMessageViaWalletNoCheck sends a message via a wallet contract. Doesn't require the receipt be successful.
func (s *RpcSuite) sendMessageViaWalletNoCheck(addrWallet types.Address, addrTo types.Address, key *ecdsa.PrivateKey,
	calldata []byte, gas types.Gas, value types.Value, currencies []types.CurrencyBalance,
) *jsonrpc.RPCReceipt {
	s.T().Helper()

	// Send the raw transaction
	txHash, err := s.client.SendMessageViaWallet(addrWallet, calldata, gas, value, currencies, addrTo, key)
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

func (s *RpcSuite) CallGetter(addr types.Address, callData []byte) []byte {
	s.T().Helper()

	callerSeqno, err := s.client.GetTransactionCount(addr, "latest")
	s.Require().NoError(err)
	seqno := hexutil.Uint64(callerSeqno)

	callArgs := &jsonrpc.CallArgs{
		From:     addr,
		Data:     callData,
		To:       addr,
		GasLimit: 10000,
		Seqno:    seqno,
	}
	res, err := s.client.Call(callArgs)
	s.Require().NoError(err)
	return []byte(res)
}

func (s *RpcSuite) runCli(args ...string) string {
	s.T().Helper()

	data, err := s.runCliNoCheck(args...)
	s.Require().NoErrorf(err, data)
	return data
}

func (s *RpcSuite) runCliNoCheck(args ...string) (string, error) {
	s.T().Helper()

	mainPath, err := filepath.Abs("../../cmd/nil_cli/main.go")
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
