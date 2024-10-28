//go:build test

package tests

import (
	"crypto/ecdsa"
	"testing"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func WaitForReceiptCommon(
	t *testing.T, client client.Client, shardId types.ShardId, hash common.Hash, check func(*jsonrpc.RPCReceipt) bool,
) *jsonrpc.RPCReceipt {
	t.Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	require.Eventually(t, func() bool {
		receipt, err = client.GetInMessageReceipt(shardId, hash)
		require.NoError(t, err)
		return check(receipt)
	}, ReceiptWaitTimeout, ReceiptPollInterval)

	assert.Equal(t, hash, receipt.MsgHash)
	return receipt
}

func WaitForReceipt(t *testing.T, client client.Client, shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	t.Helper()

	return WaitForReceiptCommon(t, client, shardId, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsComplete()
	})
}

func WaitIncludedInMain(t *testing.T, client client.Client, shardId types.ShardId, hash common.Hash) *jsonrpc.RPCReceipt {
	t.Helper()

	return WaitForReceiptCommon(t, client, shardId, hash, func(receipt *jsonrpc.RPCReceipt) bool {
		return receipt.IsCommitted()
	})
}

func GasToValue(gas uint64) types.Value {
	return types.Gas(gas).ToValue(types.DefaultGasPrice)
}

// Deploy contract to specific shard
func DeployContractViaWallet(
	t *testing.T, client client.Client, addrFrom types.Address, key *ecdsa.PrivateKey,
	shardId types.ShardId, payload types.DeployPayload, initialAmount types.Value,
) (types.Address, *jsonrpc.RPCReceipt) {
	t.Helper()

	contractAddr := types.CreateAddress(shardId, payload)
	txHash, err := client.SendMessageViaWallet(addrFrom, types.Code{}, GasToValue(100_000), initialAmount,
		[]types.CurrencyBalance{}, contractAddr, key)
	require.NoError(t, err)
	receipt := WaitForReceipt(t, client, addrFrom.ShardId(), txHash)
	require.True(t, receipt.Success)
	require.Equal(t, "Success", receipt.Status)
	require.Len(t, receipt.OutReceipts, 1)

	txHash, addr, err := client.DeployContract(shardId, addrFrom, payload, types.Value{}, key)
	require.NoError(t, err)
	require.Equal(t, contractAddr, addr)

	receipt = WaitIncludedInMain(t, client, addrFrom.ShardId(), txHash)
	require.True(t, receipt.Success)
	require.Equal(t, "Success", receipt.Status)
	require.Len(t, receipt.OutReceipts, 1)
	return addr, receipt
}
