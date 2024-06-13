package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
)

func (api *APIImpl) GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCReceipt, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, indexes, err := api.getBlockAndInMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	receipt, err := getBlockEntity[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, indexes.MessageIndex.Bytes())
	if err != nil {
		return nil, err
	}

	var outReceipts []*RPCReceipt = nil

	if receipt != nil && receipt.OutMsgNum != 0 {
		outReceipts = make([]*RPCReceipt, 0, receipt.OutMsgNum)

		for i := receipt.OutMsgIndex; i < receipt.OutMsgNum; i++ {
			sa, err := execution.NewStateAccessor()
			if err != nil {
				return nil, err
			}

			res, err := sa.Access(tx, shardId).GetOutMessage().ByIndex(types.MessageIndex(i), block)
			if err != nil {
				return nil, err
			}
			r, err := api.GetInMessageReceipt(ctx, res.Message().To.ShardId(), res.Message().Hash())
			if err != nil {
				return nil, err
			}
			outReceipts = append(outReceipts, r)
		}
	}

	return NewRPCReceipt(block, indexes.MessageIndex, receipt, outReceipts), nil
}
