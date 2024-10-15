package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

var errBlockNotFound = errors.New("block not found")

func (api *LocalShardApi) GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error) {
	shardId := address.ShardId()
	if shardId != api.ShardId {
		return types.Value{}, fmt.Errorf("address is not in the shard %d", api.ShardId)
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return types.Value{}, nil
		}
		return types.Value{}, err
	}
	return acc.Balance, nil
}

func (api *LocalShardApi) GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error) {
	shardId := address.ShardId()
	if shardId != api.ShardId {
		return types.Code{}, fmt.Errorf("address is not in the shard %d", api.ShardId)
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	code, err := db.ReadCode(tx, shardId, acc.CodeHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return code, nil
}

func (api *LocalShardApi) GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error) {
	shardId := address.ShardId()
	if shardId != api.ShardId {
		return nil, fmt.Errorf("address is not in the shard %d", api.ShardId)
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	currencyReader := execution.NewDbCurrencyTrieReader(tx, shardId)
	currencyReader.SetRootHash(acc.CurrencyRoot)
	entries, err := currencyReader.Entries()
	if err != nil {
		return nil, err
	}

	return common.SliceToMap(entries, func(_ int, kv execution.Entry[types.CurrencyId, *types.Value]) (types.CurrencyId, types.Value) {
		return kv.Key, *kv.Val
	}), nil
}

func (api *LocalShardApi) getSmartContract(tx db.RoTx, address types.Address, blockReference rawapitypes.BlockReference) (*types.SmartContract, error) {
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

	root := mpt.NewDbReader(tx, api.ShardId, db.ContractTrieTable)
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

func (api *LocalShardApi) GetMessageCount(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error) {
	if blockReference.Type() == rawapitypes.NamedBlockIdentifierReference &&
		blockReference.NamedBlockIdentifier() == rawapitypes.PendingBlock {
		seqno, inPool := api.msgpool.SeqnoToAddress(address)
		if inPool {
			seqno++
			return uint64(seqno), nil
		}
		// Fallback to latest block if no message in pool
		blockReference = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.LatestBlock)
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return 0, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockReference)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return uint64(acc.ExtSeqno), nil
}
