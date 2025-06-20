package internal

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func convertTxnInfo(scr *execution.Txn) *rawapitypes.TransactionInfo {
	return &rawapitypes.TransactionInfo{
		TransactionBytes: scr.RawTxn,
		ReceiptBytes:     scr.RawReceipt,
		Index:            scr.Index,
		BlockHash:        scr.Block.Hash,
		BlockId:          scr.Block.Id,
	}
}

func (api *localShardApiRo) getTxnByHash(tx db.RoTx, hash common.Hash) (*rawapitypes.TransactionInfo, error) {
	data, err := api.accessor.Access(tx, api.shardId()).GetInTxnByHash(hash)
	if err != nil {
		return nil, err
	}

	return convertTxnInfo(data), nil
}

func (api *localShardApiRo) getInTxnByBlockRefAndIndex(
	tx db.RoTx, blockRef rawapitypes.BlockReference, index types.TransactionIndex,
) (*rawapitypes.TransactionInfo, error) {
	blockHash, err := api.getBlockHashByRef(tx, blockRef)
	if err != nil {
		return nil, err
	}

	block, err := api.accessor.Access(tx, api.shardId()).GetBlockHeaderByHash(blockHash)
	if err != nil {
		return nil, err
	}

	data, err := api.accessor.Access(tx, api.shardId()).
		GetInTxnByIndex(index, types.NewBlockWithRawHash(block, blockHash))
	if err != nil {
		return nil, err
	}
	return convertTxnInfo(data), nil
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
		return api.getTxnByHash(tx, request.ByHash.Hash)
	}
	return api.getInTxnByBlockRefAndIndex(
		tx, request.ByBlockRefAndIndex.BlockRef, request.ByBlockRefAndIndex.Index)
}
