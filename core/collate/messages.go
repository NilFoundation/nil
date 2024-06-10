package collate

import (
	"bytes"
	"context"
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/core/vm"
)

func HandleMessages(ctx context.Context, roTx db.RoTx, es *execution.ExecutionState, msgs []*types.Message) error {
	blockContext := execution.NewEVMBlockContext(es)
	for _, message := range msgs {
		msgHash := message.Hash()
		es.AddInMessage(message)
		es.InMessageHash = msgHash

		ok, err := validateMessage(roTx, es, message)
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

			if err := es.HandleDeployMessage(ctx, message, deployMsg, &blockContext); err != nil && !errors.Is(err, new(vm.VMError)) {
				return err
			}
		} else {
			if _, err := es.HandleExecutionMessage(ctx, message, &blockContext); err != nil && !errors.Is(err, new(vm.VMError)) {
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
		sharedLogger.Debug().Err(err).Stringer(logging.FieldMessageHash, es.InMessageHash).Msg(message)
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

func validateMessage(roTx db.RoTx, es *execution.ExecutionState, message *types.Message) (bool, error) {
	if message.Internal {
		fromId := message.From.ShardId()
		msg, _, _, _, err := es.Accessor.GetMessageWithEntitiesByHash(roTx, fromId, message.Hash())
		if err != nil {
			return false, err
		}
		return msg != nil, nil
	}

	addr := message.From
	r := &types.Receipt{
		Success:         false,
		GasUsed:         0,
		MsgHash:         es.InMessageHash,
		ContractAddress: addr,
	}

	accountState := es.GetAccount(addr)
	if accountState == nil {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Msg("Invalid address.")
		return false, nil
	}

	ok := es.CallValidateExternal(message, accountState)

	if !ok {
		ok2, err := message.ValidateSignature(accountState.PublicKey[:])
		if err != nil {
			return false, err
		}
		ok = ok2
	}

	if !ok {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Msg("Invalid signature.")
		return false, nil
	}

	if accountState.Seqno != message.Seqno {
		r.Logs = es.Logs[es.InMessageHash]
		es.AddReceipt(r)
		sharedLogger.Debug().
			Stringer(logging.FieldShardId, es.ShardId).
			Stringer(logging.FieldMessageFrom, addr).
			Uint64(logging.FieldAccountSeqno, accountState.Seqno.Uint64()).
			Uint64(logging.FieldMessageSeqno, message.Seqno.Uint64()).
			Msg("Seqno gap")
		return false, nil
	}

	return true, nil
}
