package rawapi

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/ssz"
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

func (api *LocalApi) GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (ssz.SSZEncodedData, error) {
	block, err := api.getBlockByReference(ctx, shardId, blockReference, false)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

func (api *LocalApi) GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference BlockReference) (*types.RawBlockWithExtractedData, error) {
	return api.getBlockByReference(ctx, shardId, blockReference, true)
}

func (api *LocalApi) getBlockByReference(ctx context.Context, shardId types.ShardId, blockReference BlockReference, withMessages bool) (*types.RawBlockWithExtractedData, error) {
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

func (api *LocalApi) getBlockByHash(tx db.RoTx, shardId types.ShardId, hash common.Hash, withMessages bool) (*types.RawBlockWithExtractedData, error) {
	accessor := api.accessor.RawAccess(tx, shardId).GetBlock()
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

	if assert.Enable {
		blockHash := common.PoseidonHash(data.Block())
		check.PanicIfNotf(blockHash == hash, "block hash mismatch: %s != %s", blockHash, hash)
	}

	result := &types.RawBlockWithExtractedData{
		Block: data.Block(),
	}
	if withMessages {
		result.InMessages = data.InMessages()
		result.OutMessages = data.OutMessages()
		result.Receipts = data.Receipts()
		result.Errors = make(map[common.Hash]string)

		// Need to decode messages to get its hashes because external message hash
		// calculated in a bit different way (not just Hash(SSZ)).
		messages, err := ssz.DecodeContainer[types.Message, *types.Message](result.InMessages)
		if err != nil {
			return nil, err
		}
		for _, message := range messages {
			msgHash := message.Hash()
			errMsg, err := db.ReadError(tx, msgHash)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			if len(errMsg) > 0 {
				result.Errors[msgHash] = errMsg
			}
		}
	}
	return result, nil
}
