package shardchain

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/features"
)

func HandleMessages(ctx context.Context, es *execution.ExecutionState, msgs []*types.Message) error {
	blockContext := execution.NewEVMBlockContext(es)
	for _, message := range msgs {
		msgHash := message.Hash()
		es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := validateMessage(es, message)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		// Deploy message
		if message.To.IsEmpty() {
			if err := es.HandleDeployMessage(message, &blockContext); err != nil && !errors.As(err, new(vm.VMError)) {
				return err
			}
		} else {
			if _, err := es.HandleExecutionMessage(message, &blockContext); err != nil && !errors.Is(err, new(vm.VMError)) {
				return err
			}
		}
	}

	return nil
}

func validateMessage(es *execution.ExecutionState, message *types.Message) (bool, error) {
	if !features.EnableSignatureCheck {
		return true, nil
	}
	// TODO: Add internal message validation logic
	if message.Internal {
		return true, nil
	}
	addr := message.From
	accountState := es.GetAccount(addr)

	r := &types.Receipt{
		Success:         false,
		GasUsed:         0,
		MsgHash:         es.InMessageHash,
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
