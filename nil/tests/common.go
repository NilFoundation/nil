//go:build test

package tests

import (
	"context"
	"crypto/ecdsa"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/db"
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
	t *testing.T, ctx context.Context, client client.Client, hash common.Hash, check func(*jsonrpc.RPCReceipt) bool,
) *jsonrpc.RPCReceipt {
	t.Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	require.Eventually(t, func() bool {
		receipt, err = client.GetInTransactionReceipt(ctx, hash)
		require.NoError(t, err)
		return check(receipt)
	}, BlockWaitTimeout, BlockPollInterval)

	assert.Equal(t, hash, receipt.TxnHash)
	return receipt
}

func WaitForReceipt(t *testing.T, ctx context.Context, client client.Client, hash common.Hash) *jsonrpc.RPCReceipt {
	t.Helper()

	return WaitForReceiptCommon(t, ctx, client, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsComplete()
	})
}

func WaitIncludedInMain(t *testing.T, ctx context.Context, client client.Client, hash common.Hash) *jsonrpc.RPCReceipt {
	t.Helper()

	return WaitForReceiptCommon(t, ctx, client, hash, func(receipt *jsonrpc.RPCReceipt) bool {
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
	t *testing.T, ctx context.Context, client client.Client, addrFrom types.Address, key *ecdsa.PrivateKey,
	shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	t.Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := client.SendTransactionViaSmartAccount(ctx, addrFrom, types.Code{}, types.NewFeePackFromGas(10_000_000),
		initialAmount, []types.TokenBalance{}, contractAddr, key)
	require.NoError(t, err)
	receipt := WaitForReceipt(t, ctx, client, txHash)
	require.True(t, receipt.Success)
	require.Equal(t, "Success", receipt.Status)
	require.Len(t, receipt.OutReceipts, 1)

	txHash, addr, err := client.DeployContract(ctx, shardId, addrFrom, payload, types.Value{},
		types.NewFeePackFromGas(10_000_000), key)
	require.NoError(t, err)
	require.Equal(t, contractAddr, addr)

	receipt = WaitIncludedInMain(t, ctx, client, txHash)
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

func PrepareDefaultDeployPayload(t *testing.T, abi abi.ABI, code []byte, args ...any) types.DeployPayload {
	t.Helper()

	constructor, err := abi.Pack("", args...)
	require.NoError(t, err)
	code = append(code, constructor...)
	return types.BuildDeployPayload(code, common.EmptyHash)
}

func WaitBlock(t *testing.T, ctx context.Context, client client.Client, shardId types.ShardId, blockNum uint64) {
	t.Helper()

	require.Eventually(t, func() bool {
		block, err := client.GetBlock(ctx, shardId, transport.BlockNumber(blockNum), false)
		return err == nil && block != nil
	}, BlockWaitTimeout, BlockPollInterval)
}

func WaitZerostate(t *testing.T, ctx context.Context, client client.Client, shardId types.ShardId) {
	t.Helper()

	WaitBlock(t, ctx, client, shardId, 0)
}

func WaitShardTick(t *testing.T, ctx context.Context, client client.Client, shardId types.ShardId) {
	t.Helper()

	block, err := client.GetBlock(ctx, shardId, "latest", false)
	require.NoError(t, err)

	require.Eventuallyf(t, func() bool {
		block, err := client.GetBlock(ctx, shardId, transport.BlockNumber(block.Number+1), false)
		return err == nil && block != nil
	}, ShardTickWaitTimeout, ShardTickPollInterval, "Failed to wait for shard %d tick after block %d", shardId, block.Number)
}

func GetBalance(t *testing.T, ctx context.Context, client client.Client, address types.Address) types.Value {
	t.Helper()
	balance, err := client.GetBalance(ctx, address, "latest")
	require.NoError(t, err)
	return balance
}

func AbiPack(t *testing.T, abi *abi.ABI, name string, args ...any) []byte {
	t.Helper()
	data, err := abi.Pack(name, args...)
	require.NoError(t, err)
	return data
}

func SendExternalTransactionNoCheck(t *testing.T, ctx context.Context, client client.Client, bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	t.Helper()

	txHash, err := client.SendExternalTransaction(ctx, bytecode, contractAddress, execution.MainPrivateKey,
		types.NewFeePackFromGas(500_000))
	require.NoError(t, err)

	return WaitIncludedInMain(t, ctx, client, txHash)
}

// AnalyzeReceipt analyzes the receipt and returns the receipt info.
func AnalyzeReceipt(t *testing.T, ctx context.Context, client client.Client, receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	t.Helper()
	res := make(ReceiptInfo)
	analyzeReceiptRec(t, ctx, client, receipt, res, namesMap)
	return res
}

// analyzeReceiptRec is a recursive function that analyzes the receipt and fills the receipt info.
func analyzeReceiptRec(t *testing.T, ctx context.Context, client client.Client, receipt *jsonrpc.RPCReceipt, valuesMap ReceiptInfo, namesMap map[types.Address]string) {
	t.Helper()

	value := getContractInfo(receipt.ContractAddress, valuesMap)
	if namesMap != nil {
		value.Name = namesMap[receipt.ContractAddress]
	}

	if receipt.Success {
		value.NumSuccess += 1
	} else {
		value.NumFail += 1
	}
	txn, err := client.GetInTransactionByHash(ctx, receipt.TxnHash)
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
			value.RefundReceived = value.RefundReceived.Add(txn.FeeCredit).Sub(receipt.GasUsed.ToValue(receipt.GasPrice))
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
		analyzeReceiptRec(t, ctx, client, outReceipt, valuesMap, namesMap)
	}
}

func CheckBalance(t *testing.T, ctx context.Context, client client.Client, infoMap ReceiptInfo, balance types.Value, accounts []types.Address) types.Value {
	t.Helper()

	newBalance := types.NewZeroValue()

	for _, addr := range accounts {
		newBalance = newBalance.Add(GetBalance(t, ctx, client, addr))
	}

	newRealBalance := newBalance

	for _, info := range infoMap {
		newBalance = newBalance.Add(info.ValueUsed)
	}
	require.Equal(t, balance, newBalance)

	return newRealBalance
}

func CallGetter(t *testing.T, ctx context.Context, client client.Client, addr types.Address, calldata []byte, blockId any, overrides *jsonrpc.StateOverrides) []byte {
	t.Helper()

	seqno, err := client.GetTransactionCount(ctx, addr, blockId)
	require.NoError(t, err)

	log.Debug().Str("contract", addr.String()).Uint64("seqno", uint64(seqno)).Msg("sending external transaction getter")

	callArgs := &jsonrpc.CallArgs{
		Data:  (*hexutil.Bytes)(&calldata),
		To:    addr,
		Fee:   types.NewFeePackFromGas(100_000_000),
		Seqno: seqno,
	}
	res, err := client.Call(ctx, callArgs, blockId, overrides)
	require.NoError(t, err)
	require.Empty(t, res.Error)
	return res.Data
}

func CheckContractValueEqual[T any](t *testing.T, ctx context.Context, client client.Client, inAbi *abi.ABI, address types.Address, name string, value T) {
	t.Helper()

	data := AbiPack(t, inAbi, name)
	data = CallGetter(t, ctx, client, address, data, "latest", nil)
	nameRes, err := inAbi.Unpack(name, data)
	require.NoError(t, err)
	gotValue, ok := nameRes[0].(T)
	require.True(t, ok)
	require.Equal(t, value, gotValue)
}

func CallGetterT[T any](t *testing.T, ctx context.Context, client client.Client, inAbi *abi.ABI, address types.Address, name string) T {
	t.Helper()

	data := AbiPack(t, inAbi, name)
	data = CallGetter(t, ctx, client, address, data, "latest", nil)
	nameRes, err := inAbi.Unpack(name, data)
	require.NoError(t, err)
	gotValue, ok := nameRes[0].(T)
	require.True(t, ok)
	return gotValue
}

func GetContract(t *testing.T, ctx context.Context, database db.DB, address types.Address) *types.SmartContract {
	t.Helper()

	tx, err := database.CreateRoTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback()

	block, _, err := db.ReadLastBlock(tx, address.ShardId())
	require.NoError(t, err)

	contractTree := execution.NewDbContractTrieReader(tx, address.ShardId())
	contractTree.SetRootHash(block.SmartContractsRoot)

	contract, err := contractTree.Fetch(address.Hash())
	require.NoError(t, err)
	return contract
}

var DummyL1Fetcher = rollup.L1BlockFetcherMock{
	GetLastBlockInfoFunc: func(ctx context.Context) (*l1types.Header, error) {
		return nil, nil
	},
}

func GetDummyL1Fetcher() rollup.L1BlockFetcher {
	return &DummyL1Fetcher
}
