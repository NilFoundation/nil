package jsonrpc

import (
	"context"
	"fmt"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
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

	data, err := api.accessor.Access(tx, shardId).GetInMessage().WithReceipt().ByHash(hash)
	if err != nil {
		return nil, err
	}
	return NewRPCInMessage(data.Message(), data.Receipt(), data.Index(), data.Block())
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
	if err != nil {
		return nil, err
	}

	msg, receipt, err := api.getInMessageByBlockHashAndIndex(tx, shardId, block, types.MessageIndex(index))
	if err != nil {
		return nil, err
	}

	return NewRPCInMessage(msg, receipt, types.MessageIndex(index), block)
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
	if err != nil {
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

	block, indexes, err := api.getBlockAndInMessageIndexByMessageHash(tx, shardId, hash)
	if err != nil {
		return nil, err
	}
	return getRawBlockEntity(tx, shardId, db.MessageTrieTable, block.InMessagesRoot, indexes.MessageIndex.Bytes())
}

func (api *APIImpl) getBlockAndInMessageIndexByMessageHash(tx db.RoTx, shardId types.ShardId, hash common.Hash) (*types.Block, db.BlockHashAndMessageIndex, error) {
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
	if err != nil {
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
