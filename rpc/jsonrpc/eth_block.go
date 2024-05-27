package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *APIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (map[string]any, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	var requestedBlockNumber types.BlockNumber
	switch number {
	case transport.LatestExecutedBlockNumber:
		fallthrough
	case transport.FinalizedBlockNumber:
		fallthrough
	case transport.SafeBlockNumber:
		fallthrough
	case transport.PendingBlockNumber:
		return nil, errNotImplemented
	case transport.LatestBlockNumber:
		lastBlock, err := api.getLastBlock(tx, shardId)
		if err != nil {
			return nil, err
		}
		if lastBlock == nil {
			return nil, nil
		}
		requestedBlockNumber = lastBlock.Id
	case transport.EarliestBlockNumber:
		fallthrough
	default:
		requestedBlockNumber = number.BlockNumber()
	}

	blockHash, err := tx.GetFromShard(shardId, db.BlockHashByNumberIndex, requestedBlockNumber.Bytes())
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	block := db.ReadBlock(tx, shardId, common.CastToHash(*blockHash))
	if block == nil {
		return nil, nil
	}
	return toMap(block), nil
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *APIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (map[string]any, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	block := db.ReadBlock(tx, shardId, hash)
	if block == nil {
		return nil, nil
	}
	return toMap(block), nil
}

// GetBlockTransactionCountByNumber implements eth_getBlockTransactionCountByNumber. Returns the number of transactions in a block given the block's block number.
func (api *APIImpl) GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, blockNr transport.BlockNumber) (*hexutil.Uint, error) {
	return nil, errNotImplemented
}

// GetBlockTransactionCountByHash implements eth_getBlockTransactionCountByHash. Returns the number of transactions in a block given the block's block hash.
func (api *APIImpl) GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, blockHash common.Hash) (*hexutil.Uint, error) {
	return nil, errNotImplemented
}

func (api *APIImpl) getLastBlock(tx db.Tx, shardId types.ShardId) (*types.Block, error) {
	lastBlockHash, err := db.ReadLastBlockHash(tx, shardId)
	if err != nil {
		return nil, err
	}
	if lastBlockHash == common.EmptyHash {
		return nil, nil
	}
	return db.ReadBlock(tx, shardId, lastBlockHash), nil
}

func toMap(block *types.Block) map[string]any {
	var number hexutil.Big
	number.ToInt().SetUint64(block.Id.Uint64())
	return map[string]any{
		"number":     number,
		"hash":       block.Hash(),
		"parentHash": block.PrevBlock,
	}
}
