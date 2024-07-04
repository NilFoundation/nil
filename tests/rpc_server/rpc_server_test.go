package rpctest

import (
	"encoding/json"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cli/service"
	rpc_client "github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	RpcSuite
	cli *service.Service
}

func (suite *SuiteRpc) SetupTest() {
	suite.start(&nilservice.Config{
		NShards:              5,
		HttpPort:             8531,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	})

	suite.cli = service.NewService(suite.client, execution.MainPrivateKey)
	suite.Require().NotNil(suite.cli)
}

func (suite *SuiteRpc) TearDownTest() {
	suite.cancel()
}

func (s *SuiteRpc) sendRawTransaction(m *types.ExternalMessage) common.Hash {
	s.T().Helper()

	hash, err := s.client.SendMessage(m)
	s.Require().NoError(err)
	s.Equal(hash, m.Hash())
	return hash
}

func (suite *SuiteRpc) waitForReceiptOnShard(shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	suite.Require().Eventually(func() bool {
		receipt, err = suite.client.GetInMessageReceipt(shardId, hash)
		suite.Require().NoError(err)
		return receipt.IsComplete()
	}, 15*time.Second, 200*time.Millisecond)

	suite.Equal(hash, receipt.MsgHash)

	return receipt
}

func (s *SuiteRpc) TestRpcBasic() {
	var someRandomMissingBlock common.Hash
	s.Require().NoError(someRandomMissingBlock.UnmarshalText([]byte("0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")))

	shardIdListRes, err := s.client.GetShardIdList()
	s.Require().NoError(err)
	shardIdListExp := make([]types.ShardId, s.shardsNum-1)
	for i := range shardIdListExp {
		shardIdListExp[i] = types.ShardId(i + 1)
	}
	s.Require().Equal(shardIdListExp, shardIdListRes)

	gasPrice, err := s.client.GasPrice(types.BaseShardId)
	s.Require().NoError(err)
	s.Require().Equal(types.NewValueFromUint64(10), gasPrice)

	res0Num, err := s.client.GetBlock(types.BaseShardId, 0, false)
	s.Require().NoError(err)
	s.Require().NotNil(res0Num)

	res0Str, err := s.client.GetBlock(types.BaseShardId, "0", false)
	s.Require().NoError(err)
	s.Require().NotNil(res0Num)
	s.Equal(res0Num, res0Str)

	res, err := s.client.GetBlock(types.BaseShardId, transport.BlockNumber(0x1b4), false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	count, err := s.client.GetBlockTransactionCount(types.BaseShardId, transport.EarliestBlockNumber)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	count, err = s.client.GetBlockTransactionCount(types.BaseShardId, someRandomMissingBlock)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	res, err = s.client.GetBlock(types.BaseShardId, someRandomMissingBlock, false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	res, err = s.client.GetBlock(types.BaseShardId, transport.EarliestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	latest, err := s.client.GetBlock(types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	res, err = s.client.GetBlock(types.BaseShardId, latest.Hash, false)
	s.Require().NoError(err)
	s.Require().Equal(latest, res)

	msg, err := s.client.GetInMessageByHash(types.BaseShardId, someRandomMissingBlock)
	s.Require().NoError(err)
	s.Require().Nil(msg)
}

func (suite *SuiteRpc) loadContract(path string, name string) (types.Code, abi.ABI) {
	suite.T().Helper()

	contracts, err := solc.CompileSource(path)
	suite.Require().NoError(err)
	code := hexutil.FromHex(contracts[name].Code)
	abi := solc.ExtractABI(contracts[name])
	return code, abi
}

func (s *RpcSuite) prepareDefaultDeployPayload(abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	s.T().Helper()

	constructor, err := abi.Pack("", args...)
	s.Require().NoError(err)
	code = append(code, constructor...)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

var defaultContractValue = types.NewValueFromUint64(50_000_000)

func (suite *SuiteRpc) TestRpcContract() {
	contractCode, abi := suite.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployPayload := suite.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	addr, receipt := suite.deployContractViaMainWallet(types.BaseShardId, deployPayload, defaultContractValue)
	suite.Require().True(receipt.OutReceipts[0].Success)

	blockNumber := transport.LatestBlockNumber
	balance, err := suite.client.GetBalance(addr, transport.BlockNumberOrHash{BlockNumber: &blockNumber})
	suite.Require().NoError(err)
	suite.Require().Equal(defaultContractValue, balance)

	// now call (= send a message to) created contract
	calldata, err := abi.Pack("increment")
	suite.Require().NoError(err)

	receipt = suite.sendMessageViaWallet(types.MainWalletAddress, addr, execution.MainPrivateKey, calldata, types.Value{})
	suite.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcDeployToMainShardViaMainWallet() {
	code, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployPayload := s.prepareDefaultDeployPayload(abi, code, big.NewInt(0))

	txHash, _, err := s.client.DeployContract(types.MasterShardId, types.MainWalletAddress, deployPayload, types.Value{}, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(types.MainWalletAddress.ShardId(), txHash)
	s.True(receipt.Success)
}

func (s *SuiteRpc) TestRpcContractSendMessage() {
	// deploy caller contract
	callerCode, callerAbi := s.loadContract(common.GetAbsolutePath("./contracts/async_call.sol"), "Caller")
	calleeCode, calleeAbi := s.loadContract(common.GetAbsolutePath("./contracts/async_call.sol"), "Callee")
	callerAddr, receipt := s.deployContractViaMainWallet(types.MasterShardId, types.BuildDeployPayload(callerCode, common.EmptyHash), defaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	waitTilBalanceAtLeast := func(balance uint64) types.Value {
		s.T().Helper()

		var curBalance types.Value
		s.Require().Eventually(func() bool {
			var err error
			curBalance, err = s.client.GetBalance(callerAddr, transport.LatestBlockNumber)
			s.Require().NoError(err)
			return curBalance.Uint64() > balance
		}, time.Minute, 200*time.Millisecond)
		return curBalance
	}

	checkForShard := func(shardId types.ShardId) {
		s.T().Helper()

		prevBalance, err := s.client.GetBalance(callerAddr, transport.LatestBlockNumber)
		s.Require().NoError(err)
		var callValue uint64 = 10_000_000

		s.Run("FailedDeploy", func() {
			// no account at address to pay for the message
			hash, _, err := s.client.DeployExternal(shardId, types.BuildDeployPayload(calleeCode, common.EmptyHash))
			s.Require().NoError(err)

			receipt := s.waitForReceipt(shardId, hash)
			s.False(receipt.Success)
			s.True(receipt.Temporary)
			s.Equal("no account at address to pay fees", receipt.ErrorMessage)
		})

		var calleeAddr types.Address
		s.Run("DeployCallee", func() {
			// deploy callee contracts to different shards
			calleeAddr, receipt = s.deployContractViaMainWallet(shardId, types.BuildDeployPayload(calleeCode, common.EmptyHash), defaultContractValue)
			s.Require().True(receipt.Success)
			s.Require().True(receipt.OutReceipts[0].Success)
		})

		var callData []byte

		generateAddCallData := func(val int32) {
			// pack call of Callee::add into message
			callData, err = calleeAbi.Pack("add", val)
			s.Require().NoError(err)

			messageToSend := &types.InternalMessagePayload{
				Data:     callData,
				To:       calleeAddr,
				RefundTo: callerAddr,
				BounceTo: callerAddr,
				Value:    types.NewValueFromUint64(callValue),
				GasLimit: 100_004,
			}
			callData, err = messageToSend.MarshalSSZ()
			s.Require().NoError(err)

			// now call Caller::send_message
			callData, err = callerAbi.Pack("sendMessage", callData)
			s.Require().NoError(err)
		}

		s.Run("GenerateCallData", func() {
			generateAddCallData(123)
		})

		var hash common.Hash
		makeCall := func() {
			callerSeqno, err := s.client.GetTransactionCount(callerAddr, "latest")
			s.Require().NoError(err)
			callCallerMethod := &types.ExternalMessage{
				Seqno: callerSeqno,
				To:    callerAddr,
				Data:  callData,
			}
			s.Require().NoError(callCallerMethod.Sign(execution.MainPrivateKey))
			hash = s.sendRawTransaction(callCallerMethod)
		}

		s.Run("MakeCall", makeCall)

		s.Run("Check", func() {
			receipt = s.waitForReceipt(callerAddr.ShardId(), hash)
			s.Require().True(receipt.Success)

			balance, err := s.client.GetBalance(callerAddr, transport.BlockNumberOrHash{BlockHash: &receipt.BlockHash})
			s.Require().NoError(err)
			s.Require().Greater(prevBalance.Uint64(), balance.Uint64())
			s.T().Logf("Spent %v nil", prevBalance.Uint64()-balance.Uint64())

			// we should get some non-zero refund
			prevBalance = waitTilBalanceAtLeast(prevBalance.Uint64() - callValue)
		})

		s.Run("GenerateCallDataBounce", func() {
			generateAddCallData(0)
		})
		s.Run("MakeCallBounce", makeCall)
		s.Run("CheckBounce", func() {
			receipt = s.waitForReceipt(callerAddr.ShardId(), hash)
			s.Require().True(receipt.Success)

			getBounceErrName := "get_bounce_err"

			callData, err := callerAbi.Pack(getBounceErrName)
			s.Require().NoError(err)

			callerSeqno, err := s.client.GetTransactionCount(callerAddr, "latest")
			s.Require().NoError(err)
			seqno := hexutil.Uint64(callerSeqno)

			callArgs := &jsonrpc.CallArgs{
				From:     callerAddr,
				Data:     callData,
				To:       callerAddr,
				GasLimit: 10000,
				Seqno:    seqno,
			}
			res, err := s.client.Call(callArgs)
			s.T().Logf("Call res : %v, err: %v", res, err)
			s.Require().NoError(err)
			var bounceErr string
			s.Require().NoError(callerAbi.UnpackIntoInterface(&bounceErr, getBounceErrName, hexutil.FromHex(res)))
			s.Require().Equal(vm.ErrExecutionReverted.Error(), bounceErr)

			waitTilBalanceAtLeast(prevBalance.Uint64() - callValue)
		})
	}

	s.Run("ToNeighborShard", func() {
		checkForShard(types.ShardId(4))
	})

	s.Run("ToSameShard", func() {
		checkForShard(types.BaseShardId)
	})
}

func (s *SuiteRpc) TestRpcApiModules() {
	res, err := s.client.RawCall("rpc_modules")
	s.Require().NoError(err)

	var data map[string]any
	s.Require().NoError(json.Unmarshal(res, &data))
	s.Equal("1.0", data["eth"])
	s.Equal("1.0", data["rpc"])
}

func (s *SuiteRpc) TestUnsupportedClientVersion() {
	s.Run("Unsupported version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.endpoint, map[string]string{"User-Agent": "nil-cli/12"})
		_, err := client.ChainId()
		s.Require().ErrorContains(err, "unexpected status code: 426: specified revision 12, minimum supported is")
	})

	s.Run("0 means unknown - skip check", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.endpoint, map[string]string{"User-Agent": "nil-cli/0"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})

	s.Run("Valid revision", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.endpoint, map[string]string{"User-Agent": "nil-cli/10000000"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})
}

func (s *SuiteRpc) TestEmptyDeployPayload() {
	wallet := types.MainWalletAddress

	// Deploy contract with invalid payload
	hash, _, err := s.client.DeployContract(types.BaseShardId, wallet, types.DeployPayload{}, types.Value{}, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceiptOnShard(wallet.ShardId(), hash)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcError() {
	check := func(code int, msg, method string, params ...any) {
		resp, err := s.client.RawCall(method, params...)
		s.Require().ErrorContains(err, strconv.Itoa(code))
		s.Require().ErrorContains(err, msg)
		s.Require().Nil(resp)
	}

	check(-32601, "the method eth_doesntExist does not exist/is not available",
		"eth_doesntExist")

	check(-32602, "missing value for required argument 0",
		rpc_client.Eth_getBlockByNumber)

	check(-32602, "invalid argument 0: json: cannot unmarshal number 1099511627776 into Go value of type uint32",
		rpc_client.Eth_getBlockByNumber, 1<<40)

	check(-32602, "missing value for required argument 1",
		rpc_client.Eth_getBlockByNumber, types.BaseShardId)

	check(-32602, "invalid argument 1: hex string of odd length",
		rpc_client.Eth_getBlockByHash, types.BaseShardId, "0x1b4", false)

	check(-32602, "invalid argument 1: hex string without 0x prefix",
		rpc_client.Eth_getBlockByHash, types.BaseShardId, "latest")
}

func (suite *SuiteRpc) TestRpcDebugModules() {
	raw, err := suite.client.RawCall("debug_getBlockByNumber", types.BaseShardId, "latest", false)
	suite.Require().NoError(err)

	var res map[string]any
	suite.Require().NoError(json.Unmarshal(raw, &res))

	suite.Require().Contains(res, "number")
	suite.Require().Contains(res, "hash")
	suite.Require().Contains(res, "content")

	sliceContent, ok := res["content"].(string)
	suite.Require().True(ok)
	// check if the string starts with 0x prefix
	suite.Require().Equal("0x", sliceContent[:2])
	// print resp to see the result
	suite.T().Logf("resp: %v", res)

	raw, err = suite.client.RawCall("debug_getBlockByNumber", types.BaseShardId, "latest", true)

	var fullRes map[string]any
	suite.Require().NoError(json.Unmarshal(raw, &fullRes))
	suite.Require().NoError(err)
	suite.Require().Contains(fullRes, "content")
	suite.Require().Contains(fullRes, "inMessages")
	suite.Require().Contains(fullRes, "outMessages")
	suite.Require().Contains(fullRes, "receipts")
}

// Test that we remove output messages if the transaction failed
func (suite *SuiteRpc) TestNoOutMessagesIfFailure() {
	code, err := contracts.GetCode(contracts.NameCommonTest)
	suite.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameCommonTest)
	suite.Require().NoError(err)

	addr, receipt := suite.deployContractViaMainWallet(2, types.BuildDeployPayload(code, common.EmptyHash), defaultContractValue)
	suite.Require().True(receipt.OutReceipts[0].Success)

	// Call CommonTest contract with invalid argument, so no output messages should be generated
	calldata, err := abi.Pack("testFailedAsyncCall", addr, int32(0))
	suite.Require().NoError(err)

	txhash, err := suite.client.SendExternalMessage(calldata, addr, nil)
	suite.Require().NoError(err)
	receipt = suite.waitForReceipt(addr.ShardId(), txhash)
	suite.Require().False(receipt.Success)
	suite.Require().Empty(receipt.OutReceipts)
	suite.Require().Empty(receipt.OutMessages)

	// Call CommonTest contract with valid argument, so output messages should be generated
	calldata, err = abi.Pack("testFailedAsyncCall", addr, int32(10))
	suite.Require().NoError(err)

	txhash, err = suite.client.SendExternalMessage(calldata, addr, nil)
	suite.Require().NoError(err)
	receipt = suite.waitForReceipt(addr.ShardId(), txhash)
	suite.Require().True(receipt.Success)
	suite.Require().Len(receipt.OutReceipts, 1)
	suite.Require().Len(receipt.OutMessages, 1)
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
