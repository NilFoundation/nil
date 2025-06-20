package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func (api *localShardApiRo) GetInTransactionReceipt(
	ctx context.Context,
	hash common.Hash,
) (*rawapitypes.ReceiptInfo, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	txn, err := api.accessor.Access(tx, api.shardId()).GetInTxnByHash(hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		receiptWithError, cachedReceipt := execution.FailureReceiptCache.Get(hash)
		if !cachedReceipt {
			// If the transaction is not found and there is no cached receipt, we have nothing to return.
			return nil, nil
		}

		receiptBytes, err := receiptWithError.Receipt.MarshalNil()
		if err != nil {
			return nil, err
		}
		return &rawapitypes.ReceiptInfo{
			ReceiptBytes: receiptBytes,
			ErrorMessage: receiptWithError.Error.Error(),
			Temporary:    true,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	var gasPrice types.Value
	includedInMain := false
	if priorityFee, ok := execution.GetEffectivePriorityFee(txn.Block.BaseFee, txn.Transaction); ok {
		gasPrice = txn.Block.BaseFee.Add(priorityFee)
	} else if txn.Receipt.Status != types.ErrorBaseFeeTooHigh {
		api.logger.Error().
			Stringer(logging.FieldTransactionHash, hash).
			Msgf("Calculation of EffectivePriorityFee failed with wrong status: %s", txn.Receipt.Status)
	}

	// Check if the transaction is included in the main chain
	rawMainBlock, err := api.nodeApi.GetFullBlockData(ctx, types.MainShardId,
		rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
	if err == nil {
		mainBlockData, err := rawMainBlock.DecodeBytes()
		if err != nil {
			return nil, err
		}

		if api.shardId().IsMainShard() {
			includedInMain = mainBlockData.Id >= txn.Block.Id
		} else {
			if len(rawMainBlock.ChildBlocks) < int(api.shardId()) {
				return nil, fmt.Errorf(
					"%w: main shard includes only %d blocks",
					makeShardNotFoundError(methodNameChecked("GetInTransactionReceipt"), api.shardId()),
					len(rawMainBlock.ChildBlocks))
			}
			blockHash := rawMainBlock.ChildBlocks[api.shardId()-1]
			if last, err := api.accessor.Access(tx, api.shardId()).GetBlockHeaderByHash(blockHash); err == nil {
				includedInMain = last.Id >= txn.Block.Id
			}
		}
	}

	errMsg, err := db.ReadError(tx, hash)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	var outReceipts []*rawapitypes.ReceiptInfo
	var outTransactions []common.Hash

	if txn.Receipt.OutTxnNum != 0 {
		outReceipts = make([]*rawapitypes.ReceiptInfo, 0, txn.Receipt.OutTxnNum)
		outTransactions = make([]common.Hash, 0, txn.Receipt.OutTxnNum)
		for i := txn.Receipt.OutTxnIndex; i < txn.Receipt.OutTxnIndex+txn.Receipt.OutTxnNum; i++ {
			res, err := api.accessor.Access(tx, api.shardId()).
				GetOutTxnByIndex(types.TransactionIndex(i), txn.Block)
			if err != nil {
				return nil, err
			}
			txnHash := res.Transaction.Hash()
			r, err := api.nodeApi.GetInTransactionReceipt(ctx, res.Transaction.To.ShardId(), txnHash)
			if err != nil {
				return nil, err
			}
			outReceipts = append(outReceipts, r)
			outTransactions = append(outTransactions, txnHash)
		}
	}

	return &rawapitypes.ReceiptInfo{
		ReceiptBytes:    txn.RawReceipt,
		Flags:           txn.Transaction.Flags,
		Index:           txn.Index,
		BlockHash:       txn.Block.Hash,
		BlockId:         txn.Block.Id,
		IncludedInMain:  includedInMain,
		OutReceipts:     outReceipts,
		OutTransactions: outTransactions,
		ErrorMessage:    errMsg,
		GasPrice:        gasPrice,
	}, nil
}
