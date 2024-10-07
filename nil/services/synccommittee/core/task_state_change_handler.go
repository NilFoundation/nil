package core

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	nilTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type taskStateChangeHandler struct {
	proposer     Proposer
	blockStorage storage.BlockStorage
	logger       zerolog.Logger
}

func newTaskStateChangeHandler(
	proposer Proposer,
	blockStorage storage.BlockStorage,
	logger zerolog.Logger,
) api.TaskStateChangeHandler {
	return &taskStateChangeHandler{
		proposer:     proposer,
		blockStorage: blockStorage,
		logger:       logger,
	}
}

func (h taskStateChangeHandler) OnTaskTerminated(ctx context.Context, task *types.Task, result *types.TaskResult) error {
	if task.TaskType != types.ProofBlock {
		return types.UnexpectedTaskType(task)
	}

	if !result.IsSuccess {
		h.logger.Error().
			Str("errorText", result.ErrorText).
			Interface("taskId", task.Id).
			Interface(logging.FieldShardId, nilTypes.MainShardId).
			Interface(logging.FieldBlockNumber, task.BlockNum).
			Msg("block proof task has failed, data won't be sent to the L1")
		return types.ErrBlockProofTaskFailed
	}

	if err := h.sendProof(ctx, task.ShardId, task.BlockNum); err != nil {
		return err
	}

	if err := h.blockStorage.SetLastProvedBlockNum(ctx, task.ShardId, task.BlockNum); err != nil {
		return err
	}

	return h.blockStorage.CleanupStorage(ctx)
}

// Send proof to the L1 network
func (h taskStateChangeHandler) sendProof(
	ctx context.Context,
	shardId nilTypes.ShardId,
	blockNumber nilTypes.BlockNumber,
) error {
	lastProvedBlockNum, err := h.blockStorage.GetLastProvedBlockNum(ctx, shardId)
	if err != nil {
		return err
	}

	lastProvedBlock, err := h.blockStorage.GetBlock(ctx, shardId, lastProvedBlockNum)
	if err != nil {
		return err
	}

	provedBlock, err := h.blockStorage.GetBlock(ctx, shardId, blockNumber)
	if err != nil {
		return err
	}

	if provedBlock == nil {
		return fmt.Errorf("can't get proved block %d from shard %d", blockNumber, shardId)
	}

	var oldStateRoot common.Hash
	if lastProvedBlock != nil {
		oldStateRoot = lastProvedBlock.ChildBlocksRootHash
	}
	newStateRoot := provedBlock.ChildBlocksRootHash
	transactions := extractTransactions(*provedBlock)

	h.logger.Info().
		Stringer("oldStateRoot", oldStateRoot).
		Stringer("newStateRoot", newStateRoot).
		Int64("blocksCount", int64(blockNumber-lastProvedBlockNum)).
		Int64("transactionsCount", int64(len(*transactions))).
		Msg("sending proof to the L1")

	err = h.proposer.SendProof(ctx, oldStateRoot, newStateRoot, transactions)
	if err != nil {
		return fmt.Errorf("failed to send proof: %w", err)
	}

	return nil
}

func extractTransactions(block jsonrpc.RPCBlock) *[]types.PrunedTransaction {
	var transactions []types.PrunedTransaction
	for _, message := range block.Messages {
		rpcInMessage, success := message.(jsonrpc.RPCInMessage)
		if !success {
			continue
		}

		t := types.PrunedTransaction{
			Flags: rpcInMessage.Flags,
			Seqno: rpcInMessage.Seqno,
			From:  rpcInMessage.From,
			To:    rpcInMessage.To,
			Value: rpcInMessage.Value,
			Data:  rpcInMessage.Data,
		}
		transactions = append(transactions, t)
	}
	return &transactions
}
