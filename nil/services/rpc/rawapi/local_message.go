package rawapi

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func (api *LocalShardApi) getMessageByHash(tx db.RoTx, hash common.Hash) (*rawapitypes.MessageInfo, error) {
	data, err := api.accessor.Access(tx, api.ShardId).GetInMessage().WithReceipt().ByHash(hash)
	if err != nil {
		return nil, err
	}

	msg := data.Message()
	messageSSZ, err := msg.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	receipt := data.Receipt()
	receiptSSZ, err := receipt.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal receipt: %w", err)
	}

	block := data.Block()
	return &rawapitypes.MessageInfo{
		MessageSSZ: messageSSZ,
		ReceiptSSZ: receiptSSZ,
		Index:      data.Index(),
		BlockHash:  block.Hash(api.ShardId),
		BlockId:    block.Id,
	}, nil
}

func getRawBlockEntity(
	tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte,
) ([]byte, error) {
	root := mpt.NewDbReader(tx, shardId, tableName)
	root.SetRootHash(rootHash)
	entityBytes, err := root.Get(entityKey)
	if err != nil {
		return nil, err
	}
	return entityBytes, nil
}

func (api *LocalShardApi) getInMessageByBlockHashAndIndex(
	tx db.RoTx, block *types.Block, msgIndex types.MessageIndex,
) (*rawapitypes.MessageInfo, error) {
	rawMsg, err := getRawBlockEntity(tx, api.ShardId, db.MessageTrieTable, block.InMessagesRoot, msgIndex.Bytes())
	if err != nil {
		return nil, err
	}

	rawReceipt, err := getRawBlockEntity(tx, api.ShardId, db.ReceiptTrieTable, block.ReceiptsRoot, msgIndex.Bytes())
	if err != nil {
		return nil, err
	}

	return &rawapitypes.MessageInfo{
		MessageSSZ: rawMsg,
		ReceiptSSZ: rawReceipt,
		Index:      msgIndex,
		BlockHash:  block.Hash(api.ShardId),
		BlockId:    block.Id,
	}, nil
}

func (api *LocalShardApi) fetchBlockByRef(tx db.RoTx, blockRef rawapitypes.BlockReference) (*types.Block, error) {
	hash, err := api.getBlockHashByReference(tx, blockRef)
	if err != nil {
		return nil, err
	}

	data, err := api.accessor.Access(tx, api.ShardId).GetBlock().ByHash(hash)
	if err != nil {
		return nil, err
	}
	return data.Block(), nil
}

func (api *LocalShardApi) getInMessageByBlockRefAndIndex(
	tx db.RoTx, blockRef rawapitypes.BlockReference, index types.MessageIndex,
) (*rawapitypes.MessageInfo, error) {
	block, err := api.fetchBlockByRef(tx, blockRef)
	if err != nil {
		return nil, err
	}
	return api.getInMessageByBlockHashAndIndex(tx, block, index)
}

func (api *LocalShardApi) GetInMessage(ctx context.Context, request rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	if request.ByHash != nil {
		return api.getMessageByHash(tx, request.ByHash.Hash)
	}
	return api.getInMessageByBlockRefAndIndex(tx, request.ByBlockRefAndIndex.BlockRef, request.ByBlockRefAndIndex.Index)
}
