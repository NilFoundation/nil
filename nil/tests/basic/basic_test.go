package tests

import (
	"context"
	"math/big"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	ssz "github.com/NilFoundation/fastssz"
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
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	tests.RpcSuite
	dbMock         *db.DBMock
	dbImpl         db.DB
	CreateRwTxFunc func(ctx context.Context) (db.RwTx, error)
	lock           sync.Mutex
}

func (s *SuiteRpc) SetupTest() {
	s.DbInit = func() db.DB {
		var err error
		s.dbImpl, err = db.NewBadgerDbInMemory()
		s.Require().NoError(err)
		s.dbMock = db.NewDbMock(s.dbImpl)
		s.dbMock.CreateRwTxFunc = func(ctx context.Context) (db.RwTx, error) {
			s.lock.Lock()
			defer s.lock.Unlock()
			return s.CreateRwTxFunc(ctx)
		}
		s.CreateRwTxFunc = func(ctx context.Context) (db.RwTx, error) {
			return s.dbImpl.CreateRwTx(ctx)
		}
		return s.dbMock
	}
	s.Start(&nilservice.Config{
		NShards: 5,
		HttpUrl: rpc.GetSockPath(s.T()),

		// NOTE: caching won't work with parallel tests in this module, because global cache will be shared
		EnableConfigCache: true,
	})
}

