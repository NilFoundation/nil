package rpctest

import (
	"context"
	"crypto/ecdsa"
	"fmt"
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
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type RpcSuite struct {
	suite.Suite
	context   context.Context
	cancel    context.CancelFunc
	client    client.Client
	shardsNum int
}

func init() {
	logging.SetupGlobalLogger("info")
}

func (suite *RpcSuite) start(cfg *nilservice.Config) {
	suite.T().Helper()

	suite.shardsNum = cfg.NShards
	suite.context, suite.cancel = context.WithCancel(context.Background())

	badger, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.client = rpc_client.NewClient(fmt.Sprintf("http://127.0.0.1:%d/", cfg.HttpPort))
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
	addrFrom types.Address, key *ecdsa.PrivateKey, shardId types.ShardId, payload types.DeployPayload, initialAmount *types.Uint256,
) (types.Address, *jsonrpc.RPCReceipt) {
	suite.T().Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	suite.T().Logf("Deploying contract %v", contractAddr)
	txHash, err := suite.client.SendMessageViaWallet(addrFrom, types.Code{}, types.NewUint256(100_000), initialAmount,
		[]types.CurrencyBalance{}, contractAddr, key)
	suite.Require().NoError(err)
	receipt := suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)

	txHash, addr, err := suite.client.DeployContract(shardId, addrFrom, payload, nil, key)
	suite.Require().NoError(err)
	suite.Require().Equal(contractAddr, addr)

	receipt = suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)
	return addr, receipt
}

func (suite *RpcSuite) deployContractViaMainWallet(shardId types.ShardId, payload types.DeployPayload, initialAmount *types.Uint256) (types.Address, *jsonrpc.RPCReceipt) {
	suite.T().Helper()

	return suite.deployContractViaWallet(types.MainWalletAddress, execution.MainPrivateKey, shardId, payload, initialAmount)
}

func (suite *RpcSuite) sendMessageViaWallet(addrFrom types.Address, addrTo types.Address, key *ecdsa.PrivateKey, calldata []byte, value *types.Uint256) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	txHash, err := suite.client.SendMessageViaWallet(addrFrom, calldata, types.NewUint256(100_000), value,
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
	calldata []byte, gas *uint256.Int, value *uint256.Int, currencies []types.CurrencyBalance,
) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	// Send the raw transaction
	txHash, err := suite.client.SendMessageViaWallet(addrWallet, calldata, &types.Uint256{Int: *gas},
		&types.Uint256{Int: *value}, currencies, addrTo, key)
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
		Value:    types.NewUint256(0),
		GasLimit: types.NewUint256(10000),
		Seqno:    &seqno,
	}
	res, err := suite.client.Call(callArgs)
	suite.Require().NoError(err)
	return []byte(res)
}
