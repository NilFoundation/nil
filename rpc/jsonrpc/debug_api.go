package jsonrpc

import (
	"context"
	"fmt"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
)

type DebugAPI interface {
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*RPCRawBlock, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (*RPCRawBlock, error)
}

type DebugAPIImpl struct {
	*BaseAPI
	db       db.ReadOnlyDB
	logger   zerolog.Logger
	accessor *execution.StateAccessor
}

var _ DebugAPI = &DebugAPIImpl{}

func NewDebugAPI(base *BaseAPI, db db.ReadOnlyDB, logger zerolog.Logger) *DebugAPIImpl {
	accessor, _ := execution.NewStateAccessor()
	return &DebugAPIImpl{
		BaseAPI:  base,
		db:       db,
		logger:   logger,
		accessor: accessor,
	}
}

// GetBlockByNumber implements eth_getBlockByNumber. Returns information about a block given the block's number.
func (api *DebugAPIImpl) GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, withMessages bool) (*RPCRawBlock, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	if number == transport.LatestBlockNumber {
		hash, err := db.ReadLastBlockHash(tx, shardId)
		if err != nil {
			return nil, err
		}

		return api.getBlockByHash(tx, shardId, hash, withMessages)
	}

	blockHash, err := db.ReadBlockHashByNumber(tx, shardId, number.BlockNumber())
	if err != nil {
		return nil, err
	}
	return api.getBlockByHash(tx, shardId, blockHash, withMessages)
}

// GetBlockByHash implements eth_getBlockByHash. Returns information about a block given the block's hash.
func (api *DebugAPIImpl) GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, withMessages bool) (*RPCRawBlock, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	return api.getBlockByHash(tx, shardId, hash, withMessages)
}

func (api *DebugAPIImpl) getBlockByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash, withMessages bool) (*RPCRawBlock, error) {
	accessor := api.accessor.Access(tx, shardId).GetBlock()
	if withMessages {
		accessor = accessor.WithInMessages().WithOutMessages().WithReceipts()
	}

	data, err := accessor.ByHash(hash)
	if err != nil {
		return nil, err
	}

	if data.Block() == nil {
		return nil, nil
	}
	blockBytes, err := data.Block().MarshalSSZ()
	if err != nil {
		return nil, err
	}

	blockHash := data.Block().Hash()
	if blockHash != hash {
		return nil, fmt.Errorf("block hash mismatch: expected %x, got %x", hash, blockHash)
	}

	rawBlock := &RPCRawBlock{
		Number:  data.Block().Id,
		Hash:    blockHash,
		Content: hexutil.Encode(blockBytes),
	}

	if withMessages {
		hexInMessages, err := hexify(data.InMessages())
		if err != nil {
			return nil, err
		}
		hexOutMessages, err := hexify(data.OutMessages())
		if err != nil {
			return nil, err
		}
		hexReceipts, err := hexify(data.Receipts())
		if err != nil {
			return nil, err
		}

		// TODO: do we need this? looks like data.InMessages() are already ordered by index from 0 to N
		positions := make([]uint64, len(data.InMessages()))
		for i, message := range data.InMessages() {
			value, err := tx.GetFromShard(shardId, db.BlockHashAndInMessageIndexByMessageHash, message.Hash().Bytes())
			if err != nil {
				return nil, err
			}

			var blockHashAndMessageIndex db.BlockHashAndMessageIndex
			if err := blockHashAndMessageIndex.UnmarshalSSZ(value); err != nil {
				return nil, err
			}
			positions[i] = uint64(blockHashAndMessageIndex.MessageIndex)
		}
		rawBlock.InMessagesContent = hexInMessages
		rawBlock.InMessagesRoot = data.Block().InMessagesRoot
		rawBlock.OutMessagesContent = hexOutMessages
		rawBlock.OutMessagesRoot = data.Block().OutMessagesRoot
		rawBlock.ReceiptsContent = hexReceipts
		rawBlock.ReceiptsRoot = data.Block().ReceiptsRoot
		rawBlock.Positions = positions
	}

	return rawBlock, nil
}

func hexify[T fastssz.Marshaler](data []T) ([]string, error) {
	hexed := make([]string, len(data))
	for i, val := range data {
		valBytes, err := val.MarshalSSZ()
		if err != nil {
			return nil, err
		}
		hexed[i] = hexutil.Encode(valBytes)
	}
	return hexed, nil
}
