package rpctest

import (
	"context"
	"crypto/ecdsa"
	"time"

	"github.com/NilFoundation/nil/client"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/stretchr/testify/suite"
)

type RpcSuite struct {
	suite.Suite
	port      int
	context   context.Context
	cancel    context.CancelFunc
	address   types.Address
	client    client.Client
	shardsNum int
}

func init() {
	logging.SetupGlobalLogger()
}

func (suite *RpcSuite) waitForReceipt(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	suite.Require().Eventually(func() bool {
		receipt, err = suite.client.GetInMessageReceipt(shardId, hash)
		suite.Require().NoError(err)
		return receipt.IsComplete()
	}, 15*time.Second, 200*time.Millisecond)

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
func (suite *RpcSuite) deployContractViaWallet(addrFrom types.Address, key *ecdsa.PrivateKey, shardId types.ShardId, code []byte) (types.Address, *jsonrpc.RPCReceipt) {
	suite.T().Helper()

	txHash, addr, err := suite.client.DeployContract(shardId, addrFrom, code, key)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)
	return addr, receipt
}

func (suite *RpcSuite) deployContractViaMainWallet(shardId types.ShardId, code []byte) (types.Address, *jsonrpc.RPCReceipt) {
	return suite.deployContractViaWallet(types.MainWalletAddress, execution.MainPrivateKey, shardId, code)
}

func (suite *RpcSuite) sendMessageViaWallet(addrFrom types.Address, addrTo types.Address, key *ecdsa.PrivateKey, calldata []byte) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	txHash, err := suite.client.SendMessageViaWallet(addrFrom, calldata, addrTo, key)
	suite.Require().NoError(err)

	receipt := suite.waitForReceipt(addrFrom.ShardId(), txHash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)

	return receipt
}