func (s *SuiteRpc) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRpc) TestRpcContract() {
	contractCode, abi := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")
	deployPayload := s.PrepareDefaultDeployPayload(abi, contractCode, big.NewInt(0))

	addr, receipt := s.DeployContractViaMainSmartAccount(types.BaseShardId, deployPayload, tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	blockNumber := transport.LatestBlockNumber
	balance, err := s.Client.GetBalance(s.Context, addr, transport.BlockNumberOrHash{BlockNumber: &blockNumber})
	s.Require().NoError(err)
	s.Require().Equal(tests.DefaultContractValue, balance)

	// now call (= send a transaction to) created contract
	calldata, err := abi.Pack("increment")
	s.Require().NoError(err)

	receipt = s.SendTransactionViaSmartAccount(types.MainSmartAccountAddress, addr, execution.MainPrivateKey, calldata)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcContractSendTransaction() {
	// deploy caller contract
	callerCode, callerAbi := s.LoadContract(common.GetAbsolutePath("../contracts/async_call.sol"), "Caller")
	calleeCode, calleeAbi := s.LoadContract(common.GetAbsolutePath("../contracts/async_call.sol"), "Callee")
	callerAddr, receipt := s.DeployContractViaMainSmartAccount(
		types.BaseShardId, types.BuildDeployPayload(callerCode, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	waitTilBalanceAtLeast := func(balance uint64) types.Value {
		s.T().Helper()

		var curBalance types.Value
		s.Require().Eventually(func() bool {
			var err error
			curBalance, err = s.Client.GetBalance(s.Context, callerAddr, transport.LatestBlockNumber)
			s.Require().NoError(err)
			return curBalance.Uint64() > balance
		}, tests.ReceiptWaitTimeout, 200*time.Millisecond)
		return curBalance
	}

	checkForShard := func(shardId types.ShardId) {
		s.T().Helper()

		s.Run("FailedDeploy", func() {
			// no account at address to pay for the transaction
			hash, _, err := s.Client.DeployExternal(s.Context, shardId,
				types.BuildDeployPayload(calleeCode, common.EmptyHash), types.NewFeePackFromGas(100_000))
			s.Require().NoError(err)

			receipt := s.WaitForReceipt(hash)
			s.False(receipt.Success)
			s.True(receipt.Temporary)
			s.Equal("DestinationContractDoesNotExist", receipt.Status)
		})

		var calleeAddr types.Address
		s.Run("DeployCallee", func() {
			// deploy callee contracts to different shards
			calleeAddr, receipt = s.DeployContractViaMainSmartAccount(
				shardId, types.BuildDeployPayload(calleeCode, common.EmptyHash), tests.DefaultContractValue)
			s.Require().True(receipt.Success)
			s.Require().True(receipt.OutReceipts[0].Success)
		})

		prevBalance, err := s.Client.GetBalance(s.Context, callerAddr, transport.LatestBlockNumber)
		s.Require().NoError(err)
		var feeCredit uint64 = 100_000
		var callValue uint64 = 2_000_000
		var callData []byte

		generateAddCallData := func(val int32) {
			// pack call of Callee::add into transaction
			callData, err = calleeAbi.Pack("add", val)
			s.Require().NoError(err)

			transactionToSend := &types.InternalTransactionPayload{
				Data:      callData,
				To:        calleeAddr,
				RefundTo:  callerAddr,
				BounceTo:  callerAddr,
				Value:     types.NewValueFromUint64(callValue),
				FeeCredit: s.GasToValue(feeCredit),
			}
			callData, err = transactionToSend.MarshalSSZ()
			s.Require().NoError(err)

			// now call Caller::send_transaction
			callData, err = callerAbi.Pack("sendTransaction", callData)
			s.Require().NoError(err)
		}

		var hash common.Hash
		makeCall := func() {
			callerSeqno, err := s.Client.GetTransactionCount(s.Context, callerAddr, "pending")
			s.Require().NoError(err)
			callCallerMethod := &types.ExternalTransaction{
				Seqno:        callerSeqno,
				To:           callerAddr,
				Data:         callData,
				FeeCredit:    s.GasToValue(feeCredit),
				MaxFeePerGas: types.MaxFeePerGasDefault,
			}
			s.Require().NoError(callCallerMethod.Sign(execution.MainPrivateKey))
			hash, err = s.Client.SendTransaction(s.Context, callCallerMethod)
			s.Require().NoError(err)
			s.Equal(hash, callCallerMethod.Hash())
		}

		s.Run("GenerateCallData", func() {
			generateAddCallData(123)
		})
		s.Run("MakeCall", makeCall)
		extTransactionVerificationFee := uint64(8350)
		s.Run("Check", func() {
			receipt = s.WaitForReceipt(hash)
			s.Require().True(receipt.Success)

			balance, err := s.Client.GetBalance(
				s.Context, callerAddr, transport.BlockNumberOrHash{BlockHash: &receipt.BlockHash})
			s.Require().NoError(err)
			s.Require().Greater(prevBalance.Uint64(), balance.Uint64())
			s.T().Logf("Spent %v nil", prevBalance.Uint64()-balance.Uint64())
			// here we spent:
			// - `callValue`, cause we attach that amount of value to internal cross-shard transaction
			// - `GasToValue(feeCredit)`, cause we buy that amount of gas to send cross-shard transaction
			// - `GasToValue(feeCredit)`, cause it's set in our ExternalTransaction
			// - some amount to verify the ext transaction. depends on current implementation
			minimalExpectedBalance := prevBalance.Uint64() - 2*s.GasToValue(feeCredit).Uint64() - callValue - extTransactionVerificationFee //nolint: lll
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

			callerSeqno, err := s.Client.GetTransactionCount(s.Context, callerAddr, "pending")
			s.Require().NoError(err)

			callArgs := &jsonrpc.CallArgs{
				Data:  (*hexutil.Bytes)(&callData),
				To:    callerAddr,
				Fee:   types.NewFeePackFromGas(10000),
				Seqno: callerSeqno,
			}

			res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
			s.T().Logf("Call res : %v, err: %v", res, err)
			s.Require().NoError(err)
			var bounceErr string
			s.Require().NoError(callerAbi.UnpackIntoInterface(&bounceErr, getBounceErrName, res.Data))
			s.Require().Equal(vm.ErrExecutionReverted.Error()+": Value must be non-zero", bounceErr)

			s.Require().Len(receipt.OutTransactions, 1)
			receipt = s.WaitForReceipt(receipt.OutTransactions[0])
			s.Require().False(receipt.Success)
			s.Require().Len(receipt.DebugLogs, 1)
			s.Require().Equal("execution started", receipt.DebugLogs[0].Message)

			// here we spent:
			// - `callValue`, cause we attach that amount of value to internal cross-shard transaction
			// - `GasToValue(feeCredit)`, cause we buy that amount of gas to send cross-shard transaction
			// - `GasToValue(feeCredit)`, cause it's set in our ExternalTransaction
			// - some amount to verify the ext transaction. depends on current implementation
			waitTilBalanceAtLeast(
				prevBalance.Uint64() - 2*s.GasToValue(feeCredit).Uint64() - callValue - extTransactionVerificationFee)
		})
	}

	s.Run("ToNeighborShard", func() {
		checkForShard(types.ShardId(4))
	})

	s.Run("ToSameShard", func() {
		checkForShard(types.BaseShardId)
	})

	s.Run("SendToNonExistingShard", func() {
		shardId := types.ShardId(1050)
		receipt := s.SendTransactionViaSmartAccountNoCheck(
			types.MainSmartAccountAddress,
			types.GenerateRandomAddress(shardId),
			execution.MainPrivateKey,
			nil,
			types.NewFeePackFromGas(100_000),
			types.NewValueFromUint64(100_000),
			nil)
		s.Require().False(receipt.Success)
		s.Equal("ShardIdIsTooBig", receipt.Status)
	})
}

func (s *SuiteRpc) TestRpcCallWithTransactionSend() { //nolint:maintidx
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)

	var smartAccountAddr, counterAddr types.Address
	var hash common.Hash

	callerShardId := types.ShardId(2)
	calleeShardId := types.ShardId(4)

	s.Run("Deploy smart account", func() {
		pub := crypto.CompressPubkey(&pk.PublicKey)
		smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(pub)
		deployCode := types.BuildDeployPayload(smartAccountCode, common.EmptyHash)

		hash, smartAccountAddr, err = s.Client.DeployContract(
			s.Context, callerShardId, types.MainSmartAccountAddress, deployCode, types.GasToValue(10_000_000),
			types.NewFeePackFromGas(20_000_000), execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	s.Run("Deploy counter", func() {
		deployCode := contracts.CounterDeployPayload(s.T())

		hash, counterAddr, err = s.Client.DeployContract(
			s.Context, calleeShardId, types.MainSmartAccountAddress, deployCode, types.Value{},
			types.NewFeePackFromGas(2_000_000), execution.MainPrivateKey,
		)
		s.Require().NoError(err)
		receipt := s.WaitIncludedInMain(hash)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)
	})

	addCalldata := contracts.NewCounterAddCallData(s.T(), 1)

	var txnEstimation *jsonrpc.EstimateFeeRes
	s.Run("Estimate internal transaction fee", func() {
		callArgs := &jsonrpc.CallArgs{
			Data:  (*hexutil.Bytes)(&addCalldata),
			To:    counterAddr,
			Flags: types.NewTransactionFlags(types.TransactionFlagInternal),
		}

		txnEstimation, err = s.Client.EstimateFee(s.Context, callArgs, "latest")
		s.Require().NoError(err)
		s.Positive(txnEstimation.FeeCredit.Uint64())
	})

	intTxn := &types.InternalTransactionPayload{
		Data:        addCalldata,
		To:          counterAddr,
		FeeCredit:   txnEstimation.FeeCredit,
		ForwardKind: types.ForwardKindNone,
		Kind:        types.ExecutionTransactionKind,
	}

	intTxnData, err := intTxn.MarshalSSZ()
	s.Require().NoError(err)

	calldata, err := contracts.NewCallData(contracts.NameSmartAccount, "send", intTxnData)
	s.Require().NoError(err)

	callerSeqno, err := s.Client.GetTransactionCount(s.Context, smartAccountAddr, "pending")
	s.Require().NoError(err)

	callArgs := &jsonrpc.CallArgs{
		Data:  (*hexutil.Bytes)(&calldata),
		To:    smartAccountAddr,
		Seqno: callerSeqno,
	}

	var estimation *jsonrpc.EstimateFeeRes
	s.Run("Estimate external transaction fee", func() {
		estimation, err = s.Client.EstimateFee(s.Context, callArgs, "latest")
		s.Require().NoError(err)
		s.Positive(estimation.FeeCredit.Uint64())
	})

	s.Run("Call without override", func() {
		callArgs.Fee = types.NewFeePackFromFeeCredit(estimation.FeeCredit)

		res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutTransactions, 1)

		value := res.CoinsUsed.
			Add(res.OutTransactions[0].CoinsUsed).
			Add(s.GasToValue(3 * params.SstoreSentryGasEIP2200)).
			Add(s.GasToValue(10_000)). // external transaction verification
			Mul64(12).Div64(10)        // stock 20%
		s.Equal(estimation.FeeCredit.Uint64(), value.Uint64())

		txn := res.OutTransactions[0]
		s.Equal(smartAccountAddr, txn.Transaction.From)
		s.Equal(counterAddr, txn.Transaction.To)
		s.False(txn.CoinsUsed.IsZero())
		s.Empty(txn.Data, "Result of transaction execution is empty")
		s.NotEmpty(txn.Transaction.Data, "Transaction payload is not empty")
		s.Require().Empty(txn.Error)

		s.Len(txn.OutTransactions, 1)
		s.True(txn.Transaction.IsInternal())

		s.Require().Len(res.StateOverrides, 2)

		smartAccountState := res.StateOverrides[smartAccountAddr]
		s.Empty(smartAccountState.State)
		s.Empty(smartAccountState.StateDiff)
		s.NotEmpty(smartAccountState.Balance)

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
		callArgs.Fee = types.NewFeePackFromFeeCredit(estimation.FeeCredit)

		override := &jsonrpc.StateOverrides{
			smartAccountAddr: jsonrpc.Contract{Balance: &types.Value{}},
		}
		res, err := s.Client.Call(s.Context, callArgs, "latest", override)
		s.Require().NoError(err)
		s.Require().EqualError(vm.ErrInsufficientBalance, res.Error)
	})

	s.Run("Override several shards", func() {
		callArgs.Fee = types.NewFeePackFromFeeCredit(estimation.FeeCredit)

		val := types.GasToValue(50_000_000)
		override := &jsonrpc.StateOverrides{
			smartAccountAddr:              jsonrpc.Contract{Balance: &val},
			types.MainSmartAccountAddress: jsonrpc.Contract{Balance: &val},
		}
		res, err := s.Client.Call(s.Context, callArgs, "latest", override)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutTransactions, 1)
	})

	intTxn = &types.InternalTransactionPayload{
		Data:        contracts.NewCounterAddCallData(s.T(), 5),
		To:          counterAddr,
		RefundTo:    smartAccountAddr,
		FeeCredit:   types.GasToValue(5_000_000),
		ForwardKind: types.ForwardKindRemaining,
		Kind:        types.ExecutionTransactionKind,
	}

	intBytecode, err := intTxn.MarshalSSZ()
	s.Require().NoError(err)

	extPayload, err := contracts.NewCallData(contracts.NameSmartAccount, "send", intBytecode)
	s.Require().NoError(err)

	s.Run("Send raw external transaction", func() {
		extTxn := &types.ExternalTransaction{
			To:           smartAccountAddr,
			Data:         extPayload,
			Seqno:        callerSeqno,
			Kind:         types.ExecutionTransactionKind,
			FeeCredit:    s.GasToValue(100_000),
			MaxFeePerGas: types.MaxFeePerGasDefault,
		}

		extBytecode, err := extTxn.MarshalSSZ()
		s.Require().NoError(err)

		callArgs := &jsonrpc.CallArgs{
			Transaction: (*hexutil.Bytes)(&extBytecode),
			Fee:         types.NewFeePackFromGas(500_000),
		}

		res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutTransactions, 1)

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(5, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Send raw internal transaction", func() {
		callArgs := &jsonrpc.CallArgs{
			Transaction: (*hexutil.Bytes)(&intBytecode),
			From:        &smartAccountAddr,
			Seqno:       callerSeqno,
			Fee:         types.NewFeePackFromGas(500_000),
		}

		res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutTransactions, 1)
		s.Require().True(res.OutTransactions[0].Transaction.IsRefund())

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(5, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Send raw transaction", func() {
		txn := types.NewEmptyTransaction()
		txn.To = smartAccountAddr
		txn.From = smartAccountAddr
		txn.Data = extPayload
		txn.Seqno = callerSeqno
		txn.FeeCredit = types.GasToValue(5_000_000)
		txn.MaxFeePerGas = types.MaxFeePerGasDefault

		txnBytecode, err := txn.MarshalSSZ()
		s.Require().NoError(err)

		callArgs := &jsonrpc.CallArgs{
			Transaction: (*hexutil.Bytes)(&txnBytecode),
		}

		res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().Empty(res.Error)
		s.Require().Len(res.OutTransactions, 1)

		getRes := s.CallGetter(counterAddr, contracts.NewCounterGetCallData(s.T()), "latest", &res.StateOverrides)
		s.EqualValues(5, contracts.GetCounterValue(s.T(), getRes))
	})

	s.Run("Send invalid transaction", func() {
		invalidTxn := hexutil.Bytes([]byte{0x1, 0x2, 0x3})
		callArgs := &jsonrpc.CallArgs{
			Transaction: &invalidTxn,
		}

		_, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().ErrorContains(err, rpctypes.ErrInvalidTransaction.Error())
	})
}

func (s *SuiteRpc) TestChainCall() {
	addrCallee := contracts.CounterAddress(s.T(), types.ShardId(3))
	deployPayload := contracts.CounterDeployPayload(s.T()).Bytes()
	addCallData := contracts.NewCounterAddCallData(s.T(), 11)
	getCallData := contracts.NewCounterGetCallData(s.T())

	callArgs := &jsonrpc.CallArgs{
		To:  addrCallee,
		Fee: types.NewFeePackFromGas(100_000_000_000),
	}

	callArgs.Data = (*hexutil.Bytes)(&deployPayload)
	callArgs.Flags = types.NewTransactionFlags(types.TransactionFlagDeploy)
	res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
	s.Require().NoError(err, "Deployment should be successful")
	s.Contains(res.StateOverrides, addrCallee)
	s.NotEmpty(res.StateOverrides[addrCallee].Code)

	resData := s.CallGetter(addrCallee, getCallData, "latest", &res.StateOverrides)
	s.EqualValues(0, contracts.GetCounterValue(s.T(), resData), "Initial value should be 0")

	callArgs.Data = (*hexutil.Bytes)(&addCallData)
	callArgs.Flags = types.NewTransactionFlags()
	res, err = s.Client.Call(s.Context, callArgs, "latest", &res.StateOverrides)
	s.Require().NoError(err, "No errors during the first addition")

	resData = s.CallGetter(addrCallee, getCallData, "latest", &res.StateOverrides)
	s.EqualValues(11, contracts.GetCounterValue(s.T(), resData), "Updated value is 11")

	callArgs.Data = (*hexutil.Bytes)(&addCallData)
	res, err = s.Client.Call(s.Context, callArgs, "latest", &res.StateOverrides)
	s.Require().NoError(err, "No errors during the second addition")

	resData = s.CallGetter(addrCallee, getCallData, "latest", &res.StateOverrides)
	s.EqualValues(22, contracts.GetCounterValue(s.T(), resData), "Final value after two additions is 22")
}

func (s *SuiteRpc) TestAsyncAwaitCall() {
	var addrCounter, addrAwait types.Address
	s.Run("Deploy counter", func() {
		dpCounter := contracts.CounterDeployPayload(s.T())
		addrCounter, _ = s.DeployContractViaMainSmartAccount(types.BaseShardId, dpCounter, types.Value{})

		addCalldata := contracts.NewCounterAddCallData(s.T(), 123)
		getCalldata := contracts.NewCounterGetCallData(s.T())
		receipt := s.SendTransactionViaSmartAccount(
			types.MainSmartAccountAddress, addrCounter, execution.MainPrivateKey, addCalldata)
		s.Require().True(receipt.IsCommitted())

		getCallArgs := &jsonrpc.CallArgs{
			To:   addrCounter,
			Fee:  types.NewFeePackFromGas(10_000_000),
			Data: (*hexutil.Bytes)(&getCalldata),
		}
		res, err := s.Client.Call(s.Context, getCallArgs, "latest", nil)
		s.Require().NoError(err)
		s.Require().EqualValues(123, contracts.GetCounterValue(s.T(), res.Data))
	})

	s.Run("Deploy await", func() {
		dpAwait := contracts.GetDeployPayload(s.T(), contracts.NameRequestResponseTest)
		addrAwait, _ = s.DeployContractViaMainSmartAccount(types.BaseShardId, dpAwait, tests.DefaultContractValue)
	})

	abiAwait, err := contracts.GetAbi(contracts.NameRequestResponseTest)
	s.Require().NoError(err)

	callArgs := &jsonrpc.CallArgs{
		To:  addrAwait,
		Fee: types.NewFeePackFromGas(1_000_000),
	}

	s.Run("Call await", func() {
		data := s.AbiPack(abiAwait, "sumCounters", []types.Address{addrCounter})
		receipt := s.SendExternalTransactionNoCheck(data, addrAwait)
		s.Require().True(receipt.AllSuccess())

		callArgs.Data = (*hexutil.Bytes)(&data)
		res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().NoError(err)
		s.Nil(res.Data)
	})

	s.Run("Call await with result", func() {
		data := s.AbiPack(abiAwait, "get")
		callArgs.Data = (*hexutil.Bytes)(&data)

		res, err := s.Client.Call(s.Context, callArgs, "latest", nil)
		s.Require().NoError(err)
		value := s.AbiUnpack(abiAwait, "get", res.Data)
		s.Require().Len(value, 1)
		s.Require().EqualValues(123, value[0])
	})
}

func (s *SuiteRpc) TestEmptyDeployPayload() {
	smartAccount := types.MainSmartAccountAddress

	// Deploy contract with invalid payload
	hash, _, err := s.Client.DeployContract(s.Context, types.BaseShardId, smartAccount, types.DeployPayload{},
		types.Value{}, types.NewFeePackFromGas(1_000_000), execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestInvalidTransactionExternalDeployment() {
	calldataExt, err := contracts.NewCallData(contracts.NameSmartAccount, "send", []byte{0x0, 0x1, 0x2, 0x3})
	s.Require().NoError(err)

	smartAccount := types.MainSmartAccountAddress
	hash, err := s.Client.SendExternalTransaction(
		s.Context,
		calldataExt,
		smartAccount,
		execution.MainPrivateKey,
		types.NewFeePackFromGas(100_000))
	s.Require().NoError(err)
	s.Require().NotEmpty(hash)

	receipt := s.WaitForReceipt(hash)
	s.Require().False(receipt.Success)
	s.Require().Equal(types.ErrorInvalidTransactionInputUnmarshalFailed.String(), receipt.Status)
	s.Require().Equal("InvalidTransactionInputUnmarshalFailed: "+ssz.ErrSize.Error(), receipt.ErrorMessage)
}

// Test that we remove output transactions if the transaction failed
func (s *SuiteRpc) TestNoOutTransactionsIfFailure() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	addr, receipt := s.DeployContractViaMainSmartAccount(
		2,
		types.BuildDeployPayload(code, common.EmptyHash),
		tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Call Test contract with invalid argument, so no output transactions should be generated
	calldata, err := abi.Pack("testFailedAsyncCall", addr, int32(0))
	s.Require().NoError(err)

	txhash, err := s.Client.SendExternalTransaction(s.Context, calldata, addr, nil, types.NewFeePackFromGas(100_000))
	s.Require().NoError(err)
	receipt = s.WaitForReceipt(txhash)
	s.Require().False(receipt.Success)
	s.Require().NotEqual("Success", receipt.Status)
	s.Require().Empty(receipt.OutReceipts)
	s.Require().Empty(receipt.OutTransactions)

	// Call Test contract with valid argument, so output transactions should be generated
	calldata, err = abi.Pack("testFailedAsyncCall", addr, int32(10))
	s.Require().NoError(err)

	txhash, err = s.Client.SendExternalTransaction(s.Context, calldata, addr, nil, types.NewFeePackFromGas(100_000))
	s.Require().NoError(err)
	receipt = s.WaitForReceipt(txhash)
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().Len(receipt.OutTransactions, 1)
}

func (s *SuiteRpc) TestMultipleRefunds() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)

	var leftShardId types.ShardId = 1
	var rightShardId types.ShardId = 2

	_, receipt := s.DeployContractViaMainSmartAccount(
		leftShardId,
		types.BuildDeployPayload(code, common.EmptyHash),
		tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)

	_, receipt = s.DeployContractViaMainSmartAccount(
		rightShardId,
		types.BuildDeployPayload(code, common.EmptyHash),
		tests.DefaultContractValue)
	s.Require().True(receipt.OutReceipts[0].Success)
}

func (s *SuiteRpc) TestRpcBlockContent() {
	// Deploy transaction
	hash, _, err := s.Client.DeployContract(
		s.Context,
		types.BaseShardId,
		types.MainSmartAccountAddress,
		contracts.CounterDeployPayload(s.T()),
		types.Value{},
		types.NewFeePackFromGas(1_000_000),
		execution.MainPrivateKey)
	s.Require().NoError(err)

	var block *jsonrpc.RPCBlock
	s.Eventually(func() bool {
		var err error
		block, err = s.Client.GetBlock(s.Context, types.BaseShardId, "latest", false)
		s.Require().NoError(err)

		return len(block.TransactionHashes) > 0
	}, 6*time.Second, 50*time.Millisecond)

	block, err = s.Client.GetBlock(s.Context, types.BaseShardId, block.Hash, true)
	s.Require().NoError(err)

	s.Require().NotNil(block.Hash)
	s.Require().Len(block.Transactions, 1)
	s.Equal(hash, block.Transactions[0].Hash)
}

func (s *SuiteRpc) TestRpcTransactionContent() {
	shardId := types.ShardId(3)
	hash, _, err := s.Client.DeployContract(
		s.Context,
		shardId,
		types.MainSmartAccountAddress,
		contracts.CounterDeployPayload(s.T()),
		types.Value{},
		types.NewFeePackFromGas(1_000_000),
		execution.MainPrivateKey)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)

	txn1, err := s.Client.GetInTransactionByHash(s.Context, hash)
	s.Require().NoError(err)
	s.EqualValues(0, txn1.Flags.Bits)

	txn2, err := s.Client.GetInTransactionByHash(s.Context, receipt.OutTransactions[0])
	s.Require().NoError(err)
	s.EqualValues(3, txn2.Flags.Bits)
}

func (s *SuiteRpc) TestTwoInvalidSignatureTxs() {
	shardId := types.BaseShardId
	_, _, err := s.Client.DeployContract(s.Context, shardId, types.MainSmartAccountAddress,
		contracts.CounterDeployPayload(s.T()), types.Value{}, types.NewFeePackFromGas(1_000_000), nil)
	s.Require().NoError(err)

	_, _, err = s.Client.DeployContract(s.Context, shardId, types.MainSmartAccountAddress,
		contracts.CounterDeployPayload(s.T()), types.Value{}, types.NewFeePackFromGas(1_000_000), nil)
	s.Require().NoError(err)

	block, err := s.Client.GetBlock(s.Context, shardId, "latest", false)
	s.Require().NoError(err)

	tests.WaitBlock(s.T(), s.Context, s.Client, shardId, uint64(block.Number)+1)
}

func (s *SuiteRpc) TestDbApi() {
	block, err := s.Client.GetBlock(s.Context, types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)

	s.Require().NoError(s.Client.DbInitTimestamp(s.Context, block.DbTimestamp))

	hBytes, err := s.Client.DbGet(s.Context, db.LastBlockTable, types.BaseShardId.Bytes())
	s.Require().NoError(err)

	h := common.BytesToHash(hBytes)

	s.Require().Equal(block.Hash, h)
}

func (s *SuiteRpc) TestBloom() {
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	payload := contracts.GetDeployPayload(s.T(), contracts.NameTest)

	addr, receipt := s.DeployContractViaMainSmartAccount(2, payload, tests.DefaultContractValue)
	s.Require().True(receipt.AllSuccess())

	topic1 := types.NewValueFromUint64(12345)
	topic2 := types.NewValueFromUint64(67890)
	calldata := s.AbiPack(abi, "emitEvent", topic1, topic2)
	receipt = s.SendExternalTransaction(calldata, addr)
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

	block, err := s.Client.GetBlock(s.Context, addr.ShardId(), receipt.BlockHash, false)
	s.Require().NoError(err)

	checkTopics(types.BytesToBloom(receipt.Bloom))
	checkTopics(types.BytesToBloom(block.LogsBloom))
}

func (s *SuiteRpc) TestDebugLogs() {
	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	addr, receipt := s.DeployContractViaMainSmartAccount(
		2, types.BuildDeployPayload(code, common.EmptyHash), tests.DefaultContractValue)
	s.Require().True(receipt.AllSuccess())

	s.Run("DebugLog in successful transaction", func() {
		calldata, err := abi.Pack("emitLog", "Test string 1", false)
		s.Require().NoError(err)

		receipt = s.SendExternalTransaction(calldata, addr)
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

		receipt = s.SendExternalTransactionNoCheck(calldata, addr)
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

func (s *SuiteRpc) TestPanicsInDb() {
	getCallStack := func() []byte {
		buf := make([]byte, 10240)
		runtime.Stack(buf, false)
		return buf
	}

	code, err := contracts.GetCode(contracts.NameTest)
	s.Require().NoError(err)
	abi, err := contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)

	addr, receipt := s.DeployContractViaMainSmartAccount(
		types.ShardId(3),
		types.BuildDeployPayload(code, common.EmptyHash),
		tests.DefaultContractValue)
	s.Require().True(receipt.AllSuccess())

	calldata := s.AbiPack(abi, "getValue")

	s.lock.Lock()
	s.CreateRwTxFunc = func(ctx context.Context) (db.RwTx, error) {
		tx, err := s.dbImpl.CreateRwTx(ctx)
		s.Require().NoError(err)

		buf := getCallStack()
		if strings.Contains(string(buf), "nil/internal/execution.NewBlockGenerator(") {
			// Create mock tx only for a block generation
			txMock := db.NewTxMock(tx)
			txMock.GetFromShardFunc = func(
				shardId types.ShardId, tableName db.ShardedTableName, key []byte,
			) ([]byte, error) {
				buf := getCallStack()
				if strings.Contains(string(buf), "(*ExecutionState).handleExecutionTransaction") {
					// Panic only when we execute a transaction
					panic("panic in db")
				}
				return tx.GetFromShard(shardId, tableName, key)
			}
			return txMock, nil
		}
		return tx, err
	}
	s.lock.Unlock()

	receipt = s.SendExternalTransactionNoCheck(calldata, addr)
	s.Require().False(receipt.Success)
	s.Require().Equal("PanicDuringExecution", receipt.Status)
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
