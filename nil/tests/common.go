//go:build test

package tests

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rollup"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tools/solc"
	l1types "github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func WaitForReceiptCommon(
	t *testing.T, client client.Client, hash common.Hash, check func(*jsonrpc.RPCReceipt) bool,
) *jsonrpc.RPCReceipt {
	t.Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	require.Eventually(t, func() bool {
		receipt, err = client.GetInTransactionReceipt(t.Context(), hash)
		require.NoError(t, err)
		return check(receipt)
	}, BlockWaitTimeout, BlockPollInterval)

	assert.Equal(t, hash, receipt.TxnHash)
	return receipt
}

func WaitForReceipt(t *testing.T, client client.Client, hash common.Hash) *jsonrpc.RPCReceipt {
	t.Helper()

	return WaitForReceiptCommon(t, client, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsComplete()
	})
}

func WaitIncludedInMain(t *testing.T, client client.Client, hash common.Hash) *jsonrpc.RPCReceipt {
	t.Helper()

	return WaitForReceiptCommon(t, client, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		// We should not wait for transactions if an external transaction fails. Because it may not be included in the
		// main chain.
		if receipt != nil && !receipt.Flags.IsInternal() && !receipt.Success {
			return true
		}
		return receipt.IsCommitted()
	})
}

func GasToValue(gas uint64) types.Value {
	return types.Gas(gas).ToValue(types.DefaultGasPrice)
}

// Deploy contract to specific shard
func DeployContractViaSmartAccount(
	t *testing.T,
	client client.Client,
	addrFrom types.Address,
	key *ecdsa.PrivateKey,
	shardId types.ShardId,
	payload types.DeployPayload,
	initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	t.Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := client.SendTransactionViaSmartAccount(
		t.Context(),
		addrFrom,
		types.Code{},
		types.NewFeePackFromGas(10_000_000),
		initialAmount,
		[]types.TokenBalance{},
		contractAddr,
		key)
	require.NoError(t, err)
	receipt := WaitForReceipt(t, client, txHash)
	require.True(t, receipt.Success)
	require.Equal(t, "Success", receipt.Status)
	require.Len(t, receipt.OutReceipts, 1)

	txHash, addr, err := client.DeployContract(t.Context(), shardId, addrFrom, payload, types.Value{},
		types.NewFeePackFromGas(10_000_000), key)
	require.NoError(t, err)
	require.Equal(t, contractAddr, addr)

	receipt = WaitIncludedInMain(t, client, txHash)
	require.True(t, receipt.Success)
	require.Equal(t, "Success", receipt.Status)
	require.Len(t, receipt.OutReceipts, 1)
	return addr, receipt
}

func LoadContract(t *testing.T, path string, name string) (types.Code, abi.ABI) {
	t.Helper()

	contracts, err := solc.CompileSource(path)
	require.NoError(t, err)
	code := hexutil.FromHex(contracts[name].Code)
	abi := solc.ExtractABI(contracts[name])
	return code, abi
}

func PrepareDefaultDeployPayload(
	t *testing.T, abi abi.ABI, salt common.Hash, code []byte, args ...any,
) types.DeployPayload {
	t.Helper()

	constructor, err := abi.Pack("", args...)
	require.NoError(t, err)
	code = append(code, constructor...)
	return types.BuildDeployPayload(code, salt)
}

func WaitBlock(
	t *testing.T,
	client client.Client,
	shardId types.ShardId,
	blockNum uint64,
) *jsonrpc.RPCBlock {
	t.Helper()

	var block *jsonrpc.RPCBlock
	var err error
	require.Eventually(t, func() bool {
		block, err = client.GetBlock(t.Context(), shardId, transport.BlockNumber(blockNum), false)
		return err == nil && block != nil
	}, BlockWaitTimeout, BlockPollInterval)
	return block
}

func WaitZerostate(t *testing.T, client client.Client, shardId types.ShardId) {
	t.Helper()

	WaitBlock(t, client, shardId, 0)
}

func WaitShardTick(t *testing.T, client client.Client, shardId types.ShardId) {
	t.Helper()

	require.Eventually(t, func() bool {
		block, err := client.GetBlock(t.Context(), shardId, 1, false)
		require.NoError(t, err)
		return block != nil
	}, ShardTickWaitTimeout, ShardTickPollInterval)
}

func GetBalance(t *testing.T, client client.Client, address types.Address) types.Value {
	t.Helper()
	balance, err := client.GetBalance(t.Context(), address, "latest")
	require.NoError(t, err)
	return balance
}

