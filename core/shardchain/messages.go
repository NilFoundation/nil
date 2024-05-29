package shardchain

import (
	"context"

	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/features"
)

func HandleMessages(ctx context.Context, es *execution.ExecutionState, msgs []*types.Message) error {
	blockContext := execution.NewEVMBlockContext(es)
	for _, message := range msgs {
		msgHash := message.Hash()
		index := es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := validateMessage(es, message, index)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		evm := vm.NewEVM(blockContext, es)
		interpreter := evm.Interpreter()

		// Deploy message
		if message.To.IsEmpty() {
			if err := es.HandleDeployMessage(message, index); err != nil {
				return err
			}
		} else {
			if err := es.HandleExecutionMessage(message, index, interpreter); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateMessage(es *execution.ExecutionState, message *types.Message, index uint64) (bool, error) {
	if !features.EnableSignatureCheck {
		return true, nil
	}
	addr := message.From
	accountState := es.GetAccount(addr)

	r := &types.Receipt{
		Success:         false,
		GasUsed:         0,
		MsgHash:         es.InMessageHash,
		MsgIndex:        index,
		ContractAddress: addr,
	}
	if accountState == nil {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().Stringer("shardId", es.ShardId).Stringer("address", addr).Msg("Invalid address")
		return false, nil
	}

	if len(accountState.PublicKey) != 0 {
		ok, err := message.ValidateSignature(accountState.PublicKey)
		if err != nil {
			return false, err
		}
		if !ok {
			r.Logs = es.Logs[es.InMessageHash]
			es.AddReceipt(r)
			sharedLogger.Debug().Stringer("shardId", es.ShardId).Stringer("address", addr).Msg("Invalid signature")
			return false, nil
		}
	}

	if accountState.Seqno != message.Seqno {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer("shardId", es.ShardId).
			Stringer("address", addr).
			Uint64("account.seqno", accountState.Seqno).
			Uint64("message.seqno", message.Seqno).
			Msg("Seqno gap")
		return false, nil
	}

	return true, nil
}
