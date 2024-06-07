package shardchain

import (
	"bytes"
	"context"
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
	"github.com/NilFoundation/nil/features"
)

func HandleMessages(ctx context.Context, tx db.RoTx, es *execution.ExecutionState, msgs []*types.Message) error {
	blockContext := execution.NewEVMBlockContext(es)
	for _, message := range msgs {
		msgHash := message.Hash()
		es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := validateMessage(tx, es, message)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		// TODO: disallow sending messages to precompiled contracts
		isPrecompiled := bytes.Equal(message.To[:19], common.EmptyHash[:19])

		// Deploy message
		if !isPrecompiled && !es.ContractExists(message.To) {
			deployMsg := validateDeployMessage(es, message)
			if deployMsg == nil {
				continue
			}

			if err := es.HandleDeployMessage(message, deployMsg, &blockContext); err != nil && !errors.Is(err, new(vm.VMError)) {
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

func validateDeployMessage(es *execution.ExecutionState, message *types.Message) *types.DeployMessage {
	fail := func(err error, message string) *types.DeployMessage {
		r := &types.Receipt{
			Success: false,
			GasUsed: 0,
			MsgHash: es.InMessageHash,
		}
		es.AddReceipt(r)
		sharedLogger.Debug().Err(err).Stringer("hash", es.InMessageHash).Msg(message)
		return nil
	}

	deployMsg, err := types.NewDeployMessage(message.Data)
	if err != nil {
		return fail(err, "Invalid deploy message")
	}

	if types.IsMasterShard(deployMsg.ShardId) {
		return fail(nil, "Attempt to deploy to master shard")
	}

	if message.To != types.DeployMsgToAddress(deployMsg, message.From) {
		return fail(nil, "Incorrect deployment address")
	}

	return deployMsg
}

func validateMessage(tx db.RoTx, es *execution.ExecutionState, message *types.Message) (bool, error) {
	if !features.EnableSignatureCheck {
		return true, nil
	}
	if message.Internal {
		fromId := message.From.ShardId()
		msg, _, _, _, err := es.Accessor.GetMessageWithEntitiesByHash(tx, fromId, message.Hash())
		if err != nil {
			return false, err
		}
		return msg != nil, nil
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

	ok, err := message.ValidateSignature(accountState.PublicKey[:])
	if err != nil {
		return false, err
	}
	if !ok {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().Stringer("shardId", es.ShardId).Stringer("address", addr).Msg("Invalid signature")
		return false, nil
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