func AbiPack(t *testing.T, abi *abi.ABI, name string, args ...any) []byte {
	t.Helper()
	data, err := abi.Pack(name, args...)
	require.NoError(t, err)
	return data
}

func SendTransactionViaSmartAccount(
	t *testing.T,
	client client.Client,
	addrFrom types.Address,
	addrTo types.Address,
	key *ecdsa.PrivateKey,
	calldata []byte,
) *jsonrpc.RPCReceipt {
	t.Helper()

	txHash, err := client.SendTransactionViaSmartAccount(
		t.Context(),
		addrFrom,
		calldata,
		types.NewFeePackFromGas(1_000_000),
		types.NewZeroValue(),
		[]types.TokenBalance{}, addrTo, key)
	require.NoError(t, err)

	receipt := WaitIncludedInMain(t, client, txHash)
	require.True(t, receipt.Success)
	require.Equal(t, "Success", receipt.Status)
	require.Len(t, receipt.OutReceipts, 1)

	return receipt
}

func SendExternalTransactionNoCheck(
	t *testing.T,
	client client.Client,
	bytecode types.Code,
	contractAddress types.Address,
) *jsonrpc.RPCReceipt {
	t.Helper()

	txHash, err := client.SendExternalTransaction(t.Context(), bytecode, contractAddress, execution.MainPrivateKey,
		types.NewFeePackFromGas(1_000_000))
	require.NoError(t, err)

	return WaitIncludedInMain(t, client, txHash)
}

// AnalyzeReceipt analyzes the receipt and returns the receipt info.
func AnalyzeReceipt(
	t *testing.T,
	client client.Client,
	receipt *jsonrpc.RPCReceipt,
	namesMap map[types.Address]string,
) ReceiptInfo {
	t.Helper()
	res := make(ReceiptInfo)
	analyzeReceiptRec(t, client, receipt, res, namesMap)
	return res
}

// analyzeReceiptRec is a recursive function that analyzes the receipt and fills the receipt info.
func analyzeReceiptRec(
	t *testing.T,
	client client.Client,
	receipt *jsonrpc.RPCReceipt,
	valuesMap ReceiptInfo,
	namesMap map[types.Address]string,
) {
	t.Helper()

	value := getContractInfo(receipt.ContractAddress, valuesMap)
	if namesMap != nil {
		value.Name = namesMap[receipt.ContractAddress]
	}

	if receipt.Success {
		value.NumSuccess++
	} else {
		value.NumFail++
	}
	txn, err := client.GetInTransactionByHash(t.Context(), receipt.TxnHash)
	require.NoError(t, err)
	require.NotNil(t, txn)

	value.ValueUsed = value.ValueUsed.Add(receipt.GasUsed.ToValue(receipt.GasPrice))
	value.ValueForwarded = value.ValueForwarded.Add(receipt.Forwarded)
	caller := getContractInfo(txn.From, valuesMap)

	if txn.Flags.GetBit(types.TransactionFlagInternal) {
		caller.OutTransactions[receipt.ContractAddress] = txn
	}

	switch {
	case txn.Flags.GetBit(types.TransactionFlagBounce):
		value.BounceReceived = value.BounceReceived.Add(txn.Value)
		// Bounce transaction also bears refunded gas. If `To` address is equal to `RefundTo`, fee should be credited to
		// this account.
		if txn.To == txn.RefundTo {
			value.RefundReceived = value.RefundReceived.
				Add(txn.FeeCredit).
				Sub(receipt.GasUsed.ToValue(receipt.GasPrice))
		}
		// Remove the gas used by bounce transaction from the sent value
		value.ValueSent = value.ValueSent.Sub(receipt.GasUsed.ToValue(receipt.GasPrice))

		caller.BounceSent = caller.BounceSent.Add(txn.Value)
	case txn.Flags.GetBit(types.TransactionFlagRefund):
		value.RefundReceived = value.RefundReceived.Add(txn.Value)
		caller.RefundSent = caller.RefundSent.Add(txn.Value)
	default:
		// Receive value only if transaction was successful.
		if receipt.Success {
			value.ValueReceived = value.ValueReceived.Add(txn.Value)
		}
		caller.ValueSent = caller.ValueSent.Add(txn.Value)
		// For internal transaction we need to add gas limit to sent value
		if txn.Flags.GetBit(types.TransactionFlagInternal) {
			caller.ValueSent = caller.ValueSent.Add(txn.FeeCredit)
		}
	}

	for _, outReceipt := range receipt.OutReceipts {
		analyzeReceiptRec(t, client, outReceipt, valuesMap, namesMap)
	}
}

