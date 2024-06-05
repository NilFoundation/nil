package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	fastssz "github.com/ferranbt/fastssz"
)

// GetInMessageByHash implements eth_getTransactioByHash. Returns the message structure
func (api *APIImpl) GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCInMessage, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	msg, receipt, index, block, err := api.accessor.GetMessageWithEntitiesByHash(tx, shardId, hash)
	if msg == nil || err != nil {
		return nil, err
	}
	return NewRPCInMessage(msg, receipt, index, block), nil
}

func (api *APIImpl) getInMessageByBlockNumberOrHashAndIndex(ctx context.Context, shardId types.ShardId,
	hashOrNum transport.BlockNumberOrHash, index hexutil.Uint64,
) (*RPCInMessage, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, err := api.fetchBlockByNumberOrHash(tx, shardId, hashOrNum)
	if err != nil || block == nil {
		return nil, err
	}

	msg, receipt, err := api.getInMessageByBlockHashAndIndex(tx, shardId, block, types.MessageIndex(index))
	if err != nil || msg == nil || receipt == nil {
		return nil, err
	}

	return NewRPCInMessage(msg, receipt, types.MessageIndex(index), block), nil
}

func (api *APIImpl) GetInMessageByBlockHashAndIndex(
	ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64,
) (*RPCInMessage, error) {
	return api.getInMessageByBlockNumberOrHashAndIndex(ctx, shardId, transport.BlockNumberOrHash{BlockHash: &hash}, index)
}

func (api *APIImpl) GetInMessageByBlockNumberAndIndex(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64,
) (*RPCInMessage, error) {
	return api.getInMessageByBlockNumberOrHashAndIndex(ctx, shardId, transport.BlockNumberOrHash{BlockNumber: &number}, index)
}

func (api *APIImpl) getRawInMessageByBlockNumberOrHashAndIndex(ctx context.Context, shardId types.ShardId,
	hashOrNum transport.BlockNumberOrHash, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, err := api.fetchBlockByNumberOrHash(tx, shardId, hashOrNum)
	if err != nil || block == nil {
		return nil, err
	}

	msgRaw, _, err := api.getRawInMessageByBlockHashAndIndex(tx, shardId, block, types.MessageIndex(index))
	return msgRaw, err
}

func (api *APIImpl) GetRawInMessageByBlockNumberAndIndex(
	ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	return api.getRawInMessageByBlockNumberOrHashAndIndex(ctx, shardId, transport.BlockNumberOrHash{BlockNumber: &number}, index)
}

func (api *APIImpl) GetRawInMessageByBlockHashAndIndex(
	ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64,
) (hexutil.Bytes, error) {
	return api.getRawInMessageByBlockNumberOrHashAndIndex(ctx, shardId, transport.BlockNumberOrHash{BlockHash: &hash}, index)
}

func (api *APIImpl) GetRawInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (hexutil.Bytes, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block, indexes, err := api.getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	return getRawBlockEntity(tx, shardId, db.MessageTrieTable, block.InMessagesRoot, indexes.MessageIndex.Bytes())
}

func (api *APIImpl) getBlockAndMessageIndexByMessageHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, db.BlockHashAndMessageIndex, error) {
	value, err := tx.GetFromShard(shardId, db.BlockHashAndMessageIndexByMessageHash, hash.Bytes())
	if err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}

	var blockHashAndMessageIndex db.BlockHashAndMessageIndex
	if err := blockHashAndMessageIndex.UnmarshalSSZ(*value); err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}

	block := api.accessor.GetBlockByHash(tx, shardId, blockHashAndMessageIndex.BlockHash)
	if block == nil {
		return nil, db.BlockHashAndMessageIndex{}, errNotFound
	}
	return block, blockHashAndMessageIndex, nil
}

func getRawBlockEntity(
	tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte,
) ([]byte, error) {
	root := mpt.NewReaderWithRoot(tx, shardId, tableName, rootHash)
	entityBytes, err := root.Get(entityKey)
	if err != nil {
		return nil, err
	}
	return entityBytes, nil
}

func getBlockEntity[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
](tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte) (*S, error) {
	root := mpt.NewReaderWithRoot(tx, shardId, tableName, rootHash)
	return mpt.GetEntity[T](root, entityKey)
}

func (api *APIImpl) getRawInMessageByBlockHashAndIndex(
	tx db.RoTx, shardId types.ShardId, block *types.Block, msgIndex types.MessageIndex,
) ([]byte, []byte, error) {
	rawMsg, err := getRawBlockEntity(tx, shardId, db.MessageTrieTable, block.InMessagesRoot, msgIndex.Bytes())
	if err != nil {
		return nil, nil, err
	}

	rawReceipt, err := getRawBlockEntity(tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, msgIndex.Bytes())
	if err != nil {
		return nil, nil, err
	}

	return rawMsg, rawReceipt, nil
}

func (api *APIImpl) getInMessageByBlockHashAndIndex(
	tx db.RoTx, shardId types.ShardId, block *types.Block, msgIndex types.MessageIndex,
) (*types.Message, *types.Receipt, error) {
	msgRaw, receiptRaw, err := api.getRawInMessageByBlockHashAndIndex(tx, shardId, block, msgIndex)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil, nil
	}

	if err != nil || msgRaw == nil || receiptRaw == nil {
		return nil, nil, err
	}

	m := new(types.Message)
	if err := m.UnmarshalSSZ(msgRaw); err != nil {
		return nil, nil, err
	}

	r := new(types.Receipt)
	if err := r.UnmarshalSSZ(receiptRaw); err != nil {
		return nil, nil, err
	}

	return m, r, nil
}
