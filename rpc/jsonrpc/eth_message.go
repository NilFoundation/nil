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

	block, indexes, err := api.getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}

	msg, err := getBlockEntity[*types.Message](tx, shardId, db.MessageTrieTable, block.InMessagesRoot, indexes.MessageIndex.Bytes())
	if err != nil {
		return nil, err
	}
	receipt, err := getBlockEntity[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, indexes.MessageIndex.Bytes())
	if err != nil {
		return nil, err
	}
	return NewRPCInMessage(msg, receipt, indexes.MessageIndex, block), nil
}

func (api *APIImpl) GetInMessageByBlockHashAndIndex(ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64) (*RPCInMessage, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	block, msg, receipt, err := api.getInMessageByBlockHashAndIndex(tx, shardId, hash, types.MessageIndex(index))
	if err != nil || block == nil || msg == nil || receipt == nil {
		return nil, err
	}

	return NewRPCInMessage(msg, receipt, types.MessageIndex(index), block), nil
}

func (api *APIImpl) GetInMessageByBlockNumberAndIndex(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint) (*RPCInMessage, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	hash, err := api.getBlockHashByNumber(tx, shardId, number)
	if err != nil {
		return nil, err
	}

	block, msg, receipt, err := api.getInMessageByBlockHashAndIndex(tx, shardId, hash, types.MessageIndex(index))
	if err != nil || block == nil || msg == nil || receipt == nil {
		return nil, err
	}

	return NewRPCInMessage(msg, receipt, types.MessageIndex(index), block), nil
}

func (api *APIImpl) GetRawInMessageByBlockNumberAndIndex(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint) (hexutil.Bytes, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	hash, err := api.getBlockHashByNumber(tx, shardId, number)
	if err != nil {
		return nil, err
	}

	_, msgRaw, _, err := api.getRawInMessageByBlockHashAndIndex(tx, shardId, hash, types.MessageIndex(index))
	return msgRaw, err
}

func (api *APIImpl) GetRawInMessageByBlockHashAndIndex(ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint) (hexutil.Bytes, error) {
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	_, msgRaw, _, err := api.getRawInMessageByBlockHashAndIndex(tx, shardId, hash, types.MessageIndex(index))
	return msgRaw, err
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

	block := api.getBlockByHash(tx, shardId, blockHashAndMessageIndex.BlockHash)
	if block == nil {
		return nil, db.BlockHashAndMessageIndex{}, errNotFound
	}
	return block, blockHashAndMessageIndex, nil
}

func getRawBlockEntity(
	tx db.RoTx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte,
) ([]byte, error) {
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, tableName, rootHash)
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
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, tableName, rootHash)
	return mpt.GetEntity[T](root, entityKey)
}

func (api *APIImpl) getRawInMessageByBlockHashAndIndex(
	tx db.RoTx, shardId types.ShardId, blockHash common.Hash, msgIndex types.MessageIndex,
) (*types.Block, []byte, []byte, error) {
	block := api.getBlockByHash(tx, shardId, blockHash)
	if block == nil {
		return nil, nil, nil, nil
	}

	rawMsg, err := getRawBlockEntity(tx, shardId, db.MessageTrieTable, block.InMessagesRoot, msgIndex.Bytes())
	if err != nil {
		return nil, nil, nil, err
	}

	rawReceipt, err := getRawBlockEntity(tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot, msgIndex.Bytes())
	if err != nil {
		return nil, nil, nil, err
	}

	return block, rawMsg, rawReceipt, nil
}

func (api *APIImpl) getInMessageByBlockHashAndIndex(
	tx db.RoTx, shardId types.ShardId, blockHash common.Hash, msgIndex types.MessageIndex,
) (*types.Block, *types.Message, *types.Receipt, error) {
	block, msgRaw, receiptRaw, err := api.getRawInMessageByBlockHashAndIndex(tx, shardId, blockHash, msgIndex)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil, nil, nil
	}

	if err != nil || block == nil || msgRaw == nil || receiptRaw == nil {
		return nil, nil, nil, err
	}

	m := new(types.Message)
	if err := m.UnmarshalSSZ(msgRaw); err != nil {
		return nil, nil, nil, err
	}

	r := new(types.Receipt)
	if err := r.UnmarshalSSZ(receiptRaw); err != nil {
		return nil, nil, nil, err
	}

	return block, m, r, nil
}
