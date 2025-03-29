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

func (api *LocalShardApi) getTransactionByHash(tx db.RoTx, hash common.Hash) (*rawapitypes.TransactionInfo, error) {
	data, err := api.accessor.Access(tx, api.ShardId).GetInTransaction().WithReceipt().ByHash(hash)
	if err != nil {
		return nil, err
	}

	txn := data.Transaction()
	transactionSSZ, err := txn.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	receipt := data.Receipt()
	receiptSSZ, err := receipt.MarshalSSZ()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal receipt: %w", err)
	}

	block := data.Block()
	return &rawapitypes.TransactionInfo{
		TransactionSSZ: transactionSSZ,
		ReceiptSSZ:     receiptSSZ,
		Index:          data.Index(),
		BlockHash:      block.Hash(api.ShardId),
		BlockId:        block.Id,
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

func (api *LocalShardApi) getInTransactionByBlockHashAndIndex(
	tx db.RoTx, block *types.Block, txnIndex types.TransactionIndex,
) (*rawapitypes.TransactionInfo, error) {
	rawTxn, err := getRawBlockEntity(
		tx, api.ShardId, db.TransactionTrieTable, block.InTransactionsRoot, txnIndex.Bytes())
	if err != nil {
		return nil, err
	}

	rawReceipt, err := getRawBlockEntity(tx, api.ShardId, db.ReceiptTrieTable, block.ReceiptsRoot, txnIndex.Bytes())
	if err != nil {
		return nil, err
	}

	return &rawapitypes.TransactionInfo{
		TransactionSSZ: rawTxn,
		ReceiptSSZ:     rawReceipt,
		Index:          txnIndex,
		BlockHash:      block.Hash(api.ShardId),
		BlockId:        block.Id,
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

func (api *LocalShardApi) getInTransactionByBlockRefAndIndex(
	tx db.RoTx, blockRef rawapitypes.BlockReference, index types.TransactionIndex,
) (*rawapitypes.TransactionInfo, error) {
	block, err := api.fetchBlockByRef(tx, blockRef)
	if err != nil {
		return nil, err
	}
	return api.getInTransactionByBlockHashAndIndex(tx, block, index)
}

func (api *LocalShardApi) GetInTransaction(
	ctx context.Context,
	request rawapitypes.TransactionRequest,
) (*rawapitypes.TransactionInfo, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	if request.ByHash != nil {
		return api.getTransactionByHash(tx, request.ByHash.Hash)
	}
	return api.getInTransactionByBlockRefAndIndex(
		tx, request.ByBlockRefAndIndex.BlockRef, request.ByBlockRefAndIndex.Index)
}
