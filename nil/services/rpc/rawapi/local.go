package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type LocalApi struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
}

var _ Api = (*LocalApi)(nil)

func NewLocalApi(db db.ReadOnlyDB) *LocalApi {
	stateAccessor, err := execution.NewStateAccessor()
	if err != nil {
		return nil
	}
	return &LocalApi{
		db:       db,
		accessor: stateAccessor,
	}
}

func (api *LocalApi) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.Block, error) {
	block, err := api.getBlockByReference(ctx, shardId, blockReference, false)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

func (api *LocalApi) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.BlockWithRawExtractedData, error) {
	return api.getBlockByReference(ctx, shardId, blockReference, true)
}

func (api *LocalApi) getBlockByReference(ctx context.Context, shardId types.ShardId, blockReference BlockReference, withMessages bool) (*types.BlockWithRawExtractedData, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	blockHash, err := api.getBlockHashByReference(tx, shardId, blockReference)
	if err != nil {
		return nil, err
	}

	return api.getBlockByHash(tx, shardId, blockHash, withMessages)
}

func (api *LocalApi) getBlockHashByReference(tx db.RoTx, shardId types.ShardId, blockReference BlockReference) (common.Hash, error) {
	switch blockReference.Type() {
	case NumberBlockReference:
		return db.ReadBlockHashByNumber(tx, shardId, types.BlockNumber(blockReference.Number()))
	case NamedBlockIdentifierReference:
		switch blockReference.NamedBlockIdentifier() {
		case EarliestBlock:
			return db.ReadBlockHashByNumber(tx, shardId, 0)
		case LatestBlock:
			return db.ReadLastBlockHash(tx, shardId)
		default:
			return common.Hash{}, errors.New("unknown named block identifier")
		}
	case HashBlockReference:
		return blockReference.Hash(), nil
	default:
		return common.Hash{}, errors.New("unknown block reference type")
	}
}

func (api *LocalApi) getBlockByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash, withMessages bool) (*types.BlockWithRawExtractedData, error) {
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

	blockHash := data.Block().Hash()
	if blockHash != hash {
		return nil, fmt.Errorf("block hash mismatch: expected %x, got %x", hash, blockHash)
	}

	block := &types.BlockWithExtractedData{
		Block: data.Block(),
	}

	if withMessages {
		// TODO: StateAccessor without decoding
		block.InMessages = data.InMessages()
		block.OutMessages = data.OutMessages()
		block.Receipts = data.Receipts()
		block.Errors = make(map[common.Hash]string)
		for _, message := range block.InMessages {
			msgHash := message.Hash()
			errMsg, err := db.ReadError(tx, msgHash)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			if len(errMsg) > 0 {
				block.Errors[msgHash] = errMsg
			}
		}
	}
	return block.EncodeSSZ()
}
