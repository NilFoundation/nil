package rawapi

import (
	"context"
	"errors"
	"fmt"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
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

	block, indexes, err := api.getBlockAndInTransactionIndexByTransactionHash(tx, api.shardId(), hash)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	var receipt *types.Receipt
	var transaction *types.Transaction
	var gasPrice types.Value

	includedInMain := false
	if block != nil {
		receipt, err = getBlockEntity[*types.Receipt](
			tx, api.shardId(), db.ReceiptTrieTable, block.ReceiptsRoot, indexes.TransactionIndex.Bytes())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		transaction, err = getBlockEntity[*types.Transaction](
			tx,
			api.shardId(),
			db.TransactionTrieTable,
			block.InTransactionsRoot,
			indexes.TransactionIndex.Bytes())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}

		if priorityFee, ok := execution.GetEffectivePriorityFee(block.BaseFee, transaction); ok {
			gasPrice = block.BaseFee.Add(priorityFee)
		} else if receipt.Status != types.ErrorBaseFeeTooHigh {
			api.logger.Error().
				Stringer(logging.FieldTransactionHash, hash).
				Msgf("Calculation of EffectivePriorityFee failed with wrong status: %s", receipt.Status)
		}

		// Check if the transaction is included in the main chain
		rawMainBlock, err := api.nodeApi.GetFullBlockData(
			ctx,
			types.MainShardId,
			rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
		if err == nil {
			mainBlockData, err := rawMainBlock.DecodeSSZ()
			if err != nil {
				return nil, err
			}

			if api.shardId().IsMainShard() {
				includedInMain = mainBlockData.Id >= block.Id
			} else {
				if len(rawMainBlock.ChildBlocks) < int(api.shardId()) {
					return nil, fmt.Errorf(
						"%w: main shard includes only %d blocks",
						makeShardNotFoundError(methodNameChecked("GetInTransactionReceipt"), api.shardId()),
						len(rawMainBlock.ChildBlocks))
				}
				blockHash := rawMainBlock.ChildBlocks[api.shardId()-1]
				if last, err := api.accessor.Access(tx, api.shardId()).GetBlock().ByHash(blockHash); err == nil {
					includedInMain = last.Block().Id >= block.Id
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

	var outReceipts []*rawapitypes.ReceiptInfo
	var outTransactions []common.Hash

	if receipt.OutTxnNum != 0 {
		outReceipts = make([]*rawapitypes.ReceiptInfo, 0, receipt.OutTxnNum)
		for i := receipt.OutTxnIndex; i < receipt.OutTxnIndex+receipt.OutTxnNum; i++ {
			res, err := api.accessor.
				Access(tx, api.shardId()).
				GetOutTransaction().
				ByIndex(types.TransactionIndex(i), block)
			if err != nil {
				return nil, err
			}
			txnHash := res.Transaction().Hash()
			r, err := api.nodeApi.GetInTransactionReceipt(ctx, res.Transaction().To.ShardId(), txnHash)
			if err != nil {
				return nil, err
			}
			outReceipts = append(outReceipts, r)
			outTransactions = append(outTransactions, txnHash)
		}
	}

	var receiptSSZ []byte
	if receipt != nil {
		receiptSSZ, err = receipt.MarshalSSZ()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal receipt: %w", err)
		}
	}

	var flags types.TransactionFlags
	if transaction != nil {
		flags = transaction.Flags
	}

	var blockId types.BlockNumber
	var blockHash common.Hash
	if block != nil {
		blockId = block.Id
		blockHash = block.Hash(api.shardId())
	}

	return &rawapitypes.ReceiptInfo{
		ReceiptSSZ:      receiptSSZ,
		Flags:           flags,
		Index:           indexes.TransactionIndex,
		BlockHash:       blockHash,
		BlockId:         blockId,
		IncludedInMain:  includedInMain,
		OutReceipts:     outReceipts,
		OutTransactions: outTransactions,
		ErrorMessage:    errMsg,
		GasPrice:        gasPrice,
		Temporary:       cachedReceipt,
	}, nil
}

func (api *localShardApiRo) getBlockAndInTransactionIndexByTransactionHash(
	tx db.RoTx,
	shardId types.ShardId,
	hash common.Hash,
) (*types.Block, db.BlockHashAndTransactionIndex, error) {
	var index db.BlockHashAndTransactionIndex
	value, err := tx.GetFromShard(shardId, db.BlockHashAndInTransactionIndexByTransactionHash, hash.Bytes())
	if err != nil {
		return nil, index, err
	}
	if err := index.UnmarshalSSZ(value); err != nil {
		return nil, index, err
	}

	data, err := api.accessor.Access(tx, shardId).GetBlock().ByHash(index.BlockHash)
	if err != nil {
		return nil, db.BlockHashAndTransactionIndex{}, err
	}
	return data.Block(), index, nil
}

func getBlockEntity[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
](
	tx db.RoTx,
	shardId types.ShardId,
	tableName db.ShardedTableName,
	rootHash common.Hash,
	entityKey []byte,
) (*S, error) {
	root := mpt.NewDbReader(tx, shardId, tableName)
	root.SetRootHash(rootHash)
	return mpt.GetEntity[T](root, entityKey)
}
