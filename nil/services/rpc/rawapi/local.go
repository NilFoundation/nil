package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/assert"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

var errBlockNotFound = errors.New("block not found")

type LocalShardApi struct {
	db       db.ReadOnlyDB
	accessor *execution.StateAccessor
	ShardId  types.ShardId
}

var _ ShardApi = (*LocalShardApi)(nil)

func NewLocalShardApi(shardId types.ShardId, db db.ReadOnlyDB) (*LocalShardApi, error) {
	stateAccessor, err := execution.NewStateAccessor()
	if err != nil {
		return nil, err
	}
	return &LocalShardApi{
		db:       db,
		accessor: stateAccessor,
		ShardId:  shardId,
	}, nil
}

func (api *LocalShardApi) GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, err := api.getBlockByReference(tx, blockReference, false)
	if err != nil {
		return nil, err
	}
	return block.Block, nil
}

func (api *LocalShardApi) GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	return api.getBlockByReference(tx, blockReference, true)
}

func (api *LocalShardApi) GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := api.getBlockByReference(tx, blockReference, true)
	if err != nil {
		return 0, err
	}
	return uint64(len(res.InMessages)), nil
}

func (api *LocalShardApi) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address.ShardId(), address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return types.Value{}, nil
		}
		return types.Value{}, err
	}
	return acc.Balance, nil
}

func (api *LocalShardApi) getSmartContract(tx db.RoTx, shardId types.ShardId, address types.Address, blockReference rawapitypes.BlockReference) (*types.SmartContract, error) {
	rawBlock, err := api.getBlockByReference(tx, blockReference, false)
	if err != nil {
		return nil, err
	}
	if rawBlock == nil {
		return nil, errBlockNotFound
	}
	var block types.Block
	if err := block.UnmarshalSSZ(rawBlock.Block); err != nil {
		return nil, err
	}

	root := mpt.NewDbReader(tx, shardId, db.ContractTrieTable)
	root.SetRootHash(block.SmartContractsRoot)
	contractRaw, err := root.Get(address.Hash().Bytes())
	if err != nil {
		return nil, err
	}

	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(contractRaw); err != nil {
		return nil, err
	}

	return contract, nil
}

func (api *LocalShardApi) getBlockByReference(tx db.RoTx, blockReference rawapitypes.BlockReference, withMessages bool) (*types.RawBlockWithExtractedData, error) {
	blockHash, err := api.getBlockHashByReference(tx, blockReference)
	if err != nil {
		return nil, err
	}

	return api.getBlockByHash(tx, blockHash, withMessages)
}

func (api *LocalShardApi) getBlockHashByReference(tx db.RoTx, blockReference rawapitypes.BlockReference) (common.Hash, error) {
	switch blockReference.Type() {
	case rawapitypes.NumberBlockReference:
		return db.ReadBlockHashByNumber(tx, api.ShardId, types.BlockNumber(blockReference.Number()))
	case rawapitypes.NamedBlockIdentifierReference:
		switch blockReference.NamedBlockIdentifier() {
		case rawapitypes.EarliestBlock:
			return db.ReadBlockHashByNumber(tx, api.ShardId, 0)
		case rawapitypes.LatestBlock:
			return db.ReadLastBlockHash(tx, api.ShardId)
		default:
			return common.Hash{}, errors.New("unknown named block identifier")
		}
	case rawapitypes.HashBlockReference:
		return blockReference.Hash(), nil
	default:
		return common.Hash{}, errors.New("unknown block reference type")
	}
}

func (api *LocalShardApi) getBlockByHash(tx db.RoTx, hash common.Hash, withMessages bool) (*types.RawBlockWithExtractedData, error) {
	accessor := api.accessor.RawAccess(tx, api.ShardId).GetBlock()
	if withMessages {
		accessor = accessor.WithInMessages().WithOutMessages().WithReceipts().WithChildBlocks().WithDbTimestamp()
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
		result.ChildBlocks = data.ChildBlocks()
		result.DbTimestamp = data.DbTimestamp()

		// Need to decode messages to get its hashes because external message hash
		// calculated in a bit different way (not just Hash(SSZ)).
		messages, err := ssz.DecodeContainer[*types.Message](result.InMessages)
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
