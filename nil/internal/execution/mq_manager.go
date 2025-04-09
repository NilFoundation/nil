package execution

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

func CreateMQPruneTransaction(shardId types.ShardId) (*types.Transaction, error) {
	abi, err := contracts.GetAbi(contracts.NameMessageQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to get MessageQueue ABI: %w", err)
	}

	calldata, err := abi.Pack("clearQueue")
	if err != nil {
		return nil, fmt.Errorf("failed to pack clearQueue calldata: %w", err)
	}

	addr := types.GetMessageQueueAddress(shardId)
	txn := &types.Transaction{
		TransactionDigest: types.TransactionDigest{
			Flags:                types.NewTransactionFlags(types.TransactionFlagInternal),
			To:                   addr,
			FeeCredit:            types.GasToValue(types.DefaultMaxGasInBlock.Uint64()),
			MaxFeePerGas:         types.MaxFeePerGasDefault,
			MaxPriorityFeePerGas: types.Value0,
			Data:                 calldata,
		},
		From: addr,
	}

	return txn, nil
}

func GetMessageQueueContent(es *ExecutionState, nShards uint32) (map[types.ShardId][][]byte, error) {
	addr := types.GetMessageQueueAddress(es.ShardId)
	account, err := es.GetAccount(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get message queue smart contract: %w", err)
	}

	abi, err := contracts.GetAbi(contracts.NameMessageQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to get MessageQueue ABI: %w", err)
	}

	result := make(map[types.ShardId][][]byte, nShards)
	for shardId := range types.ShardId(nShards) {
		calldata, err := abi.Pack("getMessages", uint(shardId))
		if err != nil {
			return nil, fmt.Errorf("failed to pack getMessages calldata: %w", err)
		}

		if err := es.newVm(true, addr, nil); err != nil {
			return nil, fmt.Errorf("failed to create VM: %w", err)
		}
		defer es.resetVm()

		ret, _, err := es.evm.StaticCall(
			(vm.AccountRef)(account.address), account.address, calldata, types.DefaultMaxGasInBlock.Uint64())
		if err != nil {
			return nil, fmt.Errorf("failed to get message queue content: %w", err)
		}

		var shardRes [][]byte
		err = abi.UnpackIntoInterface(&shardRes, "getMessages", ret)
		if err != nil {
			return nil, fmt.Errorf("failed to unpack getMessages return data: %w", err)
		}
		result[shardId] = shardRes
	}

	return result, nil
}