func CheckBalance(
	t *testing.T,
	client client.Client,
	infoMap ReceiptInfo,
	balance types.Value,
	accounts []types.Address,
) types.Value {
	t.Helper()

	newBalance := types.NewZeroValue()

	for _, addr := range accounts {
		newBalance = newBalance.Add(GetBalance(t, client, addr))
	}

	newRealBalance := newBalance

	for _, info := range infoMap {
		newBalance = newBalance.Add(info.ValueUsed)
	}
	require.Equal(t, balance, newBalance)

	return newRealBalance
}

func CallGetter(
	t *testing.T,
	client client.Client,
	addr types.Address,
	calldata []byte,
	blockId any,
	overrides *jsonrpc.StateOverrides,
) []byte {
	t.Helper()

	seqno, err := client.GetTransactionCount(t.Context(), addr, blockId)
	require.NoError(t, err)

	log.Debug().Str("contract", addr.String()).Uint64("seqno", uint64(seqno)).Msg("sending external transaction getter")

	callArgs := &jsonrpc.CallArgs{
		Data:  (*hexutil.Bytes)(&calldata),
		To:    addr,
		Fee:   types.NewFeePackFromGas(100_000_000),
		Seqno: seqno,
	}
	res, err := client.Call(t.Context(), callArgs, blockId, overrides)
	require.NoError(t, err)
	require.Empty(t, res.Error)
	return res.Data
}

func CheckContractValueEqual[T any](
	t *testing.T,
	client client.Client,
	inAbi *abi.ABI,
	address types.Address,
	name string,
	value T,
) {
	t.Helper()

	data := AbiPack(t, inAbi, name)
	data = CallGetter(t, client, address, data, "latest", nil)
	nameRes, err := inAbi.Unpack(name, data)
	require.NoError(t, err)
	gotValue, ok := nameRes[0].(T)
	require.True(t, ok)
	require.Equal(t, value, gotValue)
}

func CallGetterT[T any](
	t *testing.T,
	client client.Client,
	inAbi *abi.ABI,
	address types.Address,
	name string,
) T {
	t.Helper()

	data := AbiPack(t, inAbi, name)
	data = CallGetter(t, client, address, data, "latest", nil)
	nameRes, err := inAbi.Unpack(name, data)
	require.NoError(t, err)
	gotValue, ok := nameRes[0].(T)
	require.True(t, ok)
	return gotValue
}

func GetContract(t *testing.T, client client.Client, address types.Address) *types.SmartContract {
	t.Helper()

	res, err := client.GetDebugContract(t.Context(), address, "latest")
	require.NoError(t, err)

	var contract types.SmartContract
	require.NoError(t, contract.UnmarshalSSZ(res.Contract))
	return &contract
}

var DummyL1Fetcher = rollup.L1BlockFetcherMock{
	GetLastBlockInfoFunc: func(ctx context.Context) (*l1types.Header, error) {
		return nil, nil
	},
}

func GetDummyL1Fetcher() rollup.L1BlockFetcher {
	return &DummyL1Fetcher
}

func SendTransactionViaSmartAccountNoCheck(
	t *testing.T,
	client client.Client,
	addrSmartAccount types.Address,
	addrTo types.Address,
	key *ecdsa.PrivateKey,
	calldata []byte,
	fee types.FeePack,
	value types.Value,
	tokens []types.TokenBalance,
) *jsonrpc.RPCReceipt {
	t.Helper()

	// Send the raw transaction
	txHash, err := client.SendTransactionViaSmartAccount(
		t.Context(), addrSmartAccount, calldata, fee, value, tokens, addrTo, key)
	require.NoError(t, err)

	receipt := WaitIncludedInMain(t, client, txHash)
	// We don't check the receipt for success here, as it can be failed on purpose
	if receipt.Success {
		// But if it is successful, we expect exactly one out receipt
		require.Len(t, receipt.OutReceipts, 1)
	} else {
		require.NotEqual(t, "Success", receipt.Status)
	}

	return receipt
}

func GetRandomBytes(t *testing.T, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	_, err := rand.Read(data)
	require.NoError(t, err)
	return data
}
