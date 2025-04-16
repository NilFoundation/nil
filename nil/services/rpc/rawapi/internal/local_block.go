package internal

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func (api *localShardApiRo) GetBlockHeader(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
) (sszx.SSZEncodedData, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, err := api.getBlockByReference(tx, blockReference, false)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

func (api *localShardApiRo) GetFullBlockData(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
) (*types.RawBlockWithExtractedData, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return api.getBlockByReference(tx, blockReference, true)
}

func (api *localShardApiRo) GetBlockTransactionCount(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
) (uint64, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := api.getBlockByReference(tx, blockReference, true)
	if err != nil {
		return 0, err
	}
	return uint64(len(res.InTransactions)), nil
}

func (api *localShardApiRo) getBlockByReference(
	tx db.RoTx,
	blockReference rawapitypes.BlockReference,
	withTransactions bool,
) (*types.RawBlockWithExtractedData, error) {
	blockHash, err := api.getBlockHashByReference(tx, blockReference)
	if err != nil {
		return nil, err
	}

	return api.getBlockByHash(tx, blockHash, withTransactions)
}

func (api *localShardApiRo) getBlockHashByReference(
	tx db.RoTx,
	blockReference rawapitypes.BlockReference,
) (common.Hash, error) {
	switch blockReference.Type() {
	case rawapitypes.NumberBlockReference:
		return db.ReadBlockHashByNumber(tx, api.shardId(), types.BlockNumber(blockReference.Number()))
	case rawapitypes.NamedBlockIdentifierReference:
		switch blockReference.NamedBlockIdentifier() {
		case rawapitypes.EarliestBlock:
			return db.ReadBlockHashByNumber(tx, api.shardId(), 0)
		case rawapitypes.LatestBlock, rawapitypes.PendingBlock:
			return db.ReadLastBlockHash(tx, api.shardId())
		}
		return common.EmptyHash, errors.New("unknown named block identifier")
	case rawapitypes.HashBlockReference:
		return blockReference.Hash(), nil
	}
	return common.EmptyHash, errors.New("unknown block reference type")
}

func (api *localShardApiRo) getBlockByHash(
	tx db.RoTx,
	hash common.Hash,
	withTransactions bool,
) (*types.RawBlockWithExtractedData, error) {
	accessor := api.accessor.RawAccess(tx, api.shardId()).GetBlock()
	if withTransactions {
		accessor = accessor.
			WithInTransactions().
			WithOutTransactions().
			WithReceipts().
			WithChildBlocks().
			WithDbTimestamp().
			WithConfig()
	}

	data, err := accessor.ByHash(hash)
	if err != nil {
		return nil, err
	}

	if data.Block() == nil {
		return nil, nil
	}

	if assert.Enable {
		var block types.Block
		if err := block.UnmarshalSSZ(data.Block()); err != nil {
			return nil, err
		}
		blockHash := block.Hash(api.shardId())
		check.PanicIfNotf(blockHash == hash, "block hash mismatch: %s != %s", blockHash, hash)
	}

	result := &types.RawBlockWithExtractedData{
		Block: data.Block(),
	}
	if withTransactions {
		result.InTransactions = data.InTransactions()
		result.InTxCounts = data.InTxCounts()
		result.OutTransactions = data.OutTransactions()
		result.OutTxCounts = data.OutTxCounts()
		result.Receipts = data.Receipts()
		result.Errors = make(map[common.Hash]string)
		result.ChildBlocks = data.ChildBlocks()
		result.DbTimestamp = data.DbTimestamp()
		result.Config = data.Config()

		// Need to decode transactions to get its hashes because external transaction hash
		// calculated in a bit different way (not just Hash(SSZ)).
		transactions, err := sszx.DecodeContainer[*types.Transaction](result.InTransactions)
		if err != nil {
			return nil, err
		}
		for _, transaction := range transactions {
			txnHash := transaction.Hash()
			errMsg, err := db.ReadError(tx, txnHash)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			if len(errMsg) > 0 {
				result.Errors[txnHash] = errMsg
			}
		}
	}
	return result, nil
}
