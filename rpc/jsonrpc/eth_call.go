package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

// Call implements eth_call. Executes a new message call immediately without creating a transaction on the block chain.
func (api *APIImpl) Call(ctx context.Context, args CallArgs, blockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	timer := common.NewTimer()
	shardId := args.From.ShardId()

	hash, err := extractBlockHash(tx, shardId, blockNrOrHash)
	if err != nil {
		return nil, err
	}

	es, err := execution.NewROExecutionState(tx, shardId, hash, timer, 1)
	if err != nil {
		return nil, err
	}

	if overrides != nil {
		if err := overrides.Override(es); err != nil {
			return nil, err
		}
	}

	msg := &types.Message{
		ChainId:   types.DefaultChainId,
		Seqno:     types.Seqno(args.Seqno),
		FeeCredit: args.FeeCredit,
		From:      args.From,
		To:        args.To,
		Value:     args.Value,
		Data:      types.Code(args.Data),
	}

	es.AddInMessage(msg)
	res := es.HandleMessage(ctx, msg, execution.SimPayer{})
	if res.IsFatal() {
		return nil, res.FatalError
	}
	if res.Error != nil {
		return nil, res.Error.Unwrap()
	}
	outMessages := es.OutMessages[msg.Hash()]
	return &CallRes{
		Data:        res.ReturnData,
		CoinsUsed:   res.CoinsUsed,
		OutMessages: outMessages,
	}, nil
}
