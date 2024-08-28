package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
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
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	var receipt *types.Receipt
	var gasPrice types.Value

	includedInMain := false

	if block != nil {
		receipt, err = getBlockEntity[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, indexes.MessageIndex.Bytes())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		gasPrice = block.GasPrice

		// Check if the message is included in the main chain
		if mainBlock, err := db.ReadLastBlock(tx, types.MainShardId); err == nil {
			treeShards := execution.NewDbShardBlocksTrieReader(tx, types.MainShardId, mainBlock.Id)
			treeShards.SetRootHash(mainBlock.ChildBlocksRootHash)
			hashBytes, err := treeShards.Get(shardId.Bytes())
			if err == nil {
				hash := common.BytesToHash(hashBytes)
				if lastBlock, err := db.ReadBlock(tx, shardId, hash); err == nil {
					includedInMain = lastBlock.Id >= block.Id
				}
			}
		}
	} else {
		gasPrice = types.DefaultGasPrice
	}

	var errMsg string
	var cachedReceipt bool
	if receipt == nil {
		var receiptWithError execution.ReceiptWithError
		receiptWithError, cachedReceipt = execution.FailureReceiptCache.Get(hash)
		if !cachedReceipt {
			return nil, nil
		}

		receipt = receiptWithError.Receipt
		errMsg = receiptWithError.Error.Error()
	} else {
		errMsg, err = db.ReadError(tx, hash)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
	}

	var outReceipts []*RPCReceipt = nil
	var outMessages []common.Hash = nil

	if receipt.OutMsgNum != 0 {
		outReceipts = make([]*RPCReceipt, 0, receipt.OutMsgNum)

		for i := receipt.OutMsgIndex; i < receipt.OutMsgIndex+receipt.OutMsgNum; i++ {
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
			outMessages = append(outMessages, res.Message().Hash())
		}
	}

	res := NewRPCReceipt(shardId, block, indexes.MessageIndex, receipt, outMessages, outReceipts, cachedReceipt, errMsg,
		gasPrice)
	res.IncludedInMain = includedInMain

	return res, nil
}
