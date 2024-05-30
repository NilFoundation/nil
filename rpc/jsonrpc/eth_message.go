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

	block, indexes, err := getBlockAndMessageIndexByMessageHash(tx, shardId, hash)
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

func (api *APIImpl) GetInMessageByBlockHashAndIndex(ctx context.Context, hash common.Hash, index hexutil.Uint64) (*RPCInMessage, error) {
	return nil, nil
}

func (api *APIImpl) GetInMessageByBlockNumberAndIndex(ctx context.Context, number transport.BlockNumber, txIndex hexutil.Uint) (*RPCInMessage, error) {
	return nil, nil
}

func (api *APIImpl) GetRawInMessageByBlockNumberAndIndex(ctx context.Context, number transport.BlockNumber, index hexutil.Uint) (hexutil.Bytes, error) {
	return nil, nil
}

func (api *APIImpl) GetRawInMessageByBlockHashAndIndex(ctx context.Context, hash common.Hash, index hexutil.Uint) (hexutil.Bytes, error) {
	return nil, nil
}

func (api *APIImpl) GetRawInMessageByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	return nil, nil
}

func getBlockAndMessageIndexByMessageHash(tx db.Tx, shardId types.ShardId, hash common.Hash) (*types.Block, db.BlockHashAndMessageIndex, error) {
	value, err := tx.GetFromShard(shardId, db.BlockHashAndMessageIndexByMessageHash, hash.Bytes())
	if err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}

	var blockHashAndMessageIndex db.BlockHashAndMessageIndex
	if err := blockHashAndMessageIndex.UnmarshalSSZ(*value); err != nil {
		return nil, db.BlockHashAndMessageIndex{}, err
	}

	block := db.ReadBlock(tx, shardId, blockHashAndMessageIndex.BlockHash)
	if block == nil {
		return nil, db.BlockHashAndMessageIndex{}, errNotFound
	}
	return block, blockHashAndMessageIndex, nil
}

func getBlockEntity[
	T interface {
		~*S
		fastssz.Unmarshaler
	},
	S any,
](tx db.Tx, shardId types.ShardId, tableName db.ShardedTableName, rootHash common.Hash, entityKey []byte) (*S, error) {
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, shardId, tableName, rootHash)
	return mpt.GetEntity[T](root, entityKey)
}
