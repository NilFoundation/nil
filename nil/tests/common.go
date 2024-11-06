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
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tools/solc"
	"github.com/rs/zerolog/log"
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

func WaitZerostate(t *testing.T, client client.Client, shardId types.ShardId) {
	t.Helper()

	require.Eventually(t, func() bool {
		block, err := client.GetBlock(shardId, transport.BlockNumber(0), false)
		return err == nil && block != nil
	}, ZeroStateWaitTimeout, ZeroStatePollInterval)
}

func GetBalance(t *testing.T, client client.Client, address types.Address) types.Value {
	t.Helper()
	balance, err := client.GetBalance(address, "latest")
	require.NoError(t, err)
	return balance
}

func AbiPack(t *testing.T, abi *abi.ABI, name string, args ...any) []byte {
	t.Helper()
	data, err := abi.Pack(name, args...)
	require.NoError(t, err)
	return data
}

func SendExternalMessageNoCheck(t *testing.T, client client.Client, bytecode types.Code, contractAddress types.Address) *jsonrpc.RPCReceipt {
	t.Helper()

	txHash, err := client.SendExternalMessage(bytecode, contractAddress, execution.MainPrivateKey, GasToValue(500_000))
	require.NoError(t, err)

	return WaitIncludedInMain(t, client, contractAddress.ShardId(), txHash)
}

// AnalyzeReceipt analyzes the receipt and returns the receipt info.
func AnalyzeReceipt(t *testing.T, client client.Client, receipt *jsonrpc.RPCReceipt, namesMap map[types.Address]string) ReceiptInfo {
	t.Helper()
	res := make(ReceiptInfo)
	analyzeReceiptRec(t, client, receipt, res, namesMap)
	return res
}

// analyzeReceiptRec is a recursive function that analyzes the receipt and fills the receipt info.
func analyzeReceiptRec(t *testing.T, client client.Client, receipt *jsonrpc.RPCReceipt, valuesMap ReceiptInfo, namesMap map[types.Address]string) {
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
	msg, err := client.GetInMessageByHash(receipt.ShardId, receipt.MsgHash)
	require.NoError(t, err)

	value.ValueUsed = value.ValueUsed.Add(receipt.GasUsed.ToValue(receipt.GasPrice))
	value.ValueForwarded = value.ValueForwarded.Add(receipt.Forwarded)
	caller := getContractInfo(msg.From, valuesMap)

	if msg.Flags.GetBit(types.MessageFlagInternal) {
		caller.OutMessages[receipt.ContractAddress] = msg
	}

	switch {
	case msg.Flags.GetBit(types.MessageFlagBounce):
		value.BounceReceived = value.BounceReceived.Add(msg.Value)
		// Bounce message also bears refunded gas. If `To` address is equal to `RefundTo`, fee should be credited to
		// this account.
		if msg.To == msg.RefundTo {
			value.RefundReceived = value.RefundReceived.Add(msg.FeeCredit).Sub(receipt.GasUsed.ToValue(receipt.GasPrice))
		}
		// Remove the gas used by bounce message from the sent value
		value.ValueSent = value.ValueSent.Sub(receipt.GasUsed.ToValue(receipt.GasPrice))

		caller.BounceSent = caller.BounceSent.Add(msg.Value)
	case msg.Flags.GetBit(types.MessageFlagRefund):
		value.RefundReceived = value.RefundReceived.Add(msg.Value)
		caller.RefundSent = caller.RefundSent.Add(msg.Value)
	default:
		// Receive value only if message was successful.
		if receipt.Success {
			value.ValueReceived = value.ValueReceived.Add(msg.Value)
		}
		caller.ValueSent = caller.ValueSent.Add(msg.Value)
		// For internal message we need to add gas limit to sent value
		if msg.Flags.GetBit(types.MessageFlagInternal) {
			caller.ValueSent = caller.ValueSent.Add(msg.FeeCredit)
		}
	}

	for _, outReceipt := range receipt.OutReceipts {
		analyzeReceiptRec(t, client, outReceipt, valuesMap, namesMap)
	}
}

func CheckBalance(t *testing.T, client client.Client, infoMap ReceiptInfo, balance types.Value, accounts []types.Address) types.Value {
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

func CallGetter(t *testing.T, client client.Client, addr types.Address, calldata []byte, blockId any, overrides *jsonrpc.StateOverrides) []byte {
	t.Helper()

	seqno, err := client.GetTransactionCount(addr, blockId)
	require.NoError(t, err)

	log.Debug().Str("contract", addr.String()).Uint64("seqno", uint64(seqno)).Msg("sending external message getter")

	callArgs := &jsonrpc.CallArgs{
		Data:      (*hexutil.Bytes)(&calldata),
		To:        addr,
		FeeCredit: GasToValue(100_000_000),
		Seqno:     seqno,
	}
	res, err := client.Call(callArgs, blockId, overrides)
	require.NoError(t, err)
	require.Empty(t, res.Error)
	return res.Data
}

func CheckContractValueEqual[T any](t *testing.T, client client.Client, inAbi *abi.ABI, address types.Address, name string, value T) {
	t.Helper()

	data := AbiPack(t, inAbi, name)
	data = CallGetter(t, client, address, data, "latest", nil)
	nameRes, err := inAbi.Unpack(name, data)
	require.NoError(t, err)
	gotValue, ok := nameRes[0].(T)
	require.True(t, ok)
	require.Equal(t, value, gotValue)
}

func CallGetterT[T any](t *testing.T, client client.Client, inAbi *abi.ABI, address types.Address, name string) T {
	t.Helper()

	data := AbiPack(t, inAbi, name)
	data = CallGetter(t, client, address, data, "latest", nil)
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
