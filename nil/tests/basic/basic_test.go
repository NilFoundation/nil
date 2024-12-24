package tests

import (
	"encoding/json"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	ssz "github.com/NilFoundation/fastssz"
	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/params"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	tests.RpcSuite
}

func (s *SuiteRpc) SetupTest() {
	s.Start(&nilservice.Config{
		NShards: 5,
		HttpUrl: rpc.GetSockPath(s.T()),
	})
}

func (s *SuiteRpc) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRpc) sendRawTransaction(m *types.ExternalMessage) common.Hash {
	s.T().Helper()

	hash, err := s.Client.SendMessage(m)
	s.Require().NoError(err)
	s.Equal(hash, m.Hash())
	return hash
}

func (s *SuiteRpc) TestRpcBasic() {
	var someRandomMissingBlock common.Hash
	s.Require().NoError(someRandomMissingBlock.UnmarshalText([]byte("0x0001117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")))

	shardIdListRes, err := s.Client.GetShardIdList()
	s.Require().NoError(err)
	shardIdListExp := make([]types.ShardId, s.ShardsNum-1)
	for i := range shardIdListExp {
		shardIdListExp[i] = types.ShardId(i + 1)
	}
	s.Require().Equal(shardIdListExp, shardIdListRes)

	gasPrice, err := s.Client.GasPrice(types.BaseShardId)
	s.Require().NoError(err)
	s.Require().Equal(types.DefaultGasPrice, gasPrice)

	res0Num, err := s.Client.GetBlock(types.BaseShardId, 0, false)
	s.Require().NoError(err)
	s.Require().NotNil(res0Num)

	res0Str, err := s.Client.GetBlock(types.BaseShardId, "0", false)
	s.Require().NoError(err)
	s.Require().NotNil(res0Num)
	s.Equal(res0Num, res0Str)

	res, err := s.Client.GetBlock(types.BaseShardId, transport.BlockNumber(0x1b4), false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	count, err := s.Client.GetBlockTransactionCount(types.BaseShardId, transport.EarliestBlockNumber)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	count, err = s.Client.GetBlockTransactionCount(types.BaseShardId, someRandomMissingBlock)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	res, err = s.Client.GetBlock(types.BaseShardId, someRandomMissingBlock, false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	res, err = s.Client.GetBlock(types.BaseShardId, transport.EarliestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	latest, err := s.Client.GetBlock(types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	res, err = s.Client.GetBlock(types.BaseShardId, latest.Hash, false)
	s.Require().NoError(err)
	s.Require().Equal(latest, res)

	msg, err := s.Client.GetInMessageByHash(someRandomMissingBlock)
	s.Require().NoError(err)
	s.Require().Nil(msg)
}

func (s *SuiteRpc) TestRpcContract() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	addr, receipt := s.DeployContractViaMainWallet(types.BaseShardId, deployPayload, tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	blockNumber := transport.LatestBlockNumber
	balance, err := s.Client.GetBalance(addr, transport.BlockNumberOrHash{BlockNumber: &blockNumber})
	s.Require().NoError(err)
	s.Require().Equal(tests.DefaultContractValue, balance)

	// now call (= send a message to) created contract
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	receipt = s.SendMessageViaWallet(types.MainWalletAddress, addr, execution.MainPrivateKey, calldata)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcContractSendMessage() {
	// deploy caller contract
	callerCode, callerAbi := s.LoadContract(common.GetAbsolutePath("../contracts/async_call.sol"), "Caller")
	calleeCode, calleeAbi := s.LoadContract(common.GetAbsolutePath("../contracts/async_call.sol"), "Callee")
	callerAddr, receipt := s.DeployContractViaMainWallet(
		types.BaseShardId, types.BuildDeployPayload(callerCode, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	waitTilBalanceAtLeast := func(balance uint64) types.Value {
		s.T().Helper()

		var curBalance types.Value
		s.Require().Eventually(func() bool {
			var err error
			curBalance, err = s.Client.GetBalance(callerAddr, transport.LatestBlockNumber)
			s.Require().NoError(err)
			return curBalance.Uint64() > balance
		}, tests.ReceiptWaitTimeout, 200*time.Millisecond)
		return curBalance
	}

	checkForShard := func(shardId types.ShardId) {
		s.T().Helper()

		s.Run("FailedDeploy", func() {
			// no account at address to pay for the message
			hash, _, err := s.Client.DeployExternal(shardId, types.BuildDeployPayload(calleeCode, common.EmptyHash), s.GasToValue(100_000))
			s.Require().NoError(err)

			receipt := s.WaitForReceipt(hash)
			s.False(receipt.Success)
			s.True(receipt.Temporary)
			s.Equal("DestinationContractDoesNotExist", receipt.Status)
		})

		var calleeAddr types.Address
		s.Run("DeployCallee", func() {
			// deploy callee contracts to different shards
			calleeAddr, receipt = s.DeployContractViaMainWallet(
				shardId, types.BuildDeployPayload(calleeCode, common.EmptyHash), tests.DefaultContractValue)
			s.Require().True(receipt.Success)
			s.Require().True(receipt.OutReceipts[0].Success)
		})

		prevBalance, err := s.Client.GetBalance(callerAddr, transport.LatestBlockNumber)
		s.Require().NoError(err)
		var feeCredit uint64 = 100_000
		var callValue uint64 = 2_000_000
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
				FeeCredit: s.GasToValue(feeCredit),
			}
			callData, err = messageToSend.MarshalSSZ()
			s.Require().NoError(err)

			// now call Caller::send_message
			callData, err = callerAbi.Pack("sendMessage", callData)
			s.Require().NoError(err)
		}

		var hash common.Hash
		makeCall := func() {
			callerSeqno, err := s.Client.GetTransactionCount(callerAddr, "pending")
			s.Require().NoError(err)
			callCallerMethod := &types.ExternalMessage{
				Seqno:     callerSeqno,
				To:        callerAddr,
				Data:      callData,
				FeeCredit: s.GasToValue(feeCredit),
			}
			s.Require().NoError(callCallerMethod.Sign(execution.MainPrivateKey))
			hash = s.sendRawTransaction(callCallerMethod)
		}

		s.Run("GenerateCallData", func() {
			generateAddCallData(123)
		})
		s.Run("MakeCall", makeCall)
		extMessageVerificationFee := uint64(8350)
		s.Run("Check", func() {
			receipt = s.WaitForReceipt(hash)
			s.Require().True(receipt.Success)

			balance, err := s.Client.GetBalance(callerAddr, transport.BlockNumberOrHash{BlockHash: &receipt.BlockHash})
			s.Require().NoError(err)
			s.Require().Greater(prevBalance.Uint64(), balance.Uint64())
			s.T().Logf("Spent %v nil", prevBalance.Uint64()-balance.Uint64())
			// here we spent:
			// - `callValue`, cause we attach that amount of value to internal cross-shard message
			// - `GasToValue(feeCredit)`, cause we buy that amount of gas to send cross-shard message
			// - `GasToValue(feeCredit)`, cause it's set in our ExternalMessage
			// - some amount to verify the ext message. depends on current implementation
			minimalExpectedBalance := prevBalance.Uint64() - 2*s.GasToValue(feeCredit).Uint64() - callValue - extMessageVerificationFee
			s.Require().GreaterOrEqual(balance.Uint64(), minimalExpectedBalance)

			// we should get some non-zero refund
			prevBalance = waitTilBalanceAtLeast(minimalExpectedBalance)
		})

		s.Run("GenerateCallDataBounce", func() {
			generateAddCallData(0)
		})
		s.Run("MakeCallBounce", makeCall)
		s.Run("CheckBounce", func() {
			receipt = s.WaitIncludedInMain(hash)
			s.Require().True(receipt.Success)

			getBounceErrName := "get_bounce_err"

			callData, err := callerAbi.Pack(getBounceErrName)
			s.Require().NoError(err)

			callerSeqno, err := s.Client.GetTransactionCount(callerAddr, "pending")
			s.Require().NoError(err)

			callArgs := &jsonrpc.CallArgs{
				Data:      (*hexutil.Bytes)(&callData),
				To:        callerAddr,
				FeeCredit: s.GasToValue(10000),
				Seqno:     callerSeqno,
			}

			res, err := s.Client.Call(callArgs, "latest", nil)
			s.T().Logf("Call res : %v, err: %v", res, err)
			s.Require().NoError(err)
			var bounceErr string
			s.Require().NoError(callerAbi.UnpackIntoInterface(&bounceErr, getBounceErrName, res.Data))
			s.Require().Equal(vm.ErrExecutionReverted.Error()+": Value must be non-zero", bounceErr)

			s.Require().Len(receipt.OutMessages, 1)
			receipt = s.WaitForReceipt(receipt.OutMessages[0])
			s.Require().False(receipt.Success)
			s.Require().Len(receipt.DebugLogs, 1)
			s.Require().Equal("execution started", receipt.DebugLogs[0].Message)

			// here we spent:
			// - `callValue`, cause we attach that amount of value to internal cross-shard message
			// - `GasToValue(feeCredit)`, cause we buy that amount of gas to send cross-shard message
			// - `GasToValue(feeCredit)`, cause it's set in our ExternalMessage
			// - some amount to verify the ext message. depends on current implementation
			waitTilBalanceAtLeast(prevBalance.Uint64() - 2*s.GasToValue(feeCredit).Uint64() - callValue - extMessageVerificationFee)
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

	var walletAddr, counterAddr types.Address
	var hash common.Hash

	callerShardId := types.ShardId(2)
	calleeShardId := types.ShardId(4)

	s.Run("Deploy wallet", func() {
		pub := crypto.CompressPubkey(&pk.PublicKey)
		walletCode := contracts.PrepareDefaultWalletForOwnerCode(pub)
		deployCode := types.BuildDeployPayload(walletCode, common.EmptyHash)

		hash, walletAddr, err = s.Client.DeployContract(
			callerShardId, types.MainWalletAddress, deployCode, types.NewValueFromUint64(10_000_000), execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Deploy counter", func() {
		deployCode := contracts.CounterDeployPayload(s.T())

		hash, counterAddr, err = s.Client.DeployContract(
			calleeShardId, types.MainWalletAddress, deployCode, types.Value{}, execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitIncludedInMain(hash)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	addCalldata := contracts.NewCounterAddCallData(s.T(), 1)

	var intMsgEstimation types.Value
	s.Run("Estimate internal message fee", func() {
		callArgs := &jsonrpc.CallArgs{
			Data:  (*hexutil.Bytes)(&addCalldata),
			To:    counterAddr,
			Flags: types.NewMessageFlags(types.MessageFlagInternal),
		}

		intMsgEstimation, err = s.Client.EstimateFee(callArgs, "latest")
		s.Require().NoError(err)
		s.Positive(intMsgEstimation.Uint64())
	})

	intMsg := &types.InternalMessagePayload{
		Data:        addCalldata,
		To:          counterAddr,
		FeeCredit:   intMsgEstimation,
		ForwardKind: types.ForwardKindNone,
		Kind:        types.ExecutionMessageKind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	s.Require().NoError(err)

	calldata, err := contracts.NewCallData(contracts.NameWallet, "send", intMsgData)
	s.Require().NoError(err)

	callerSeqno, err := s.Client.GetTransactionCount(walletAddr, "pending")
	s.Require().NoError(err)

	callArgs := &jsonrpc.CallArgs{
		Data:  (*hexutil.Bytes)(&calldata),
		To:    walletAddr,
		Seqno: callerSeqno,
	}

	var estimation types.Value
	s.Run("Estimate external message fee", func() {
		estimation, err = s.Client.EstimateFee(callArgs, "latest")
		s.Require().NoError(err)
		s.Positive(estimation.Uint64())
	})

	s.Run("Call without override", func() {
		callArgs.FeeCredit = estimation

		res, err := s.Client.Call(callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutMessages, 1)

		value := res.CoinsUsed.
			Add(res.OutMessages[0].CoinsUsed).
			Add(s.GasToValue(3 * params.SstoreSentryGasEIP2200)).
			Add(s.GasToValue(10_000)). // external message verification
			Mul64(12).Div64(10)        // stock 20%
		s.Equal(estimation.Uint64(), value.Uint64())

		msg := res.OutMessages[0]
		s.Equal(walletAddr, msg.Message.From)
		s.Equal(counterAddr, msg.Message.To)
		s.False(msg.CoinsUsed.IsZero())
		s.Empty(msg.Data, "Result of message execution is empty")
		s.NotEmpty(msg.Message.Data, "Message payload is not empty")
		s.Require().Empty(msg.Error)

		s.Len(msg.OutMessages, 1)
		s.True(msg.Message.IsInternal())

		s.Require().Len(res.StateOverrides, 2)

		walletState := res.StateOverrides[walletAddr]
		s.Empty(walletState.State)
		s.Empty(walletState.StateDiff)
		s.NotEmpty(walletState.Balance)

		counterState := res.StateOverrides[counterAddr]
		s.Empty(counterState.State)
		s.NotEmpty(counterState.StateDiff)
		s.Empty(counterState.Balance)

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", nil)
		s.EqualValues(0, contracts.GetCounterValue(s.T(), getRes))

		getRes = s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(1, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Override for \"insufficient balance for transfer\"", func() {
		callArgs.FeeCredit = estimation

		override := &jsonrpc.StateOverrides{
			walletAddr: jsonrpc.Contract{Balance: &types.Value{}},
		}
		res, err := s.Client.Call(callArgs, "latest", override)
		s.Require().NoError(err)
		s.Require().EqualError(vm.ErrInsufficientBalance, res.Error)
	})

	s.Run("Override several shards", func() {
		callArgs.FeeCredit = estimation

		val := types.NewValueFromUint64(50_000_000)
		override := &jsonrpc.StateOverrides{
			walletAddr:              jsonrpc.Contract{Balance: &val},
			types.MainWalletAddress: jsonrpc.Contract{Balance: &val},
		}
		res, err := s.Client.Call(callArgs, "latest", override)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutMessages, 1)
	})

	intMsg = &types.InternalMessagePayload{
		Data:        contracts.NewCounterAddCallData(s.T(), 5),
		To:          counterAddr,
		RefundTo:    walletAddr,
		FeeCredit:   types.NewValueFromUint64(5_000_000),
		ForwardKind: types.ForwardKindRemaining,
		Kind:        types.ExecutionMessageKind,
	}

	intBytecode, err := intMsg.MarshalSSZ()
	s.Require().NoError(err)

	extPayload, err := contracts.NewCallData(contracts.NameWallet, "send", intBytecode)
	s.Require().NoError(err)

	s.Run("Send raw external message", func() {
		extMsg := &types.ExternalMessage{
			To:        walletAddr,
			Data:      extPayload,
			Seqno:     callerSeqno,
			Kind:      types.ExecutionMessageKind,
			FeeCredit: s.GasToValue(100_000),
		}

		extBytecode, err := extMsg.MarshalSSZ()
		s.Require().NoError(err)

		callArgs := &jsonrpc.CallArgs{
			Message:   (*hexutil.Bytes)(&extBytecode),
			FeeCredit: types.NewValueFromUint64(5_000_000),
		}

		res, err := s.Client.Call(callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutMessages, 1)

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(5, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Send raw internal message", func() {
		callArgs := &jsonrpc.CallArgs{
			Message: (*hexutil.Bytes)(&intBytecode),
			From:    &walletAddr,
			Seqno:   callerSeqno,
		}

		res, err := s.Client.Call(callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutMessages, 1)
		s.Require().True(res.OutMessages[0].Message.IsRefund())

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(5, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Send raw message", func() {
		msg := types.NewEmptyMessage()
		msg.To = walletAddr
		msg.From = walletAddr
		msg.Data = extPayload
		msg.Seqno = callerSeqno
		msg.FeeCredit = types.NewValueFromUint64(5_000_000)

		msgBytecode, err := msg.MarshalSSZ()
		s.Require().NoError(err)

		callArgs := &jsonrpc.CallArgs{
			Message: (*hexutil.Bytes)(&msgBytecode),
		}

		res, err := s.Client.Call(callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutMessages, 1)

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(5, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Send invalid message", func() {
		invalidMsg := hexutil.Bytes([]byte{0x1, 0x2, 0x3})
		callArgs := &jsonrpc.CallArgs{
			Message: &invalidMsg,
		}

		_, err := s.Client.Call(callArgs, "latest", nil)
		s.Require().ErrorContains(err, rpctypes.ErrInvalidMessage.Error())
	})
}

func (s *SuiteRpc) TestChainCall() {
	addrCallee := contracts.CounterAddress(s.T(), types.ShardId(3))
	deployPayload := contracts.CounterDeployPayload(s.T()).Bytes()
	addCallData := contracts.NewCounterAddCallData(s.T(), 11)
	getCallData := contracts.NewCounterGetCallData(s.T())

	callArgs := &jsonrpc.CallArgs{
		To:        addrCallee,
		FeeCredit: s.GasToValue(100000000000),
	}

	callArgs.Data = (*hexutil.Bytes)(&deployPayload)
	callArgs.Flags = types.NewMessageFlags(types.MessageFlagDeploy)
	res, err := s.Client.Call(callArgs, "latest", nil)
	s.Require().NoError(err, "Deployment should be successful")
	s.Contains(res.StateOverrides, addrCallee)
	s.NotEmpty(res.StateOverrides[addrCallee].Code)

	resData := s.CallGetter(addrCallee, getCallData, "latest", &res.StateOverrides)
	s.EqualValues(0, contracts.GetCounterValue(s.T(), resData), "Initial value should be 0")

	callArgs.Data = (*hexutil.Bytes)(&addCallData)
	callArgs.Flags = types.NewMessageFlags()
	res, err = s.Client.Call(callArgs, "latest", &res.StateOverrides)
	s.Require().NoError(err, "No errors during the first addition")

	resData = s.CallGetter(addrCallee, getCallData, "latest", &res.StateOverrides)
	s.EqualValues(11, contracts.GetCounterValue(s.T(), resData), "Updated value is 11")

	callArgs.Data = (*hexutil.Bytes)(&addCallData)
	res, err = s.Client.Call(callArgs, "latest", &res.StateOverrides)
	s.Require().NoError(err, "No errors during the second addition")

	resData = s.CallGetter(addrCallee, getCallData, "latest", &res.StateOverrides)
	s.EqualValues(22, contracts.GetCounterValue(s.T(), resData), "Final value after two additions is 22")
}

func (s *SuiteRpc) TestAsyncAwaitCall() {
	var addrCounter, addrAwait types.Address
	s.Run("Deploy counter", func() {
		dpCounter := contracts.CounterDeployPayload(s.T())
		addrCounter, _ = s.DeployContractViaMainWallet(types.BaseShardId, dpCounter, types.Value{})

		addCalldata := contracts.NewCounterAddCallData(s.T(), 123)
		getCalldata := contracts.NewCounterGetCallData(s.T())
		receipt := s.SendMessageViaWallet(types.MainWalletAddress, addrCounter, execution.MainPrivateKey, addCalldata)
		s.Require().True(receipt.IsCommitted())

		getCallArgs := &jsonrpc.CallArgs{
			To:        addrCounter,
			FeeCredit: s.GasToValue(10_000_000),
			Data:      (*hexutil.Bytes)(&getCalldata),
		}
		res, err := s.Client.Call(getCallArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().EqualValues(123, contracts.GetCounterValue(s.T(), res.Data))
	})

	s.Run("Deploy await", func() {
		dpAwait := contracts.GetDeployPayload(s.T(), contracts.NameRequestResponseTest)
		addrAwait, _ = s.DeployContractViaMainWallet(types.BaseShardId, dpAwait, tests.DefaultContractValue)
	})

	abiAwait, err := contracts.GetAbi(contracts.NameRequestResponseTest)
	s.Require().NoError(err)

	data := s.AbiPack(abiAwait, "sumCounters", []types.Address{addrCounter})
	receipt := s.SendExternalMessageNoCheck(data, addrAwait)
	s.Require().True(receipt.AllSuccess())

	callArgs := &jsonrpc.CallArgs{
		To:        addrAwait,
		FeeCredit: s.GasToValue(10_000_000),
		Data:      (*hexutil.Bytes)(&data),
	}
	res, err := s.Client.Call(callArgs, "latest", nil)
	s.Require().NoError(err)
	s.Nil(res.Data)

	data = s.AbiPack(abiAwait, "get")
	callArgs.Data = (*hexutil.Bytes)(&data)

	res, err = s.Client.Call(callArgs, "latest", nil)
	s.Require().NoError(err)
	value := s.AbiUnpack(abiAwait, "get", res.Data)
	s.Require().Len(value, 1)
	s.Require().EqualValues(123, value[0])
}

func (s *SuiteRpc) TestRpcApiModules() {
	res, err := s.Client.RawCall("rpc_modules")
	s.Require().NoError(err)

	var data map[string]any
	s.Require().NoError(json.Unmarshal(res, &data))
	s.Equal("1.0", data["eth"])
	s.Equal("1.0", data["rpc"])
}

func (s *SuiteRpc) TestUnsupportedCliVersion() {
	logger := zerolog.New(os.Stderr)
	s.Run("Unsupported version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.Endpoint, logger, map[string]string{"User-Agent": "nil-cli/12"})
		_, err := client.ChainId()
		s.Require().ErrorContains(err, "unexpected status code: 426: specified revision 12, minimum supported is")
	})

	s.Run("0 means unknown - skip check", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.Endpoint, logger, map[string]string{"User-Agent": "nil-cli/0"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})

	s.Run("Valid revision", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.Endpoint, logger, map[string]string{"User-Agent": "nil-cli/10000000"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})
}

func (s *SuiteRpc) TestUnsupportedNiljsVersion() {
	logger := zerolog.New(os.Stderr)
	s.Run("Unsupported version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.Endpoint, logger, map[string]string{"Client-Version": "0.0.1"})
		_, err := client.ChainId()
		s.Require().ErrorContains(err, "unexpected status code: 426: specified niljs version 0.0.1, minimum supported is")
	})

	s.Run("Valid version", func() {
		client := rpc_client.NewClientWithDefaultHeaders(s.Endpoint, logger, map[string]string{"Client-Version": "2.0.0"})
		_, err := client.ChainId()
		s.Require().NoError(err)
	})
}

func (s *SuiteRpc) TestEmptyDeployPayload() {
	wallet := types.MainWalletAddress

	// Deploy contract with invalid payload
	hash, _, err := s.Client.DeployContract(types.BaseShardId, wallet, types.DeployPayload{}, types.Value{}, execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestInvalidMessageExternalDeployment() {
	calldataExt, err := contracts.NewCallData(contracts.NameWallet, "send", []byte{0x0, 0x1, 0x2, 0x3})
	s.Require().NoError(err)

	wallet := types.MainWalletAddress
	hash, err := s.Client.SendExternalMessage(calldataExt, wallet, execution.MainPrivateKey, s.GasToValue(100_000))
	s.Require().NoError(err)
	s.Require().NotEmpty(hash)

	receipt := s.WaitForReceipt(hash)
	s.Require().False(receipt.Success)
	s.Require().Equal(types.ErrorInvalidMessageInputUnmarshalFailed.String(), receipt.Status)
	s.Require().Equal("InvalidMessageInputUnmarshalFailed: "+ssz.ErrSize.Error(), receipt.ErrorMessage)
}

func (s *SuiteRpc) TestRpcError() {
	check := func(code int, msg, method string, params ...any) {
		resp, err := s.Client.RawCall(method, params...)
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

	check(-32602, "invalid argument 0: hex string of odd length",
		rpc_client.Eth_getBlockByHash, "0x1b4", false)

	check(-32602, "invalid argument 0: hex string without 0x prefix",
		rpc_client.Eth_getBlockByHash, "latest")
}

func (s *SuiteRpc) TestRpcDebugModules() {
	res, err := s.Client.GetDebugBlock(types.BaseShardId, "latest", false)
	s.Require().NoError(err)

	block, err := res.DecodeSSZ()
	s.Require().NoError(err)

	s.Require().NotEmpty(block.Id)
	s.Require().NotEqual(common.EmptyHash, block.Block.Hash(types.BaseShardId))
	s.Require().NotEmpty(res.Content)

	fullRes, err := s.Client.GetDebugBlock(types.BaseShardId, "latest", true)
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

	addr, receipt := s.DeployContractViaMainWallet(2, types.BuildDeployPayload(code, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Call Test contract with invalid argument, so no output messages should be generated
	calldata, err := abi.Pack("testFailedAsyncCall", addr, int32(0))
	s.Require().NoError(err)

	txhash, err := s.Client.SendExternalMessage(calldata, addr, nil, s.GasToValue(100_000))
	s.Require().NoError(err)
	receipt = s.WaitForReceipt(txhash)
	s.Require().False(receipt.Success)
	s.Require().NotEqual("Success", receipt.Status)
	s.Require().Empty(receipt.OutReceipts)
	s.Require().Empty(receipt.OutMessages)

	// Call Test contract with valid argument, so output messages should be generated
	calldata, err = abi.Pack("testFailedAsyncCall", addr, int32(10))
	s.Require().NoError(err)

	txhash, err = s.Client.SendExternalMessage(calldata, addr, nil, s.GasToValue(100_000))
	s.Require().NoError(err)
	receipt = s.WaitForReceipt(txhash)
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().Len(receipt.OutMessages, 1)
}

func (s *SuiteRpc) TestMultipleRefunds() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)

	var leftShardId types.ShardId = 1
	var rightShardId types.ShardId = 2

	_, receipt := s.DeployContractViaMainWallet(leftShardId, types.BuildDeployPayload(code, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	_, receipt = s.DeployContractViaMainWallet(rightShardId, types.BuildDeployPayload(code, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcBlockContent() {
	// Deploy message
	hash, _, err := s.Client.DeployContract(types.BaseShardId, types.MainWalletAddress,
		contracts.CounterDeployPayload(s.T()), types.Value{},
		execution.MainPrivateKey)
	s.Require().NoError(err)

	var block *jsonrpc.RPCBlock
	s.Eventually(func() bool {
		var err error
		block, err = s.Client.GetBlock(types.BaseShardId, "latest", false)
		s.Require().NoError(err)

		return len(block.MessageHashes) > 0
	}, 6*time.Second, 50*time.Millisecond)

	block, err = s.Client.GetBlock(types.BaseShardId, block.Hash, true)
	s.Require().NoError(err)

	s.Require().NotNil(block.Hash)
	s.Require().Len(block.Messages, 1)
	s.Equal(hash, block.Messages[0].Hash)
}

func (s *SuiteRpc) TestRpcMessageContent() {
	shardId := types.ShardId(3)
	hash, _, err := s.Client.DeployContract(shardId, types.MainWalletAddress,
		contracts.CounterDeployPayload(s.T()), types.Value{},
		execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)

	msg1, err := s.Client.GetInMessageByHash(hash)
	s.Require().NoError(err)
	s.EqualValues(0, msg1.Flags.Bits)

	msg2, err := s.Client.GetInMessageByHash(receipt.OutMessages[0])
	s.Require().NoError(err)
	s.EqualValues(3, msg2.Flags.Bits)
}

func (s *SuiteRpc) TestDbApi() {
	block, err := s.Client.GetBlock(types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)

	s.Require().NoError(s.Client.DbInitTimestamp(block.DbTimestamp))

	hBytes, err := s.Client.DbGet(db.LastBlockTable, types.BaseShardId.Bytes())
	s.Require().NoError(err)

	h := common.BytesToHash(hBytes)

	s.Require().Equal(block.Hash, h)
}

func (s *SuiteRpc) TestBatch() {
	testcases := map[string]string{
		"[]": `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"empty batch"}}`,
		`[{"jsonrpc":"2.0","id": 1, "method":"rpc_modules","params":[]}]`:                                                                `[{"jsonrpc":"2.0","id":1,"result":{"db":"1.0","debug":"1.0","eth":"1.0","faucet":"1.0","rpc":"1.0"}}]`,
		`[{"jsonrpc":"2.0","id": 1, "method":"rpc_modules","params":[]}, {"jsonrpc":"2.0","id": 2, "method":"rpc_modules","params":[]}]`: `[{"jsonrpc":"2.0","id":1,"result":{"db":"1.0","debug":"1.0","eth":"1.0","faucet":"1.0","rpc":"1.0"}}, {"jsonrpc":"2.0","id":2,"result":{"db":"1.0","debug":"1.0","faucet":"1.0","eth":"1.0","rpc":"1.0"}}]`,
		`[{"jsonrpc":"2.0", "method":"rpc_modules","params":[]}]`:                                                                        `[{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request"}}]`,
		`[{"jsonrpc":"2.0", "method":"eth_getBlockByNumber", "params": [0, "100500", false], "id": 1}]`:                                  `[{"jsonrpc":"2.0","id":1,"result":null}]`,
	}

	for req, expectedResp := range testcases {
		body, err := s.Client.PlainTextCall([]byte(req))
		s.Require().NoError(err)
		s.JSONEq(expectedResp, string(body))
	}

	var err error
	batch := s.Client.CreateBatchRequest()

	_, err = batch.GetBlock(types.MainShardId, "latest", false)
	s.Require().NoError(err)
	_, err = batch.GetDebugBlock(types.BaseShardId, "latest", false)
	s.Require().NoError(err)
	const tooBigNonexistentBlockNumber = "100500"
	_, err = batch.GetBlock(types.MainShardId, tooBigNonexistentBlockNumber, false)
	s.Require().NoError(err)
	_, err = batch.GetDebugBlock(types.BaseShardId, tooBigNonexistentBlockNumber, false)
	s.Require().NoError(err)

	result, err := s.Client.BatchCall(batch)
	s.Require().NoError(err)
	s.Require().Len(result, 4)

	b1, ok := result[0].(*jsonrpc.RPCBlock)
	s.Require().True(ok)
	s.Equal(types.MainShardId, b1.ShardId)

	b2, ok := result[1].(*jsonrpc.DebugRPCBlock)
	s.Require().True(ok)
	s.NotEmpty(b2.Content)

	s.Require().Nil(result[2])
	s.Require().Nil(result[3])
}

func (s *SuiteRpc) TestAddressCalculation() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	data := s.GetRandomBytes(65)
	refHash := common.PoseidonHash(data)
	salt := s.GetRandomBytes(32)
	shardId := types.ShardId(2)
	address := types.CreateAddress(shardId, types.BuildDeployPayload(code, common.BytesToHash(salt)))
	address2 := types.CreateAddressForCreate2(address, code, common.BytesToHash(salt))
	codeHash := common.PoseidonHash(code).Bytes()

	addr, receipt := s.DeployContractViaMainWallet(2, types.BuildDeployPayload(code, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Test `Nil.getPoseidonHash()` library method
	calldata, err := abi.Pack("getPoseidonHash", data)
	s.Require().NoError(err)
	resHash := s.CallGetter(addr, calldata, "latest", nil)
	s.Require().Equal(refHash[:], resHash)

	// Test `Nil.createAddress()` library method
	calldata, err = abi.Pack("createAddress", big.NewInt(int64(shardId)), []byte(code), big.NewInt(0).SetBytes(salt))
	s.Require().NoError(err)
	resAddress := s.CallGetter(addr, calldata, "latest", nil)
	s.Require().Equal(address, types.BytesToAddress(resAddress))

	// Test `Nil.createAddress2()` library method
	calldata, err = abi.Pack("createAddress2", big.NewInt(int64(shardId)), address, big.NewInt(0).SetBytes(salt),
		big.NewInt(0).SetBytes(codeHash))
	s.Require().NoError(err)
	resAddress = s.CallGetter(addr, calldata, "latest", nil)
	s.Require().Equal(address2, types.BytesToAddress(resAddress))
}

func (s *SuiteRpc) TestBloom() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	payload := contracts.GetDeployPayload(s.T(), contracts.NameTest)

	addr, receipt := s.DeployContractViaMainWallet(2, payload, tests.DefaultContractValue)
	s.Require().True(receipt.AllSuccess())

	topic1 := types.NewValueFromUint64(12345)
	topic2 := types.NewValueFromUint64(67890)
	calldata := s.AbiPack(abi, "emitEvent", topic1, topic2)
	receipt = s.SendExternalMessage(calldata, addr)
	s.Require().True(receipt.AllSuccess())
	s.Require().NotEmpty(receipt.Bloom)

	checkTopics := func(bloom types.Bloom) {
		b := topic1.Bytes32()
		s.Require().True(bloom.Test(b[:]))
		b = topic2.Bytes32()
		s.Require().True(bloom.Test(b[:]))
		b = [32]byte{1}
		s.Require().False(bloom.Test(b[:]))
	}

	block, err := s.Client.GetBlock(addr.ShardId(), receipt.BlockHash, false)
	s.Require().NoError(err)

	checkTopics(types.BytesToBloom(receipt.Bloom))
	checkTopics(types.BytesToBloom(block.LogsBloom))
}

func (s *SuiteRpc) TestDebugLogs() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	addr, receipt := s.DeployContractViaMainWallet(2, types.BuildDeployPayload(code, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.AllSuccess())

	s.Run("DebugLog in successful transaction", func() {
		calldata, err := abi.Pack("emitLog", "Test string 1", false)
		s.Require().NoError(err)

		receipt = s.SendExternalMessage(calldata, addr)
		s.Require().True(receipt.AllSuccess())

		s.Require().Len(receipt.Logs, 1)
		s.Require().Len(receipt.DebugLogs, 2)
		s.Require().Equal("Test string 1", receipt.DebugLogs[0].Message)
		s.Require().Empty(receipt.DebugLogs[0].Data)

		s.Require().Equal("Test string 1", receipt.DebugLogs[1].Message)
		s.Require().Len(receipt.DebugLogs[1].Data, 2)
		s.Require().Equal(*types.NewUint256(8888), receipt.DebugLogs[1].Data[0])
		s.Require().Equal(*types.NewUint256(0), receipt.DebugLogs[1].Data[1])
	})

	s.Run("DebugLog in failed transaction", func() {
		calldata, err := abi.Pack("emitLog", "Test string 2", true)
		s.Require().NoError(err)

		receipt = s.SendExternalMessageNoCheck(calldata, addr)
		s.Require().False(receipt.AllSuccess())

		s.Require().Empty(receipt.Logs)
		s.Require().Len(receipt.DebugLogs, 2)
		s.Require().Equal("Test string 2", receipt.DebugLogs[0].Message)
		s.Require().Empty(receipt.DebugLogs[0].Data)

		s.Require().Equal("Test string 2", receipt.DebugLogs[1].Message)
		s.Require().Len(receipt.DebugLogs[1].Data, 2)
		s.Require().Equal(*types.NewUint256(8888), receipt.DebugLogs[1].Data[0])
		s.Require().Equal(*types.NewUint256(1), receipt.DebugLogs[1].Data[1])
	})
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
