package internal

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func (api *localShardApiRo) getTransactionByHash(tx db.RoTx, hash common.Hash) (*rawapitypes.TransactionInfo, error) {
	data, err := api.accessor.Access(tx, api.shardId()).GetInTransaction().WithReceipt().ByHash(hash)
	if err != nil {
		return nil, err
	}

	txn := data.Transaction()
	transactionBytes, err := txn.MarshalNil()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	receipt := data.Receipt()
	receiptBytes, err := receipt.MarshalNil()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal receipt: %w", err)
	}

	block := data.Block()
	return &rawapitypes.TransactionInfo{
		TransactionBytes: transactionBytes,
		ReceiptBytes:     receiptBytes,
		Index:            data.Index(),
		BlockHash:        block.Hash(api.shardId()),
		BlockId:          block.Id,
	}, nil
}

func getRawBlockEntity(
	tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte,
) ([]byte, error) {
	root := mpt.NewDbReader(tx, shardId, tableName)
	if err := root.SetRootHash(rootHash); err != nil {
		return nil, err
	}
	entityBytes, err := root.Get(entityKey)
	if err != nil {
		return nil, err
	}
	return entityBytes, nil
}

func (api *localShardApiRo) getInTransactionByBlockHashAndIndex(
	tx db.RoTx, block *types.Block, txnIndex types.TransactionIndex,
) (*rawapitypes.TransactionInfo, error) {
	rawTxn, err := getRawBlockEntity(
		tx, api.shardId(), db.TransactionTrieTable, block.InTransactionsRoot, txnIndex.Bytes())
	if err != nil {
		return nil, err
	}

	rawReceipt, err := getRawBlockEntity(tx, api.shardId(), db.ReceiptTrieTable, block.ReceiptsRoot, txnIndex.Bytes())
	if err != nil {
		return nil, err
	}

	return &rawapitypes.TransactionInfo{
		TransactionBytes: rawTxn,
		ReceiptBytes:     rawReceipt,
		Index:            txnIndex,
		BlockHash:        block.Hash(api.shardId()),
		BlockId:          block.Id,
	}, nil
}

func (api *localShardApiRo) fetchBlockByRef(tx db.RoTx, blockRef rawapitypes.BlockReference) (*types.Block, error) {
	hash, err := api.getBlockHashByReference(tx, blockRef)
	if err != nil {
		return nil, err
	}

	data, err := api.accessor.Access(tx, api.shardId()).GetBlock().ByHash(hash)
	if err != nil {
		return nil, err
	}
	return data.Block(), nil
}

func (api *localShardApiRo) getInTransactionByBlockRefAndIndex(
	tx db.RoTx, blockRef rawapitypes.BlockReference, index types.TransactionIndex,
) (*rawapitypes.TransactionInfo, error) {
	block, err := api.fetchBlockByRef(tx, blockRef)
	if err != nil {
		return nil, err
	}
	return api.getInTransactionByBlockHashAndIndex(tx, block, index)
}

func (api *localShardApiRo) GetInTransaction(
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
