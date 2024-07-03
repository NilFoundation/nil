package rpctest

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

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
	"github.com/stretchr/testify/suite"
)

type RpcSuite struct {
	suite.Suite
	context   context.Context
	cancel    context.CancelFunc
	client    client.Client
	shardsNum int
	endpoint  string
}

func init() {
	logging.SetupGlobalLogger("debug")
}

func (suite *RpcSuite) start(cfg *nilservice.Config) {
	suite.T().Helper()

	suite.shardsNum = cfg.NShards
	suite.context, suite.cancel = context.WithCancel(context.Background())

	badger, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.endpoint = fmt.Sprintf("http://127.0.0.1:%d", cfg.HttpPort)
	suite.client = rpc_client.NewClient(suite.endpoint)
	go nilservice.Run(suite.context, cfg, badger)
	suite.waitZerostate()
}

func (suite *RpcSuite) waitForReceipt(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	suite.Require().Eventually(func() bool {
		receipt, err = suite.client.GetInMessageReceipt(shardId, hash)
		suite.Require().NoError(err)
		return receipt.IsComplete()
	}, 15*time.Minute, 200*time.Millisecond)

	return receipt
}

func (suite *RpcSuite) waitZerostate() {
	for i := range suite.shardsNum {
		suite.Require().Eventually(func() bool {
			block, err := suite.client.GetBlock(types.ShardId(i), transport.BlockNumber(0), false)
			suite.Require().NoError(err)
			return block != nil
		}, 10*time.Second, 100*time.Millisecond)
	}
}

// Deploy contract to specific shard
func (suite *RpcSuite) deployContractViaWallet(
	addrFrom types.Address, key *ecdsa.PrivateKey, shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	suite.T().Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	suite.T().Logf("Deploying contract %v", contractAddr)
	txHash, err := suite.client.SendMessageViaWallet(addrFrom, types.Code{}, 100_000, initialAmount,
		[]types.CurrencyBalance{}, contractAddr, key)
	suite.Require().NoError(err)
	receipt := suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)

	txHash, addr, err := suite.client.DeployContract(shardId, addrFrom, payload, types.Value{}, key)
	suite.Require().NoError(err)
	suite.Require().Equal(contractAddr, addr)

	receipt = suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)
	return addr, receipt
}

func (suite *RpcSuite) deployContractViaMainWallet(shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value) (types.Address, *jsonrpc.RPCReceipt) {
	suite.T().Helper()

	return suite.deployContractViaWallet(types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
}

func (suite *RpcSuite) sendMessageViaWallet(addrFrom types.Address, addrTo types.Address, key *ecdsa.PrivateKey, calldata []byte, value types.Value) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	txHash, err := suite.client.SendMessageViaWallet(addrFrom, calldata, 1_000_000, value,
		[]types.CurrencyBalance{}, addrTo, key)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)

	return receipt
}

func (suite *RpcSuite) sendExternalMessage(bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	txHash, err := suite.client.SendExternalMessage(bytecode, contractAddress, execution.MainPrivateKey)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(contractAddress.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)

	return receipt
}

// sendMessageViaWalletNoCheck sends a message via a wallet contract. Doesn't require the receipt be successful.
func (suite *RpcSuite) sendMessageViaWalletNoCheck(addrWallet types.Address, addrTo types.Address, key *ecdsa.PrivateKey,
	calldata []byte, gas types.Gas, value types.Value, currencies []types.CurrencyBalance,
) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	// Send the raw transaction
	txHash, err := suite.client.SendMessageViaWallet(addrWallet, calldata, gas, value, currencies, addrTo, key)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(addrWallet.ShardId(), txHash)
	// We don't check the receipt for success here, as it can be failed on purpose
	if receipt.Success {
		// But if it is successful, we expect exactly one out receipt
		suite.Require().Len(receipt.OutReceipts, 1)
	}

	return receipt
}

func (suite *RpcSuite) CallGetter(addr types.Address, callData []byte) []byte {
	suite.T().Helper()

	callerSeqno, err := suite.client.GetTransactionCount(addr, "latest")
	suite.Require().NoError(err)
	seqno := hexutil.Uint64(callerSeqno)

	callArgs := &jsonrpc.CallArgs{
		From:     addr,
		Data:     callData,
		To:       addr,
		GasLimit: 10000,
		Seqno:    seqno,
	}
	res, err := suite.client.Call(callArgs)
	suite.Require().NoError(err)
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
