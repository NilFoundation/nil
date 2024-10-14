package rawapi

import (
	"context"
	"errors"
	"fmt"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func (api *LocalShardApi) GetInMessageReceipt(ctx context.Context, hash common.Hash) (*rawapitypes.ReceiptInfo, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, indexes, err := api.getBlockAndInMessageIndexByMessageHash(tx, api.ShardId, hash)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	var receipt *types.Receipt
	var gasPrice types.Value

	includedInMain := false
	if block != nil {
		receipt, err = getBlockEntity[*types.Receipt](tx, api.ShardId, db.ReceiptTrieTable, block.ReceiptsRoot, indexes.MessageIndex.Bytes())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}
		gasPrice = block.GasPrice

		// Check if the message is included in the main chain
		rawMainBlock, err := api.nodeApi.GetFullBlockData(ctx, types.MainShardId, rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock))
		if err == nil {
			mainBlockData, err := rawMainBlock.DecodeSSZ()
			if err != nil {
				return nil, err
			}

			if api.ShardId.IsMainShard() {
				includedInMain = mainBlockData.Id >= block.Id
			} else {
				if len(rawMainBlock.ChildBlocks) < int(api.ShardId) {
					return nil, errShardNotFound
				}
				blockHash := rawMainBlock.ChildBlocks[api.ShardId-1]
				if last, err := api.accessor.Access(tx, api.ShardId).GetBlock().ByHash(blockHash); err == nil {
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

	var outReceipts []*rawapitypes.ReceiptInfo = nil
	var outMessages []common.Hash = nil

	if receipt.OutMsgNum != 0 {
		outReceipts = make([]*rawapitypes.ReceiptInfo, 0, receipt.OutMsgNum)
		for i := receipt.OutMsgIndex; i < receipt.OutMsgIndex+receipt.OutMsgNum; i++ {
			res, err := api.accessor.Access(tx, api.ShardId).GetOutMessage().ByIndex(types.MessageIndex(i), block)
			if err != nil {
				return nil, err
			}
			msgHash := res.Message().Hash()
			r, err := api.nodeApi.GetInMessageReceipt(ctx, res.Message().To.ShardId(), msgHash)
			if err != nil {
				return nil, err
			}
			outReceipts = append(outReceipts, r)
			outMessages = append(outMessages, msgHash)
		}
	}

	var receiptSSZ []byte
	if receipt != nil {
		receiptSSZ, err = receipt.MarshalSSZ()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal receipt: %w", err)
		}
	}

	var blockId types.BlockNumber
	var blockHash common.Hash
	if block != nil {
		blockId = block.Id
		blockHash = block.Hash()
	}

	return &rawapitypes.ReceiptInfo{
		ShardId:        api.ShardId,
		ReceiptSSZ:     receiptSSZ,
		Index:          indexes.MessageIndex,
		BlockHash:      blockHash,
		BlockId:        blockId,
		IncludedInMain: includedInMain,
		OutReceipts:    outReceipts,
		OutMessages:    outMessages,
		ErrorMessage:   errMsg,
		GasPrice:       gasPrice,
		Temporary:      cachedReceipt,
	}, nil
}

func (api *LocalShardApi) getBlockAndInMessageIndexByMessageHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, db.BlockHashAndMessageIndex, error) {
	var index db.BlockHashAndMessageIndex
	value, err := tx.GetFromShard(shardId, db.BlockHashAndInMessageIndexByMessageHash, hash.Bytes())
	if err != nil {
		return nil, index, err
	}
	if err := index.UnmarshalSSZ(value); err != nil {
		return nil, index, err
	}

	data, err := api.accessor.Access(tx, shardId).GetBlock().ByHash(index.BlockHash)
	if err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}
	return data.Block(), index, nil
}

func getBlockEntity[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
](tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte) (*S, error) {
	root := mpt.NewDbReader(tx, shardId, tableName)
	root.SetRootHash(rootHash)
	return mpt.GetEntity[T](root, entityKey)
}
