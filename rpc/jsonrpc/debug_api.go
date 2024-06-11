package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
)

type DebugAPI interface {
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (map[string]any, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (map[string]any, error)
}

type DebugAPIImpl struct {
	*BaseAPI
	db     db.ReadOnlyDB
	logger zerolog.Logger
}

var _ DebugAPI = &DebugAPIImpl{}

func NewDebugAPI(base *BaseAPI, db db.ReadOnlyDB, logger zerolog.Logger) *DebugAPIImpl {
	return &DebugAPIImpl{
		BaseAPI: base,
		db:      db,
		logger:  logger,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (map[string]any, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	defer tx.Rollback()

	if number == transport.LatestBlockNumber {
		hash, err := tx.Get(db.LastBlockTable, shardId.Bytes())
		if err != nil {
			return nil, err
		}

		return api.getBlockByHash(tx, shardId, common.CastToHash(*hash), withMessages)
	}

	blockHash, err := tx.GetFromShard(shardId, db.BlockHashByNumberIndex, number.BlockNumber().Bytes())
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return api.getBlockByHash(tx, shardId, common.CastToHash(*blockHash), withMessages)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (map[string]any, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	return api.getBlockByHash(tx, shardId, hash, withMessages)
}

func (api *DebugAPIImpl) getBlockByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash, withMessages bool) (map[string]any, error) {
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
	if withMessages {
		inMsgs, err := execution.CollectBlockEntities[*types.Message](tx, shardId, db.MessageTrieTable, block.InMessagesRoot)
		if err != nil {
			return nil, err
		}
		hexInMessages := make([]string, len(inMsgs))
		for i, message := range inMsgs {
			messageBytes, err := message.MarshalSSZ()
			if err != nil {
				return nil, err
			}
			hexInMessages[i] = hexutil.Encode(messageBytes)
		}

		outMsgs, err := execution.CollectBlockEntities[*types.Message](tx, shardId, db.MessageTrieTable, block.OutMessagesRoot)
		if err != nil {
			return nil, err
		}
		hexOutMessages := make([]string, len(outMsgs))
		for i, message := range outMsgs {
			messageBytes, err := message.MarshalSSZ()
			if err != nil {
				return nil, err
			}
			hexOutMessages[i] = hexutil.Encode(messageBytes)
		}

		receipts, err := execution.CollectBlockEntities[*types.Receipt](tx, shardId, db.ReceiptTrieTable, block.ReceiptsRoot)
		if err != nil {
			return nil, err
		}
		hexReceipts := make([]string, len(receipts))
		for i, receipt := range receipts {
			receiptBytes, err := receipt.MarshalSSZ()
			if err != nil {
				return nil, err
			}
			hexReceipts[i] = hexutil.Encode(receiptBytes)
		}

		positions := make([]uint64, len(inMsgs))
		for i, message := range inMsgs {
			value, err := tx.GetFromShard(shardId, db.BlockHashAndMessageIndexByMessageHash, message.Hash().Bytes())
			if err != nil {
				return nil, err
			}

			var blockHashAndMessageIndex db.BlockHashAndMessageIndex
			if err := blockHashAndMessageIndex.UnmarshalSSZ(*value); err != nil {
				return nil, err
			}
			positions[i] = uint64(blockHashAndMessageIndex.MessageIndex)
		}
		result["inMessages"] = hexInMessages
		result["outMessages"] = hexOutMessages
		result["receipts"] = hexReceipts
		result["positions"] = positions
	}

	return result, nil
}
