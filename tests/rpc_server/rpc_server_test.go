package rpctest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	rpc_client "github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	RpcSuite
}

func (s *SuiteRpc) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              5,
		HttpPort:             8530,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	})
}

func (s *SuiteRpc) TearDownTest() {
	s.cancel()
}

func (s *SuiteRpc) sendRawTransaction(m *types.ExternalMessage) common.Hash {
	s.T().Helper()

	hash, err := s.client.SendMessage(m)
	s.Require().NoError(err)
	s.Equal(hash, m.Hash())
	return hash
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

func (s *RpcSuite) prepareDefaultDeployPayload(abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	s.T().Helper()

	constructor, err := abi.Pack("", args...)
	s.Require().NoError(err)
	code = append(code, constructor...)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

var defaultContractValue = types.NewValueFromUint64(50_000_000)

func (s *SuiteRpc) TestRpcContract() {
	contractCode, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployPayload := s.prepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	addr, receipt := s.deployContractViaMainWallet(types.BaseShardId, deployPayload, defaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	blockNumber := transport.LatestBlockNumber
	balance, err := s.client.GetBalance(addr, transport.BlockNumberOrHash{BlockNumber: &blockNumber})
	s.Require().NoError(err)
	s.Require().Equal(defaultContractValue, balance)

	// now call (= send a message to) created contract
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(types.MainWalletAddress, addr, execution.MainPrivateKey, calldata, types.Value{})
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcDeployToMainShardViaMainWallet() {
	code, abi := s.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	deployPayload := s.prepareDefaultDeployPayload(abi, code, big.NewInt(0))

	txHash, _, err := s.client.DeployContract(types.MainShardId, types.MainWalletAddress, deployPayload, types.Value{}, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(types.MainWalletAddress.ShardId(), txHash)
	s.True(receipt.Success)
}

func (s *SuiteRpc) TestRpcContractSendMessage() {
	// deploy caller contract
	callerCode, callerAbi := s.loadContract(common.GetAbsolutePath("./contracts/async_call.sol"), "Caller")
	calleeCode, calleeAbi := s.loadContract(common.GetAbsolutePath("./contracts/async_call.sol"), "Callee")
	callerAddr, receipt := s.deployContractViaMainWallet(types.MainShardId, types.BuildDeployPayload(callerCode, common.EmptyHash), defaultContractValue)
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
				Data:      callData,
				To:        calleeAddr,
				RefundTo:  callerAddr,
				BounceTo:  callerAddr,
				Value:     types.NewValueFromUint64(callValue),
				FeeCredit: s.gasToValue(100_000),
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
				From:      callerAddr,
				Data:      callData,
				To:        callerAddr,
				FeeCredit: s.gasToValue(10000),
				Seqno:     seqno,
			}
			res, err := s.client.Call(callArgs)
			s.T().Logf("Call res : %v, err: %v", res, err)
			s.Require().NoError(err)
			var bounceErr string
			s.Require().NoError(callerAbi.UnpackIntoInterface(&bounceErr, getBounceErrName, res.Data))
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

func (s *SuiteRpc) TestRpcCallWithMessageSend() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	pub := crypto.CompressPubkey(&pk.PublicKey)
	walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
	deployCode := types.BuildDeployPayload(walletCode, common.EmptyHash)

	dstShardId := types.ShardId(2)
	hash, address, err := s.client.DeployContract(
		dstShardId, types.MainWalletAddress, deployCode, types.NewValueFromUint64(500_000), execution.MainPrivateKey,
	)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(types.MainWalletAddress.ShardId(), hash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)
	fmt.Println(address)

	intMsg := &types.InternalMessagePayload{
		Data:        []byte("check"),
		To:          address,
		FeeCredit:   types.NewValueFromUint64(100000),
		ForwardKind: types.ForwardKindNone,
		Kind:        types.ExecutionMessageKind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	s.Require().NoError(err)

	calldata, err := contracts.NewCallData(contracts.NameWallet, "send", intMsgData)
	s.Require().NoError(err)

	callerSeqno, err := s.client.GetTransactionCount(types.MainWalletAddress, "latest")
	s.Require().NoError(err)
	seqno := hexutil.Uint64(callerSeqno)

	callArgs := &jsonrpc.CallArgs{
		From:      types.MainWalletAddress,
		Data:      calldata,
		To:        types.MainWalletAddress,
		FeeCredit: s.gasToValue(100000000000),
		Seqno:     seqno,
	}

	res, err := s.client.Call(callArgs)
	s.T().Logf("Call res : %v, err: %v", res, err)
	s.Require().NoError(err)
	s.Require().Len(res.OutMessages, 1)

	msg := res.OutMessages[0]
	s.Equal(types.MainWalletAddress, msg.From)
	s.Equal(address, msg.To)
	s.NotEmpty(msg.Data)
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
	logger := zerolog.New(os.Stderr)
	s.Run("Unsupported version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.endpoint, logger, map[string]string{"User-Agent": "nil-cli/12"})
		_, err := client.ChainId()
		s.Require().ErrorContains(err, "unexpected status code: 426: specified revision 12, minimum supported is")
	})

	s.Run("0 means unknown - skip check", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.endpoint, logger, map[string]string{"User-Agent": "nil-cli/0"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})

	s.Run("Valid revision", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.endpoint, logger, map[string]string{"User-Agent": "nil-cli/10000000"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})
}

func (s *SuiteRpc) TestEmptyDeployPayload() {
	wallet := types.MainWalletAddress

	// Deploy contract with invalid payload
	hash, _, err := s.client.DeployContract(types.BaseShardId, wallet, types.DeployPayload{}, types.Value{}, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(wallet.ShardId(), hash)
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

func (s *SuiteRpc) TestRpcDebugModules() {
	res, err := s.client.GetDebugBlock(types.BaseShardId, "latest", false)
	s.Require().NoError(err)

	block, err := res.DecodeHexAndSSZ()
	s.Require().NoError(err)

	s.Require().NotEmpty(block.Id)
	s.Require().NotEqual(common.EmptyHash, block.Block.Hash())
	s.Require().NotEmpty(res.Content)

	sliceContent := res.Content
	// check if the string starts with 0x prefix
	s.Require().Equal("0x", sliceContent[:2])
	// print resp to see the result
	s.T().Logf("resp: %v", res)

	fullRes, err := s.client.GetDebugBlock(types.BaseShardId, "latest", true)
	s.Require().NoError(err)
	s.Require().NotEmpty(fullRes.Content)
	s.Require().Empty(block.InMessages)
	s.Require().Empty(block.OutMessages)
	s.Require().Empty(block.Receipts)
}

// Test that we remove output messages if the transaction failed
func (s *SuiteRpc) TestNoOutMessagesIfFailure() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	addr, receipt := s.deployContractViaMainWallet(2, types.BuildDeployPayload(code, common.EmptyHash), defaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Call Test contract with invalid argument, so no output messages should be generated
	calldata, err := abi.Pack("testFailedAsyncCall", addr, int32(0))
	s.Require().NoError(err)

	txhash, err := s.client.SendExternalMessage(calldata, addr, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(addr.ShardId(), txhash)
	s.Require().False(receipt.Success)
	s.Require().NotEqual("Success", receipt.Status)
	s.Require().Empty(receipt.OutReceipts)
	s.Require().Empty(receipt.OutMessages)

	// Call Test contract with valid argument, so output messages should be generated
	calldata, err = abi.Pack("testFailedAsyncCall", addr, int32(10))
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(calldata, addr, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(addr.ShardId(), txhash)
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().Len(receipt.OutMessages, 1)
}

func (s *SuiteRpc) TestMultipleRefunds() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)

	var leftShardId types.ShardId = 1
	var rightShardId types.ShardId = 2

	_, receipt := s.deployContractViaMainWallet(leftShardId, types.BuildDeployPayload(code, common.EmptyHash), defaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	_, receipt = s.deployContractViaMainWallet(rightShardId, types.BuildDeployPayload(code, common.EmptyHash), defaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcBlockContent() {
	// Deploy message
	hash, _, err := s.client.DeployContract(types.BaseShardId, types.MainWalletAddress,
		contracts.CounterDeployPayload(s.T()), types.Value{},
		execution.MainPrivateKey)
	s.Require().NoError(err)

	var block *jsonrpc.RPCBlock
	s.Eventually(func() bool {
		var err error
		block, err = s.client.GetBlock(types.BaseShardId, "latest", false)
		s.Require().NoError(err)

		return len(block.Messages) > 0
	}, 6*time.Second, 50*time.Millisecond)

	block, err = s.client.GetBlock(types.BaseShardId, block.Hash, true)
	s.Require().NoError(err)

	s.Require().NotNil(block.Hash)
	s.Require().Len(block.Messages, 1)

	msg, ok := block.Messages[0].(map[string]any)
	s.Require().True(ok)
	s.Equal(hash.Hex(), msg["hash"])
}

func (s *SuiteRpc) TestRpcMessageContent() {
	shardId := types.ShardId(3)
	hash, _, err := s.client.DeployContract(shardId, types.MainWalletAddress,
		contracts.CounterDeployPayload(s.T()), types.Value{},
		execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.waitForReceipt(types.MainWalletAddress.ShardId(), hash)

	msg1, err := s.client.GetInMessageByHash(types.MainWalletAddress.ShardId(), hash)
	s.Require().NoError(err)
	s.EqualValues(0, msg1.Flags.Bits)

	msg2, err := s.client.GetInMessageByHash(shardId, receipt.OutMessages[0])
	s.Require().NoError(err)
	s.EqualValues(3, msg2.Flags.Bits)
}

func (s *SuiteRpc) TestDbApi() {
	block, err := s.client.GetBlock(types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)

	s.Require().NoError(s.client.DbInitTimestamp(block.DbTimestamp))

	hBytes, err := s.client.DbGet(db.LastBlockTable, types.BaseShardId.Bytes())
	s.Require().NoError(err)

	h := common.BytesToHash(hBytes)

	s.Require().Equal(block.Hash, h)
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
