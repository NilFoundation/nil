package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/holiman/uint256"
)

func (api *APIImpl) getSmartContract(tx db.RoTx, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*types.SmartContract, error) {
	shardId := address.ShardId()
	block, err := api.fetchBlockByNumberOrHash(tx, shardId, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, errBlockNotFound
	}

	root := mpt.NewReaderWithRoot(tx, shardId, db.ContractTrieTable, block.SmartContractsRoot)
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

// GetBalance implements eth_getBalance. Returns the balance of an account for a given address.
func (api *APIImpl) GetBalance(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error) {
	shardId := address.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockNrOrHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return (*hexutil.Big)(big.NewInt(0)), nil
		}
		return nil, err
	}
	return (*hexutil.Big)(acc.Balance.ToBig()), nil
}

// GetCurrencies implements eth_getCurrencies. Returns the balance of all currencies of account for a given address.
func (api *APIImpl) GetCurrencies(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (map[string]*hexutil.Big, error) {
	shardId := address.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockNrOrHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	currencyReader := mpt.NewReaderWithRoot(tx, shardId, db.CurrencyTrieTable, acc.CurrencyRoot)

	res := make(map[string]*hexutil.Big)
	for kv := range currencyReader.Iterate() {
		var v uint256.Int
		if err = v.UnmarshalSSZ(kv.Value); err != nil {
			return nil, err
		}
		key := hexutil.ToHexNoLeadingZeroes(kv.Key)
		res[key] = (*hexutil.Big)(v.ToBig())
	}
	return res, nil
}

// GetTransactionCount implements eth_getTransactionCount. Returns the number of transactions sent from an address (the nonce / seqno).
func (api *APIImpl) GetTransactionCount(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Uint64, error) {
	shardId := address.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	if blockNrOrHash.BlockNumber != nil && *blockNrOrHash.BlockNumber == transport.PendingBlockNumber {
		nonce, inPool := api.msgPools[shardId].SeqnoToAddress(address)
		if inPool {
			nonce++
			return (*hexutil.Uint64)(&nonce), nil
		}
		// Fallback to latest block if no message in pool
		blockNrOrHash.BlockNumber = transport.LatestBlock.BlockNumber
	}
	zeroNonce := hexutil.Uint64(0)
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockNrOrHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return &zeroNonce, nil
		}

		return nil, err
	}
	return (*hexutil.Uint64)(&acc.ExtSeqno), nil
}

// GetCode implements eth_getCode. Returns the byte code at a given address (if it's a smart contract).
func (api *APIImpl) GetCode(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error) {
	shardId := address.ShardId()
	if err := api.checkShard(shardId); err != nil {
		return nil, err
	}

	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot open tx to find account: %w", err)
	}
	defer tx.Rollback()

	acc, err := api.getSmartContract(tx, address, blockNrOrHash)
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
	return hexutil.Bytes(code), nil
}
