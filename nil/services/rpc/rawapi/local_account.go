package rawapi

import (
	"context"
	"errors"
	"fmt"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
)

var errBlockNotFound = errors.New("block not found")

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
