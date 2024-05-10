package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
)

type DebugAPI interface {
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (map[string]any, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (map[string]any, error)
}

type DebugAPIImpl struct {
	*BaseAPI
	db     db.DB
	logger *zerolog.Logger
}

func NewDebugAPI(base *BaseAPI, db db.DB, logger *zerolog.Logger) *DebugAPIImpl {
	return &DebugAPIImpl{
		BaseAPI: base,
		db:      db,
		logger:  logger,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (map[string]any, error) {
	if number == transport.LatestBlockNumber {
		hash, err := api.db.Get(db.LastBlockTable, shardId.Bytes())
		if err != nil {
			return nil, err
		}

		return api.GetBlockByHash(ctx, shardId, common.CastToHash(*hash))
	}

	return nil, errNotImplemented
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (map[string]any, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block := db.ReadBlock(tx, shardId, hash)
	if block == nil {
		return nil, nil
	}

	blockBytes, err := block.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"number":  block.Id,
		"hash":    block.Hash(),
		"content": hexutil.Encode(blockBytes),
	}

	return result, nil
}
