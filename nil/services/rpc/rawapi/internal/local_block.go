package internal

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

func (api *localShardApiRo) GetBlockHeader(
	ctx context.Context,
	blockReference rawapitypes.BlockReference,
) (serialization.EncodedData, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, err := api.getRawHeaderByRef(tx, blockReference)
	if err != nil {
		return nil, err
	}
	return block, nil
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
	return api.getFullBlockByRef(tx, blockReference)
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

	// We're caching blocks, so taking a full block is not a serious problem.
	// (Unless we're attacked by transaction count lovers.)
	res, err := api.getFullBlockByRef(tx, blockReference)
	if err != nil {
		return 0, err
	}
	return uint64(len(res.InTransactions)), nil
}

func handleBlockFetchError(err error) error {
	if errors.Is(err, db.ErrKeyNotFound) {
		return rawapitypes.ErrBlockNotFound
	}
	return err
}

// getBlockByReference tries to fetch the block from db, if such block is not in the db, `rawapitypes.ErrBlockNotFound`
func (api *localShardApiRo) getRawHeaderByRef(tx db.RoTx, ref rawapitypes.BlockReference) ([]byte, error) {
	hash, err := api.getBlockHashByRef(tx, ref)
	if err != nil {
		return nil, handleBlockFetchError(err)
	}

	res, err := api.accessor.Access(tx, api.shardId()).GetRawBlockHeaderByHash(hash)
	if err != nil {
		return nil, handleBlockFetchError(err)
	}

	return res, nil
}

func (api *localShardApiRo) getHeaderByRef(tx db.RoTx, ref rawapitypes.BlockReference) (*types.Block, error) {
	hash, err := api.getBlockHashByRef(tx, ref)
	if err != nil {
		return nil, handleBlockFetchError(err)
	}

	res, err := api.accessor.Access(tx, api.shardId()).GetBlockHeaderByHash(hash)
	if err != nil {
		return nil, handleBlockFetchError(err)
	}
	return res, nil
}

func (api *localShardApiRo) getFullBlockByRef(
	tx db.RoTx, ref rawapitypes.BlockReference,
) (*types.RawBlockWithExtractedData, error) {
	hash, err := api.getBlockHashByRef(tx, ref)
	if err != nil {
		return nil, handleBlockFetchError(err)
	}

	result, err := api.accessor.Access(tx, api.shardId()).GetFullBlockByHash(hash)
	if err != nil {
		return nil, handleBlockFetchError(err)
	}

	return fixTxns(tx, result)
}

func (api *localShardApiRo) getBlockHashByRef(
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

func fixTxns(tx db.RoTx, result *types.RawBlockWithExtractedData) (*types.RawBlockWithExtractedData, error) {
	// Need to decode transactions to get its hashes because external transaction hash
	// calculated in a bit different way (not just Hash(bytes)).
	transactions, err := serialization.DecodeContainer[*types.Transaction](result.InTransactions)
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
			if result.Errors == nil {
				result.Errors = make(map[common.Hash]string)
			}
			result.Errors[txnHash] = errMsg
		}
	}

	return result, nil
}
