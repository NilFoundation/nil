package execution

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type message struct {
	Data    []byte
	Address types.Address
}

func CreateMQPruneTransaction(shardId types.ShardId, bn types.BlockNumber) (*types.Transaction, error) {
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
			Seqno:                types.Seqno(bn + 1),
		},
		RefundTo: addr,
		From:     addr,
	}

	return txn, nil
}

func GetMessageQueueContent(es *ExecutionState) ([]message, error) {
	addr := types.GetMessageQueueAddress(es.ShardId)
	account, err := es.GetAccount(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get message queue smart contract: %w", err)
	}

	abi, err := contracts.GetAbi(contracts.NameMessageQueue)
	if err != nil {
		return nil, fmt.Errorf("failed to get MessageQueue ABI: %w", err)
	}

	calldata, err := abi.Pack("getMessages")
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

	var result []message
	if err := abi.UnpackIntoInterface(&result, "getMessages", ret); err != nil {
		return nil, fmt.Errorf("failed to unpack getMessages return data: %w", err)
	}

	return result, nil
}

func HandleOutMessage(es *ExecutionState, msg *message) error {
	var payload types.InternalTransactionPayload
	if err := payload.UnmarshalSSZ(msg.Data); err != nil {
		return types.NewWrapError(types.ErrorInvalidTransactionInputUnmarshalFailed, err)
	}

	cfgAccessor := es.GetConfigAccessor()
	nShards, err := config.GetParamNShards(cfgAccessor)
	if err != nil {
		return types.NewVmVerboseError(types.ErrorPrecompileConfigGetParamFailed, err.Error())
	}

	if uint32(payload.To.ShardId()) >= nShards {
		return vm.ErrShardIdIsTooBig
	}

	if payload.To.ShardId().IsMainShard() {
		return vm.ErrTransactionToMainShard
	}

	// TODO: support estimate fee for such messages
	payload.FeeCredit = types.MaxFeePerGasDefault

	// TODO: withdrawFunds should be implemneted
	// if err := withdrawFunds(es, msg.Address, payload.Value); err != nil {
	// 	return nil, fmt.Errorf("withdraw value failed: %w", err)
	// }

	// if payload.ForwardKind == types.ForwardKindNone {
	// 	if err := withdrawFunds(es, msg.Address, payload.FeeCredit); err != nil {
	// 		return nil, fmt.Errorf("withdraw FeeCredit failed: %w", err)
	// 	}
	// }

	// TODO: We should consider non-refundable transactions
	if payload.RefundTo == types.EmptyAddress {
		payload.RefundTo = msg.Address
	}
	if payload.BounceTo == types.EmptyAddress {
		payload.BounceTo = msg.Address
	}

	_, err = es.AddOutTransaction(msg.Address, &payload)
	return err
}
